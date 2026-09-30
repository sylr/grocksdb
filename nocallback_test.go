package grocksdb

import (
	"bytes"
	"os"
	"regexp"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// Reviewed list of symbols annotated in cgo_annotations.go. Adding a symbol means
// checking in db/c.cc, and in the C++ it forwards to, that it neither keeps a pointer
// argument after returning nor calls into Go on the calling thread.
var cgoAnnotationsAllowlist = []string{
	// Only append to WriteBatch::rep.
	"rocksdb_writebatch_put",
	"rocksdb_writebatch_put_cf",
	"rocksdb_writebatch_put_cf_with_ts",
	"rocksdb_writebatch_merge",
	"rocksdb_writebatch_merge_cf",
	"rocksdb_writebatch_delete",
	"rocksdb_writebatch_delete_cf",
	"rocksdb_writebatch_delete_cf_with_ts",
	"rocksdb_writebatch_singledelete",
	"rocksdb_writebatch_singledelete_cf",
	"rocksdb_writebatch_singledelete_cf_with_ts",
	"rocksdb_writebatch_delete_range",
	"rocksdb_writebatch_delete_range_cf",
	"rocksdb_writebatch_put_log_data",
	"rocksdb_writebatch_data",
	// Return state already cached by the iterator or pinnable slice.
	"rocksdb_iter_key",
	"rocksdb_iter_value",
	"rocksdb_iter_timestamp",
	"rocksdb_iter_get_error",
	"rocksdb_pinnableslice_value",
	"rocksdb_pinnable_handle_get_value",
	// Forwarders, only called on DBs opened with SetNoGoCallbacks(true).
	"grocksdb_nocb_get",
	"grocksdb_nocb_get_cf",
	"grocksdb_nocb_get_pinned",
	"grocksdb_nocb_get_pinned_cf",
	"grocksdb_nocb_put",
	"grocksdb_nocb_put_cf",
	"grocksdb_nocb_delete",
	"grocksdb_nocb_delete_cf",
	"grocksdb_nocb_singledelete",
	"grocksdb_nocb_singledelete_cf",
	"grocksdb_nocb_merge",
	"grocksdb_nocb_merge_cf",
	"grocksdb_nocb_write",
}

// These keep the pointer they are given (rocksdb_readoptions_t stores a Slice to it),
// so they must never be marked noescape.
var cgoAnnotationsDenylist = []string{
	"rocksdb_readoptions_set_iterate_upper_bound",
	"rocksdb_readoptions_set_iterate_lower_bound",
	"rocksdb_readoptions_set_timestamp",
	"rocksdb_readoptions_set_iter_start_ts",
}

func TestCgoAnnotations(t *testing.T) {
	src, err := os.ReadFile("cgo_annotations.go")
	require.NoError(t, err)

	collect := func(directive string) []string {
		var names []string
		re := regexp.MustCompile(`(?m)^// #cgo ` + directive + ` (\w+)$`)
		for _, m := range re.FindAllSubmatch(src, -1) {
			names = append(names, string(m[1]))
		}
		sort.Strings(names)
		return names
	}
	noEscape, noCallback := collect("noescape"), collect("nocallback")

	// cgo only avoids the heap when both are present, so they must always be paired.
	require.Equal(t, noEscape, noCallback)

	want := append([]string(nil), cgoAnnotationsAllowlist...)
	sort.Strings(want)
	require.Equal(t, want, noEscape)

	for _, name := range cgoAnnotationsDenylist {
		require.NotContains(t, noEscape, name)
	}
}

func newNoGoCallbacksOptions() *Options {
	opts := NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	opts.SetCreateIfMissingColumnFamilies(true)
	opts.SetNoGoCallbacks(true)
	return opts
}

func TestNoGoCallbacksRejectsGoCallbacks(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*Options){
		"comparator": func(o *Options) {
			o.SetComparator(NewComparator("grocksdb.test", bytes.Compare))
		},
		"merge operator": func(o *Options) {
			o.SetMergeOperator(&mockMergeOperator{})
		},
		"prefix extractor": func(o *Options) {
			o.SetPrefixExtractor(&testSliceTransform{})
		},
		"compaction filter": func(o *Options) {
			o.SetCompactionFilter(&mockCompactionFilter{})
		},
	}

	for name, install := range cases {
		t.Run(name, func(t *testing.T) {
			// On the DB options.
			opts := newNoGoCallbacksOptions()
			install(opts)
			_, err := OpenDb(opts, t.TempDir())
			require.ErrorIs(t, err, ErrGoCallbacksNotAllowed)

			// On a column family's options.
			cfOpts := NewDefaultOptions()
			install(cfOpts)
			_, _, err = OpenDbColumnFamilies(newNoGoCallbacksOptions(), t.TempDir(),
				[]string{"default", "cf"}, []*Options{NewDefaultOptions(), cfOpts})
			require.ErrorIs(t, err, ErrGoCallbacksNotAllowed)

			// Through GetOptionsFromString, which copies the base natively.
			base := newNoGoCallbacksOptions()
			install(base)
			derived, err := GetOptionsFromString(base, "write_buffer_size=1048576")
			require.NoError(t, err)
			_, err = OpenDb(derived, t.TempDir())
			require.ErrorIs(t, err, ErrGoCallbacksNotAllowed)

			// Through Clone.
			_, err = OpenDb(base.Clone(), t.TempDir())
			require.ErrorIs(t, err, ErrGoCallbacksNotAllowed)

			// On a column family created after open.
			db, err := OpenDb(newNoGoCallbacksOptions(), t.TempDir())
			require.NoError(t, err)
			defer db.Close()

			_, err = db.CreateColumnFamily(cfOpts, "later")
			require.ErrorIs(t, err, ErrGoCallbacksNotAllowed)
			_, err = db.CreateColumnFamilies(cfOpts, []string{"later"})
			require.ErrorIs(t, err, ErrGoCallbacksNotAllowed)
			_, err = db.CreateColumnFamilyWithTTL(cfOpts, "later", 10)
			require.ErrorIs(t, err, ErrGoCallbacksNotAllowed)
		})
	}
}

// Exercises every fast path on an opted-in DB, native callbacks only.
func TestNoGoCallbacksFastPaths(t *testing.T) {
	t.Parallel()

	opts := newNoGoCallbacksOptions()
	opts.SetUint64AddMergeOperator() // native, allowed
	opts.SetPrefixExtractor(NewFixedPrefixTransform(3))

	db, cfs, err := OpenDbColumnFamilies(opts, t.TempDir(),
		[]string{"default", "cf"}, []*Options{opts, opts})
	require.NoError(t, err)
	defer db.Close()
	require.True(t, db.noGoCallbacks)
	cf := cfs[1]

	later, err := db.CreateColumnFamily(NewDefaultOptions(), "later")
	require.NoError(t, err)
	later.Destroy()

	wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()

	require.NoError(t, db.Put(wo, []byte("key1"), []byte("v1")))
	require.NoError(t, db.PutCF(wo, cf, []byte("key1"), []byte("cf1")))

	v, err := db.Get(ro, []byte("key1"))
	require.NoError(t, err)
	require.Equal(t, []byte("v1"), v.Data())
	v.Free()

	v, err = db.GetCF(ro, cf, []byte("key1"))
	require.NoError(t, err)
	require.Equal(t, []byte("cf1"), v.Data())
	v.Free()

	p, err := db.GetPinned(ro, []byte("key1"))
	require.NoError(t, err)
	require.Equal(t, []byte("v1"), p.Data())
	p.Destroy()

	p, err = db.GetPinnedCF(ro, cf, []byte("key1"))
	require.NoError(t, err)
	require.Equal(t, []byte("cf1"), p.Data())
	p.Destroy()

	one := []byte{1, 0, 0, 0, 0, 0, 0, 0}
	require.NoError(t, db.Merge(wo, []byte("ctr"), one))
	require.NoError(t, db.Merge(wo, []byte("ctr"), one))
	require.NoError(t, db.MergeCF(wo, cf, []byte("ctr"), one))
	v, err = db.Get(ro, []byte("ctr"))
	require.NoError(t, err)
	require.Equal(t, []byte{2, 0, 0, 0, 0, 0, 0, 0}, v.Data())
	v.Free()

	wb := NewWriteBatch()
	wb.Put([]byte("key2"), []byte("v2"))
	wb.PutCF(cf, []byte("key2"), []byte("cf2"))
	require.NoError(t, db.Write(wo, wb))
	wb.Destroy()

	require.NoError(t, db.Delete(wo, []byte("key1")))
	require.NoError(t, db.DeleteCF(wo, cf, []byte("key1")))
	require.NoError(t, db.SingleDelete(wo, []byte("key2")))
	require.NoError(t, db.SingleDeleteCF(wo, cf, []byte("key2")))

	for _, k := range []string{"key1", "key2"} {
		v, err = db.Get(ro, []byte(k))
		require.NoError(t, err)
		require.False(t, v.Exists())
		v.Free()
	}
}

// Go callbacks keep working on DBs that did not opt in, including through the Class A
// functions, which are annotated unconditionally. A wrong annotation panics here.
func TestGoCallbacksStillWork(t *testing.T) {
	t.Parallel()

	var compares, merges, transforms atomic.Int64
	db := newTestDB(t, func(opts *Options) {
		opts.SetComparator(NewComparator("grocksdb.counting", func(a, b []byte) int {
			compares.Add(1)
			return bytes.Compare(a, b)
		}))
		opts.SetMergeOperator(&mockMergeOperator{
			fullMerge: func(_, existing []byte, operands [][]byte) ([]byte, bool) {
				merges.Add(1)
				out := append([]byte(nil), existing...)
				for _, op := range operands {
					out = append(out, op...)
				}
				return out, true
			},
		})
		opts.SetPrefixExtractor(&countingSliceTransform{calls: &transforms})
		opts.SetMemTablePrefixBloomSizeRatio(0.1)
	})
	defer db.Close()
	require.False(t, db.noGoCallbacks)

	wo, ro := NewDefaultWriteOptions(), NewDefaultReadOptions()

	wb := NewWriteBatch()
	for _, k := range []string{"aaa1", "aaa2", "bbb1", "bbb2"} {
		wb.Put([]byte(k), []byte("x"))
	}
	wb.Merge([]byte("aaa1"), []byte("y"))
	require.NotEmpty(t, wb.Data())
	require.NoError(t, db.Write(wo, wb))
	wb.Destroy()

	require.NoError(t, db.Put(wo, []byte("ccc1"), []byte("z")))

	v, err := db.Get(ro, []byte("aaa1"))
	require.NoError(t, err)
	require.Equal(t, []byte("xy"), v.Data())
	v.Free()

	p, err := db.GetPinned(ro, []byte("bbb1"))
	require.NoError(t, err)
	require.Equal(t, []byte("x"), p.Data())
	p.Destroy()

	iter := db.NewIterator(ro)
	n := 0
	for iter.SeekToFirst(); iter.Valid(); iter.Next() {
		k, val := iter.Key(), iter.Value()
		require.NotEmpty(t, k.Data())
		require.NotEmpty(t, val.Data())
		k.Free()
		val.Free()
		n++
	}
	require.NoError(t, iter.Err())
	iter.Close()
	require.Equal(t, 5, n)

	require.Positive(t, compares.Load(), "comparator never ran")
	require.Positive(t, merges.Load(), "merge operator never ran")
	require.Positive(t, transforms.Load(), "prefix extractor never ran")
}

type countingSliceTransform struct {
	calls *atomic.Int64
}

func (st *countingSliceTransform) Name() string { return "grocksdb.counting" }
func (st *countingSliceTransform) Transform(src []byte) []byte {
	st.calls.Add(1)
	return src[0:3]
}
func (st *countingSliceTransform) InDomain(src []byte) bool { return len(src) >= 3 }
func (st *countingSliceTransform) InRange(src []byte) bool  { return len(src) == 3 }
func (st *countingSliceTransform) Destroy()                 {}

// Checks that WriteBatch really copies its input: the caller's buffer is reused
// right after each call.
func TestWriteBatchCopiesInput(t *testing.T) {
	t.Parallel()

	wb := NewWriteBatch()
	defer wb.Destroy()

	var buf [8]byte
	for i := byte(0); i < 4; i++ {
		buf = [8]byte{'k', i, 'v', i}
		wb.Put(buf[:2], buf[2:4])
	}
	buf = [8]byte{}

	it := wb.NewIterator()
	var i byte
	for it.Next() {
		rec := it.Record()
		require.Equal(t, []byte{'k', i}, rec.Key)
		require.Equal(t, []byte{'v', i}, rec.Value)
		i++
	}
	require.Equal(t, byte(4), i)
}
