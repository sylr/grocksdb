//go:build !grocksdb_no_cgo_annotations

// cgo only keeps Go pointer arguments off the heap when a C function is marked both
// noescape and nocallback. Every symbol below must satisfy both contracts:
//
//   - noescape: the function does not keep any pointer argument after it returns.
//     cgo can't check this; a wrong entry means memory corruption.
//   - nocallback: the function never calls into Go on the calling thread. The runtime
//     panics if it does.
//
// Build with -tags grocksdb_no_cgo_annotations to turn all of this off.
//
// Any change here must update TestCgoAnnotations (cgo_annotations_test.go), and
// RocksDB upgrades must re-check each entry against db/c.cc.

package grocksdb

// /* Always safe: these only append to the batch's own buffer, or read state the
//    iterator/pinnable slice has already cached. They never reach user code. */
// /* c.cc create_from copies rep into std::string/WriteBatch; no callbacks. */
// #cgo noescape rocksdb_writebatch_create_from
// #cgo nocallback rocksdb_writebatch_create_from
// #cgo noescape rocksdb_writebatch_put
// #cgo nocallback rocksdb_writebatch_put
// #cgo noescape rocksdb_writebatch_put_cf
// #cgo nocallback rocksdb_writebatch_put_cf
// #cgo noescape rocksdb_writebatch_put_cf_with_ts
// #cgo nocallback rocksdb_writebatch_put_cf_with_ts
// #cgo noescape rocksdb_writebatch_merge
// #cgo nocallback rocksdb_writebatch_merge
// #cgo noescape rocksdb_writebatch_merge_cf
// #cgo nocallback rocksdb_writebatch_merge_cf
// #cgo noescape rocksdb_writebatch_delete
// #cgo nocallback rocksdb_writebatch_delete
// #cgo noescape rocksdb_writebatch_delete_cf
// #cgo nocallback rocksdb_writebatch_delete_cf
// #cgo noescape rocksdb_writebatch_delete_cf_with_ts
// #cgo nocallback rocksdb_writebatch_delete_cf_with_ts
// #cgo noescape rocksdb_writebatch_singledelete
// #cgo nocallback rocksdb_writebatch_singledelete
// #cgo noescape rocksdb_writebatch_singledelete_cf
// #cgo nocallback rocksdb_writebatch_singledelete_cf
// #cgo noescape rocksdb_writebatch_singledelete_cf_with_ts
// #cgo nocallback rocksdb_writebatch_singledelete_cf_with_ts
// #cgo noescape rocksdb_writebatch_delete_range
// #cgo nocallback rocksdb_writebatch_delete_range
// #cgo noescape rocksdb_writebatch_delete_range_cf
// #cgo nocallback rocksdb_writebatch_delete_range_cf
// #cgo noescape rocksdb_writebatch_put_log_data
// #cgo nocallback rocksdb_writebatch_put_log_data
// #cgo noescape rocksdb_writebatch_data
// #cgo nocallback rocksdb_writebatch_data
// #cgo noescape rocksdb_iter_key
// #cgo nocallback rocksdb_iter_key
// #cgo noescape rocksdb_iter_value
// #cgo nocallback rocksdb_iter_value
// #cgo noescape rocksdb_iter_timestamp
// #cgo nocallback rocksdb_iter_timestamp
// #cgo noescape rocksdb_iter_get_error
// #cgo nocallback rocksdb_iter_get_error
// #cgo noescape rocksdb_pinnableslice_value
// #cgo nocallback rocksdb_pinnableslice_value
// #cgo noescape rocksdb_pinnable_handle_get_value
// #cgo nocallback rocksdb_pinnable_handle_get_value
//
// /* Forwarders to calls that can reach Go comparators, merge operators, prefix
//    extractors or compaction filters. They are used only on DBs opened with
//    Options.SetNoGoCallbacks(true). See grocksdb.h. */
// #cgo noescape grocksdb_nocb_get
// #cgo nocallback grocksdb_nocb_get
// #cgo noescape grocksdb_nocb_get_cf
// #cgo nocallback grocksdb_nocb_get_cf
// #cgo noescape grocksdb_nocb_get_pinned
// #cgo nocallback grocksdb_nocb_get_pinned
// #cgo noescape grocksdb_nocb_get_pinned_cf
// #cgo nocallback grocksdb_nocb_get_pinned_cf
// #cgo noescape grocksdb_nocb_put
// #cgo nocallback grocksdb_nocb_put
// #cgo noescape grocksdb_nocb_put_cf
// #cgo nocallback grocksdb_nocb_put_cf
// #cgo noescape grocksdb_nocb_delete
// #cgo nocallback grocksdb_nocb_delete
// #cgo noescape grocksdb_nocb_delete_cf
// #cgo nocallback grocksdb_nocb_delete_cf
// #cgo noescape grocksdb_nocb_singledelete
// #cgo nocallback grocksdb_nocb_singledelete
// #cgo noescape grocksdb_nocb_singledelete_cf
// #cgo nocallback grocksdb_nocb_singledelete_cf
// #cgo noescape grocksdb_nocb_merge
// #cgo nocallback grocksdb_nocb_merge
// #cgo noescape grocksdb_nocb_merge_cf
// #cgo nocallback grocksdb_nocb_merge_cf
// #cgo noescape grocksdb_nocb_write
// #cgo nocallback grocksdb_nocb_write
//
// /* c.cc buffer reads use a local PinnableSlice and copy synchronously. Neither
//    retains Go arguments; these forwarders are called only when no Go callbacks
//    can run. */
// #cgo noescape grocksdb_nocb_get_into_buffer
// #cgo nocallback grocksdb_nocb_get_into_buffer
// #cgo noescape grocksdb_nocb_get_into_buffer_cf
// #cgo nocallback grocksdb_nocb_get_into_buffer_cf
//
// #include "grocksdb.h"
import "C"
