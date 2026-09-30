package grocksdb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func callReductionMultiGetDB(t testing.TB, noGoCallbacks bool, configure func(*Options)) (*DB, []*ColumnFamilyHandle) {
	t.Helper()
	opts := NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetCreateIfMissingColumnFamilies(true)
	opts.SetNoGoCallbacks(noGoCallbacks)
	if configure != nil {
		configure(opts)
	}
	db, cfs, err := OpenDbColumnFamilies(opts, t.TempDir(), []string{"default", "extra"}, []*Options{opts, opts})
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, cf := range cfs {
			cf.Destroy()
		}
		db.Close()
		// The C options own their merge operator through shared_ptr. Avoid the
		// legacy Options.Destroy double-destroy of that same native object.
		opts.cmo = nil
		opts.Destroy()
	})
	return db, cfs
}

func TestBatchedMultiGetCFKeyCopy(t *testing.T) {
	for _, mode := range benchModes {
		t.Run(mode.name, func(t *testing.T) {
			db, cfs := callReductionMultiGetDB(t, mode.noGoCallbacks, nil)
			wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()
			defer wo.Destroy()
			defer ro.Destroy()
			keys := [][]byte{nil, {}, {0, 0xff, 1}, []byte("dup"), []byte("emptyvalue"), []byte("missing"), []byte("dup")}
			for cfIndex, cf := range cfs {
				require.NoError(t, db.PutCF(wo, cf, nil, []byte(fmt.Sprintf("empty-key-cf-%d", cfIndex))))
				require.NoError(t, db.PutCF(wo, cf, keys[2], []byte{0xff, 0, byte(cfIndex)}))
				require.NoError(t, db.PutCF(wo, cf, keys[3], []byte(fmt.Sprintf("duplicate-cf-%d", cfIndex))))
				require.NoError(t, db.PutCF(wo, cf, keys[4], nil))
				baseline, err := db.BatchedMultiGetCF(ro, cf, false, keys...)
				require.NoError(t, err)
				values, err := db.BatchedMultiGetCF(ro, cf, false, keys...)
				require.NoError(t, err)
				require.Len(t, values, len(keys))
				for i, v := range values {
					require.Equal(t, baseline[i].Exists(), v.Exists())
					require.Equal(t, baseline[i].Data(), v.Data())
				}
				require.True(t, values[4].Exists())
				require.False(t, values[5].Exists())
				baseline.Destroy()
				// Storage holding input keys has already been freed on return.
				// More reads must not invalidate pinned results.
				other, err := db.BatchedMultiGetCF(ro, cf, true, []byte("dup"), []byte("emptyvalue"), []byte("missing"))
				require.NoError(t, err)
				require.Equal(t, []byte(fmt.Sprintf("duplicate-cf-%d", cfIndex)), other[0].Data())
				require.True(t, other[1].Exists())
				require.False(t, other[2].Exists())
				other.Destroy()
				keys[2][0] = 99
				require.Equal(t, []byte{0xff, 0, byte(cfIndex)}, values[2].Data())
				keys[2][0] = 0
				require.Equal(t, []byte(fmt.Sprintf("duplicate-cf-%d", cfIndex)), values[3].Data())
				values.Destroy()
				allEmpty, err := db.BatchedMultiGetCF(ro, cf, true, nil, []byte{}, nil)
				require.NoError(t, err)
				for _, value := range allEmpty {
					require.Equal(t, []byte(fmt.Sprintf("empty-key-cf-%d", cfIndex)), value.Data())
				}
				allEmpty.Destroy()
				for _, input := range [][][]byte{nil, {}} {
					empty, err := db.BatchedMultiGetCF(ro, cf, false, input...)
					require.NoError(t, err)
					require.Empty(t, empty)
					empty.Destroy()
				}
			}
		})
	}
}

func TestBatchedMultiGetCFKeyCopyCallbacks(t *testing.T) {
	var compares, merges atomic.Int64
	db, cfs := callReductionMultiGetDB(t, false, func(opts *Options) {
		opts.SetComparator(NewComparator("grocksdb.multiget.contiguous", func(a, b []byte) int {
			compares.Add(1)
			return bytes.Compare(a, b)
		}))
		opts.SetMergeOperator(&mockMergeOperator{fullMerge: func(_ []byte, existing []byte, operands [][]byte) ([]byte, bool) {
			merges.Add(1)
			out := append([]byte(nil), existing...)
			for _, operand := range operands {
				out = append(out, operand...)
			}
			return out, true
		}})
	})
	wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()
	defer wo.Destroy()
	defer ro.Destroy()
	require.NoError(t, db.PutCF(wo, cfs[1], []byte("a"), []byte("x")))
	require.NoError(t, db.MergeCF(wo, cfs[1], []byte("a"), []byte("y")))
	require.NoError(t, db.PutCF(wo, cfs[1], []byte("b"), []byte("z")))
	beforeCompares, beforeMerges := compares.Load(), merges.Load()
	values, err := db.BatchedMultiGetCF(ro, cfs[1], false, []byte("b"), []byte("a"), []byte("missing"))
	require.NoError(t, err)
	defer values.Destroy()
	require.Equal(t, []byte("z"), values[0].Data())
	require.Equal(t, []byte("xy"), values[1].Data())
	require.False(t, values[2].Exists())
	require.Greater(t, compares.Load(), beforeCompares)
	require.Greater(t, merges.Load(), beforeMerges)
}

func TestBatchedMultiGetCFKeyCopyError(t *testing.T) {
	db, cfs := callReductionMultiGetDB(t, false, func(opts *Options) {
		opts.SetMergeOperator(&mockMergeOperator{fullMerge: func(_ []byte, _ []byte, _ [][]byte) ([]byte, bool) {
			return nil, false
		}})
	})
	wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()
	defer wo.Destroy()
	defer ro.Destroy()
	require.NoError(t, db.PutCF(wo, cfs[1], []byte("ok"), []byte("value")))
	require.NoError(t, db.MergeCF(wo, cfs[1], []byte("fail"), []byte("value")))
	values, err := db.BatchedMultiGetCF(ro, cfs[1], false, []byte("ok"), []byte("fail"))
	require.Error(t, err)
	require.Nil(t, values)
}

// Compare against master to see the effect of the single key allocation.
func BenchmarkCallReductionMultiGet(b *testing.B) {
	for _, mode := range benchModes {
		for _, count := range []int{1, 16, 128} {
			b.Run(fmt.Sprintf("%s/keys=%d", mode.name, count), func(b *testing.B) {
				db, cfs := callReductionMultiGetDB(b, mode.noGoCallbacks, nil)
				wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()
				defer wo.Destroy()
				defer ro.Destroy()
				keys := make([][]byte, count)
				for i := range keys {
					keys[i] = make([]byte, 8)
					binary.BigEndian.PutUint64(keys[i], uint64(i))
					if err := db.PutCF(wo, cfs[1], keys[i], keys[i]); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportAllocs()
				for b.Loop() {
					values, err := db.BatchedMultiGetCF(ro, cfs[1], true, keys...)
					if err != nil {
						b.Fatal(err)
					}
					values.Destroy()
				}
			})
		}
	}
}
