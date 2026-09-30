package grocksdb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteBatchBuilderNativeEncoding(t *testing.T) {
	t.Parallel()
	_, cfs, cleanup := newBuilderTestDBCF(t, nil)
	defer cleanup()
	for _, size := range []int{0, 1, 127, 128, 16383, 16384} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			key, value := bytes.Repeat([]byte{0x80}, size), bytes.Repeat([]byte{0xff}, size)
			native := NewWriteBatch()
			defer native.Destroy()
			var builder WriteBatchBuilder
			check := func() {
				built := builder.Build()
				defer built.Destroy()
				require.Equal(t, native.Data(), built.Data())
				require.Equal(t, native.Count(), builder.Count())
			}
			check()
			native.Put(key, value)
			builder.Put(key, value)
			check()
			native.Merge(key, value)
			builder.Merge(key, value)
			check()
			native.Delete(key)
			builder.Delete(key)
			check()
			native.SingleDelete(key)
			builder.SingleDelete(key)
			check()
			native.DeleteRange(key, value)
			builder.DeleteRange(key, value)
			check()
			for _, cf := range cfs {
				id := cf.ID()
				native.PutCF(cf, key, value)
				builder.PutCF(id, key, value)
				check()
				native.MergeCF(cf, key, value)
				builder.MergeCF(id, key, value)
				check()
				native.DeleteCF(cf, key)
				builder.DeleteCF(id, key)
				check()
				native.SingleDeleteCF(cf, key)
				builder.SingleDeleteCF(id, key)
				check()
				native.DeleteRangeCF(cf, key, value)
				builder.DeleteRangeCF(id, key, value)
				check()
			}
			native.PutLogData(value)
			builder.PutLogData(value)
			check()
		})
	}
}

func TestWriteBatchBuilderCFVarints(t *testing.T) {
	for _, id := range []uint32{127, 128, 16383, 16384, math.MaxUint32} {
		var builder WriteBatchBuilder
		builder.PutCF(id, []byte("key"), []byte("value"))
		built := builder.Build()
		iter := built.NewIterator()
		require.True(t, iter.Next())
		require.Equal(t, id, uint32(iter.Record().CF))
		require.Equal(t, []byte("key"), iter.Record().Key)
		require.Equal(t, []byte("value"), iter.Record().Value)
		require.False(t, iter.Next())
		require.NoError(t, iter.Error())
		built.Destroy()
	}
}

func TestWriteBatchBuilderNativeCFVarint(t *testing.T) {
	names := make([]string, 129)
	names[0] = "default"
	for i := 1; i < len(names); i++ {
		names[i] = fmt.Sprintf("cf%d", i)
	}
	_, cfs, cleanup := newBuilderTestDBCF(t, nil, names...)
	defer cleanup()
	cf := cfs[128]
	require.GreaterOrEqual(t, cf.ID(), uint32(128))
	var builder WriteBatchBuilder
	native := NewWriteBatch()
	defer native.Destroy()
	key, value := []byte("key"), []byte("value")
	builder.PutCF(cf.ID(), key, value)
	native.PutCF(cf, key, value)
	builder.MergeCF(cf.ID(), key, value)
	native.MergeCF(cf, key, value)
	builder.DeleteCF(cf.ID(), key)
	native.DeleteCF(cf, key)
	builder.SingleDeleteCF(cf.ID(), key)
	native.SingleDeleteCF(cf, key)
	builder.DeleteRangeCF(cf.ID(), key, value)
	native.DeleteRangeCF(cf, key, value)
	built := builder.Build()
	defer built.Destroy()
	require.Equal(t, native.Data(), built.Data())
}

func TestWriteBatchBuilderReuse(t *testing.T) {
	builder := NewWriteBatchBuilder()
	key, value := []byte("key"), []byte("value")
	builder.Put(key, value)
	key[0], value[0] = 'x', 'x'
	first := builder.Build()
	defer first.Destroy()
	want := bytes.Clone(first.Data())
	capacity := cap(builder.data)
	builder.Clear()
	require.Equal(t, capacity, cap(builder.data))
	require.Zero(t, builder.Count())
	empty := builder.Build()
	require.Equal(t, make([]byte, 12), empty.Data())
	empty.Destroy()
	builder.Put([]byte("next"), []byte("record"))
	require.Equal(t, want, first.Data())
	iter := first.NewIterator()
	require.True(t, iter.Next())
	require.Equal(t, []byte("key"), iter.Record().Key)
	require.Equal(t, []byte("value"), iter.Record().Value)
	second := builder.Build()
	defer second.Destroy()
	builder.PutLogData(nil)
	require.Equal(t, 1, second.Count())
	require.Equal(t, 1, builder.Count())
	logOnly := NewWriteBatchBuilder()
	logOnly.PutLogData(nil)
	require.Zero(t, logOnly.Count())
	logBatch := logOnly.Build()
	require.Zero(t, logBatch.Count())
	logBatch.Destroy()
}

func TestWriteBatchBuilderCountOverflow(t *testing.T) {
	b := NewWriteBatchBuilder()
	b.count = math.MaxUint32
	binary.LittleEndian.PutUint32(b.data[8:12], b.count)
	want := bytes.Clone(b.data)
	require.Panics(t, func() { b.Put(nil, nil) })
	require.Equal(t, want, b.data)
}

func TestWriteBatchBuilderLargeCount(t *testing.T) {
	var builder WriteBatchBuilder
	native := NewWriteBatch()
	defer native.Destroy()
	for range 300 {
		builder.Put(nil, nil)
		native.Put(nil, nil)
	}
	built := builder.Build()
	defer built.Destroy()
	require.Equal(t, 300, builder.Count())
	require.Equal(t, native.Data(), built.Data())
}

func TestWriteBatchBuilderDB(t *testing.T) {
	t.Parallel()
	merger := &mockMergeOperator{fullMerge: func(_ []byte, existing []byte, operands [][]byte) ([]byte, bool) {
		result := bytes.Clone(existing)
		for _, operand := range operands {
			result = append(result, operand...)
		}
		return result, true
	}}
	db, cfs, cleanup := newBuilderTestDBCF(t, func(opts *Options) { opts.SetMergeOperator(merger) })
	defer cleanup()
	wo := NewDefaultWriteOptions()
	defer wo.Destroy()
	ro := NewDefaultReadOptions()
	defer ro.Destroy()
	var builder WriteBatchBuilder
	for _, cf := range cfs {
		id := cf.ID()
		builder.PutCF(id, []byte("keep"), []byte("value"))
		builder.PutCF(id, []byte("merge"), []byte("a"))
		builder.MergeCF(id, []byte("merge"), []byte("b"))
		builder.PutCF(id, []byte("delete"), []byte("gone"))
		builder.DeleteCF(id, []byte("delete"))
		builder.PutCF(id, []byte("single"), []byte("gone"))
		builder.SingleDeleteCF(id, []byte("single"))
		builder.PutCF(id, []byte("range1"), []byte("gone"))
		builder.DeleteRangeCF(id, []byte("range"), []byte("ranges"))
	}
	builder.PutLogData([]byte("log"))
	batch := builder.Build()
	defer batch.Destroy()
	require.NoError(t, db.Write(wo, batch))
	for _, cf := range cfs {
		for _, entry := range []struct{ key, value string }{{"keep", "value"}, {"merge", "ab"}, {"delete", ""}, {"single", ""}, {"range1", ""}} {
			value, err := db.GetCF(ro, cf, []byte(entry.key))
			require.NoError(t, err)
			require.Equal(t, entry.value, string(value.Data()))
			value.Free()
		}
	}
}

func newBuilderTestDBCF(t *testing.T, configure func(*Options), names ...string) (*DB, []*ColumnFamilyHandle, func()) {
	t.Helper()
	opts := NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetCreateIfMissingColumnFamilies(true)
	if configure != nil {
		configure(opts)
	}
	if len(names) == 0 {
		names = []string{"default", "custom"}
	}
	cfOpts := make([]*Options, len(names))
	for i := range cfOpts {
		cfOpts[i] = opts
	}
	db, cfs, err := OpenDbColumnFamilies(opts, t.TempDir(), names, cfOpts)
	require.NoError(t, err)
	return db, cfs, func() {
		for _, cf := range cfs {
			cf.Destroy()
		}
		db.Close()
		// RocksDB's options shared_ptr owns the merge operator installed above.
		// Avoid the existing Options.Destroy path deleting that pointer twice.
		opts.cmo = nil
		opts.Destroy()
	}
}

func BenchmarkCallReductionWriteBatch(b *testing.B) {
	key, value := []byte("benchmark-key"), bytes.Repeat([]byte("v"), 64)
	for _, count := range []int{1, 16, 128, 1024} {
		b.Run(fmt.Sprintf("entries=%d/Native", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				batch := NewWriteBatch()
				for range count {
					batch.Put(key, value)
				}
				batch.Destroy()
			}
		})
		b.Run(fmt.Sprintf("entries=%d/Builder", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				builder := NewWriteBatchBuilder()
				for range count {
					builder.Put(key, value)
				}
				batch := builder.Build()
				batch.Destroy()
			}
		})
		b.Run(fmt.Sprintf("entries=%d/NativeReuse", count), func(b *testing.B) {
			batch := NewWriteBatch()
			defer batch.Destroy()
			b.ReportAllocs()
			for b.Loop() {
				batch.Clear()
				for range count {
					batch.Put(key, value)
				}
			}
		})
		b.Run(fmt.Sprintf("entries=%d/BuilderReuse", count), func(b *testing.B) {
			builder := NewWriteBatchBuilder()
			b.ReportAllocs()
			for b.Loop() {
				builder.Clear()
				for range count {
					builder.Put(key, value)
				}
				batch := builder.Build()
				batch.Destroy()
			}
		})
	}
}
