package grocksdb

import (
	"encoding/binary"
	"testing"
)

// Run each hot path with and without Options.SetNoGoCallbacks. Compare against
// -tags grocksdb_no_cgo_annotations to isolate the effect of the annotations.
var benchModes = []struct {
	name          string
	noGoCallbacks bool
}{
	{"default", false},
	{"noGoCallbacks", true},
}

func benchDB(b *testing.B, noGoCallbacks bool) *DB {
	opts := NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetNoGoCallbacks(noGoCallbacks)
	db, err := OpenDb(opts, b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(db.Close)
	return db
}

func benchFill(b *testing.B, db *DB, n int) {
	wo := NewDefaultWriteOptions()
	defer wo.Destroy()
	var k [8]byte
	for i := 0; i < n; i++ {
		binary.BigEndian.PutUint64(k[:], uint64(i))
		if err := db.Put(wo, k[:], k[:]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNoGoCallbacksPut(b *testing.B) {
	for _, m := range benchModes {
		b.Run(m.name, func(b *testing.B) {
			db := benchDB(b, m.noGoCallbacks)
			wo := NewDefaultWriteOptions()
			wo.DisableWAL(true)
			defer wo.Destroy()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var k [8]byte
				binary.BigEndian.PutUint64(k[:], uint64(i))
				if err := db.Put(wo, k[:], k[:]); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkNoGoCallbacksGet(b *testing.B) {
	const n = 1 << 12
	for _, m := range benchModes {
		b.Run(m.name, func(b *testing.B) {
			db := benchDB(b, m.noGoCallbacks)
			benchFill(b, db, n)
			ro := NewDefaultReadOptions()
			defer ro.Destroy()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var k [8]byte
				binary.BigEndian.PutUint64(k[:], uint64(i%n))
				v, err := db.Get(ro, k[:])
				if err != nil {
					b.Fatal(err)
				}
				v.Free()
			}
		})
	}
}

func BenchmarkNoGoCallbacksGetPinned(b *testing.B) {
	const n = 1 << 12
	for _, m := range benchModes {
		b.Run(m.name, func(b *testing.B) {
			db := benchDB(b, m.noGoCallbacks)
			benchFill(b, db, n)
			ro := NewDefaultReadOptions()
			defer ro.Destroy()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var k [8]byte
				binary.BigEndian.PutUint64(k[:], uint64(i%n))
				v, err := db.GetPinned(ro, k[:])
				if err != nil {
					b.Fatal(err)
				}
				_ = v.Data()
				v.Destroy()
			}
		})
	}
}

func BenchmarkWriteBatchPut(b *testing.B) {
	wb := NewWriteBatch()
	defer wb.Destroy()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var k [8]byte
		binary.BigEndian.PutUint64(k[:], uint64(i))
		wb.Put(k[:], k[:])
		if i%1024 == 1023 {
			wb.Clear()
		}
	}
}

func BenchmarkIteratorScan(b *testing.B) {
	const n = 1 << 12
	db := benchDB(b, false)
	benchFill(b, db, n)
	ro := NewDefaultReadOptions()
	defer ro.Destroy()
	it := db.NewIterator(ro)
	defer it.Close()
	b.ReportAllocs()
	b.ResetTimer()
	it.SeekToFirst()
	for i := 0; i < b.N; i++ {
		if !it.Valid() {
			it.SeekToFirst()
		}
		k, v := it.Key(), it.Value()
		_, _ = k.Data(), v.Data()
		it.Next()
	}
}
