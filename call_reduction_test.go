package grocksdb

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIteratorKV(t *testing.T) {
	for _, mode := range benchModes {
		t.Run(mode.name, func(t *testing.T) {
			opts := NewDefaultOptions()
			opts.SetCreateIfMissing(true)
			opts.SetCreateIfMissingColumnFamilies(true)
			opts.SetNoGoCallbacks(mode.noGoCallbacks)
			defer opts.Destroy()
			db, cfs, err := OpenDbColumnFamilies(opts, t.TempDir(), []string{"default", "cf"}, []*Options{opts, opts})
			require.NoError(t, err)
			defer db.Close()
			for _, cf := range cfs {
				defer cf.Destroy()
			}
			wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()
			defer wo.Destroy()
			defer ro.Destroy()
			it := db.NewIterator(ro)
			_, _, valid := it.ReadKV()
			require.False(t, valid)
			it.SeekToFirst()
			_, _, valid = it.NextKV()
			require.False(t, valid)
			it.Close()
			keys := [][]byte{{}, {0, 1}, {'z'}}
			values := [][]byte{{}, {'b', 0}, {'v'}}
			for _, cf := range cfs {
				for i := range keys {
					require.NoError(t, db.PutCF(wo, cf, keys[i], values[i]))
				}
			}
			iters, err := db.NewIterators(ro, cfs)
			require.NoError(t, err)
			iters = append(iters, db.NewIterator(ro), db.NewIteratorCF(ro, cfs[1]))
			for _, it := range iters {
				it.SeekToFirst()
				i := 0
				for k, v, ok := it.ReadKV(); ok; k, v, ok = it.NextKV() {
					require.Less(t, i, len(keys))
					require.True(t, bytes.Equal(keys[i], k.Data()))
					require.True(t, bytes.Equal(values[i], v.Data()))
					i++
				}
				require.Equal(t, len(keys), i)
				k, v, ok := it.NextKV()
				require.False(t, ok)
				require.Empty(t, k.Data())
				require.Empty(t, v.Data())
				require.NoError(t, it.Err())
				it.Seek(keys[1])
				k, _, ok = it.ReadKV()
				require.True(t, ok)
				require.Equal(t, keys[1], k.Data())
				it.Close()
			}
		})
	}
}

func TestGetIntoBuffer(t *testing.T) {
	for _, mode := range benchModes {
		t.Run(mode.name, func(t *testing.T) {
			db, cfs, cleanup := newTestDBMultiCF(t, []string{"default", "cf"}, func(opts *Options) { opts.SetNoGoCallbacks(mode.noGoCallbacks) })
			defer cleanup()
			wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()
			defer wo.Destroy()
			defer ro.Destroy()
			key, value := []byte{0, 'k'}, []byte{1, 0, 2, 3}
			require.NoError(t, db.Put(wo, key, value))
			require.NoError(t, db.PutCF(wo, cfs[1], key, value))
			require.NoError(t, db.Put(wo, nil, nil))
			require.NoError(t, db.PutCF(wo, cfs[1], nil, nil))
			reads := []func([]byte, []byte) (int, bool, error){
				func(k, buf []byte) (int, bool, error) { return db.GetIntoBuffer(ro, k, buf) },
				func(k, buf []byte) (int, bool, error) { return db.GetIntoBufferCF(ro, cfs[1], k, buf) },
			}
			for _, read := range reads {
				for _, length := range []int{0, 2, 4, 8} {
					backing := bytes.Repeat([]byte{0xaa}, 8)
					buf := backing[:length]
					n, found, err := read(key, buf)
					require.NoError(t, err)
					require.True(t, found)
					require.Equal(t, len(value), n)
					if length < n {
						require.Equal(t, bytes.Repeat([]byte{0xaa}, 8), backing)
					} else {
						require.Equal(t, value, buf[:n])
						require.Equal(t, bytes.Repeat([]byte{0xaa}, 8-n), backing[n:])
					}
				}
				for _, buf := range [][]byte{nil, {}} {
					n, found, err := read(nil, buf)
					require.NoError(t, err)
					require.True(t, found)
					require.Zero(t, n)
				}
				buf := []byte{0xaa}
				n, found, err := read([]byte("missing"), buf)
				require.NoError(t, err)
				require.False(t, found)
				require.Zero(t, n)
				require.Equal(t, []byte{0xaa}, buf)
			}
		})
	}
}

func TestCallReductionGoCallbacks(t *testing.T) {
	var compares, merges atomic.Int64
	db, cfs := callReductionMultiGetDB(t, false, func(opts *Options) {
		opts.SetComparator(NewComparator("call-reduction", func(a, b []byte) int { compares.Add(1); return bytes.Compare(a, b) }))
		opts.SetMergeOperator(&mockMergeOperator{fullMerge: func(_, existing []byte, operands [][]byte) ([]byte, bool) {
			merges.Add(1)
			out := bytes.Clone(existing)
			for _, op := range operands {
				out = append(out, op...)
			}
			return out, true
		}})
	})
	wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()
	defer wo.Destroy()
	defer ro.Destroy()
	for _, k := range []string{"a", "b", "c"} {
		require.NoError(t, db.Put(wo, []byte(k), []byte("x")))
		require.NoError(t, db.Merge(wo, []byte(k), []byte("y")))
		require.NoError(t, db.PutCF(wo, cfs[1], []byte(k), []byte("x")))
		require.NoError(t, db.MergeCF(wo, cfs[1], []byte(k), []byte("y")))
	}
	compares.Store(0)
	for _, k := range []string{"a", "b", "c"} {
		var buf [4]byte
		n, found, err := db.GetIntoBuffer(ro, []byte(k), buf[:])
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "xy", string(buf[:n]))
		n, found, err = db.GetIntoBufferCF(ro, cfs[1], []byte(k), buf[:])
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, "xy", string(buf[:n]))
	}
	require.Positive(t, compares.Load())
	merges.Store(0)
	it := db.NewIterator(ro)
	defer it.Close()
	it.SeekToFirst()
	n := 0
	for _, v, valid := it.ReadKV(); valid; _, v, valid = it.NextKV() {
		require.Equal(t, "xy", string(v.Data()))
		n++
	}
	require.Equal(t, 3, n)
	require.Positive(t, merges.Load())
	require.NoError(t, it.Err())
}

func TestGetIntoBufferError(t *testing.T) {
	db, cfs := callReductionMultiGetDB(t, false, func(opts *Options) {
		opts.SetMergeOperator(&mockMergeOperator{fullMerge: func(_, _ []byte, _ [][]byte) ([]byte, bool) { return nil, false }})
	})
	wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()
	defer wo.Destroy()
	defer ro.Destroy()
	require.NoError(t, db.Merge(wo, []byte("bad"), []byte("x")))
	require.NoError(t, db.MergeCF(wo, cfs[1], []byte("bad"), []byte("x")))
	buf := []byte{0xaa}
	n, found, err := db.GetIntoBuffer(ro, []byte("bad"), buf)
	require.Error(t, err)
	require.Zero(t, n)
	require.False(t, found)
	require.Equal(t, []byte{0xaa}, buf)
	n, found, err = db.GetIntoBufferCF(ro, cfs[1], []byte("bad"), buf)
	require.Error(t, err)
	require.Zero(t, n)
	require.False(t, found)
	require.Equal(t, []byte{0xaa}, buf)
	it := db.NewIterator(ro)
	defer it.Close()
	it.SeekToFirst()
	_, _, valid := it.ReadKV()
	require.False(t, valid)
	require.Error(t, it.Err())
}

var callReductionSink int

func BenchmarkCallReductionIterator(b *testing.B) {
	for _, mode := range benchModes {
		for _, path := range []string{"existing", "slices", "merged"} {
			b.Run(mode.name+"/"+path, func(b *testing.B) {
				db := benchDB(b, mode.noGoCallbacks)
				benchFill(b, db, 4096)
				ro := NewDefaultReadOptions()
				defer ro.Destroy()
				it := db.NewIterator(ro)
				defer it.Close()
				it.SeekToFirst()
				b.ReportAllocs()
				if path == "merged" {
					k, v, valid := it.ReadKV()
					for b.Loop() {
						if !valid {
							it.SeekToFirst()
							k, v, valid = it.ReadKV()
						}
						callReductionSink = len(k.Data()) + len(v.Data())
						k, v, valid = it.NextKV()
					}
				} else {
					for b.Loop() {
						if !it.Valid() {
							it.SeekToFirst()
						}
						if path == "existing" {
							k, v := it.Key(), it.Value()
							callReductionSink = len(k.Data()) + len(v.Data())
							k.Free()
							v.Free()
						} else {
							callReductionSink = len(it.KeySlice().Data()) + len(it.ValueSlice().Data())
						}
						it.Next()
					}
				}
			})
		}
	}
}

func BenchmarkCallReductionGet(b *testing.B) {
	for _, mode := range benchModes {
		for _, size := range []int{8, 1024} {
			for _, path := range []string{"existing", "buffer"} {
				b.Run(fmt.Sprintf("%s/bytes=%d/%s", mode.name, size, path), func(b *testing.B) {
					db := benchDB(b, mode.noGoCallbacks)
					wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()
					defer wo.Destroy()
					defer ro.Destroy()
					key := []byte("key")
					require.NoError(b, db.Put(wo, key, bytes.Repeat([]byte{1}, size)))
					buf := make([]byte, size)
					b.ReportAllocs()
					for b.Loop() {
						if path == "existing" {
							v, err := db.Get(ro, key)
							if err != nil {
								b.Fatal(err)
							}
							copy(buf, v.Data())
							callReductionSink = len(v.Data())
							v.Free()
						} else {
							n, found, err := db.GetIntoBuffer(ro, key, buf)
							if err != nil || !found || n != size {
								b.Fatalf("get: %d %v %v", n, found, err)
							}
							callReductionSink = n
						}
					}
				})
			}
		}
	}
}
