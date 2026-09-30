# grocksdb, RocksDB wrapper for Go

[![](https://github.com/linxGnu/grocksdb/workflows/CI/badge.svg)]()
[![Go Report Card](https://goreportcard.com/badge/github.com/linxGnu/grocksdb)](https://goreportcard.com/report/github.com/linxGnu/grocksdb)
[![Coverage Status](https://coveralls.io/repos/github/linxGnu/grocksdb/badge.svg?branch=master)](https://coveralls.io/github/linxGnu/grocksdb?branch=master)
[![godoc](https://img.shields.io/badge/docs-GoDoc-green.svg)](https://godoc.org/github.com/linxGnu/grocksdb)

This is a `Fork` from [tecbot/gorocksdb](https://github.com/tecbot/gorocksdb). I respect the author work and community contribution.
The `LICENSE` still remains as upstream.

Why I made a patched clone instead of PR:
- Supports almost C API (unlike upstream). Catching up with latest version of Rocksdb as promise.
- This fork contains `no defer` in codebase (my side project requires as less overhead as possible). This introduces loose
convention of how/when to free c-mem, thus break the rule of [tecbot/gorocksdb](https://github.com/tecbot/gorocksdb).

## Install

### Prerequisite 

- librocksdb
- libsnappy
- libz
- liblz4
- libzstd
- libbz2 (optional)

Please follow this guide: https://github.com/facebook/rocksdb/blob/master/INSTALL.md to build above libs.

### Build 

After installing both `rocksdb` and `grocksdb`, you can build your app using the following commands:

```bash
CGO_CFLAGS="-I/path/to/rocksdb/include" \
CGO_LDFLAGS="-L/path/to/rocksdb -lrocksdb -lstdc++ -lm -lz -lsnappy -llz4 -lzstd" \
  go build
```

Or just:
```bash
go build // if prerequisites are in linker paths
```

If your rocksdb was linked with bz2:
```bash
CGO_LDFLAGS="-L/path/to/rocksdb -lrocksdb -lstdc++ -lm -lz -lsnappy -llz4 -lzstd -lbz2" \
  go build
```

#### Customize the build flags
Currently, the default build flags without specifying `CGO_LDFLAGS` or the corresponding environment variables are `-lrocksdb -pthread -lstdc++ -ldl -lm -lzstd -llz4 -lz -lsnappy`

If you want to customize the build flags:

1. Use `-tags grocksdb_clean_link` to create a cleaner set of flags and build it based on the cleaner flag. The base build flags after using the tag are `-lrocksdb -pthread -lstdc++ -ldl`.
```bash
CGO_LDFLAGS="-L/path/to/rocksdb -lzstd" go build -tags grocksdb_clean_link
```
2. Use `-tags grocksdb_no_link` to ignore the build flags provided by the library and build it fully based on the custom flags.
```bash
CGO_LDFLAGS="-L/path/to/rocksdb -lrocksdb -lstdc++ -lzstd -llz4" go build -tags grocksdb_clean_link
```

#### cgo `noescape`/`nocallback` annotations
Since Go 1.24, grocksdb marks hot C calls with `#cgo noescape` and `#cgo nocallback`, so their arguments can stay on the stack instead of the heap.

- Always on: `WriteBatchFrom`/builder materialization, `WriteBatch` mutations, iterator `Key`/`Value`/`Timestamp`/`Err`, and pinnable slice data.
- Opt-in: `Get`, `GetCF`, `GetPinned`, `GetPinnedCF`, `Put`, `PutCF`, `Delete`, `DeleteCF`, `SingleDelete`, `SingleDeleteCF`, `Merge`, `MergeCF`, `Write` and `GetIntoBuffer`/`GetIntoBufferCF`. Enable with `opts.SetNoGoCallbacks(true)` when the DB uses no Go comparator, merge operator, prefix extractor or compaction filter. Opening the DB fails with `ErrGoCallbacksNotAllowed` otherwise.
- Build with `-tags grocksdb_no_cgo_annotations` to turn the annotations off.

## Usage

See also: [doc](https://godoc.org/github.com/linxGnu/grocksdb)

### Fewer cgo calls on hot paths

For scans, `KeySlice()` and `ValueSlice()` return `OptimizedSlice` values and
avoid the Go wrapper allocations made by `Key()` and `Value()`. For fewer cgo
crossings, `ReadKV()` reads validity, key and value together, and `NextKV()`
advances first and reads the new pair together:

```go
it := db.NewIterator(ro) // NewIteratorCF and NewIterators work too
// Close before closing the DB, and keep ro alive until then.
defer it.Close()
it.SeekToFirst()
for key, value, valid := it.ReadKV(); valid; key, value, valid = it.NextKV() {
    process(key.Data(), value.Data())
}
if err := it.Err(); err != nil {
    return err
}
```

All iterator slices borrow native storage. Do not modify them; copy any data
needed after an iterator movement, `Refresh`, or `Close`. `ReadKV` does not move;
`NextKV` on an invalid iterator returns false without moving. Seek before reading
and check `Err` after a scan. These methods work with Go callbacks; the gain comes
from one cgo crossing per entry instead of four.

`GetIntoBuffer(ro, key, buffer)` and `GetIntoBufferCF(ro, cf, key, buffer)` copy a
value into caller-owned memory without a native value allocation or a `Free` call:

```go
buffer := make([]byte, 1024)
size, found, err := db.GetIntoBuffer(ro, key, buffer)
if err != nil {
    return err
}
if found && size > len(buffer) {
    buffer = make([]byte, size)
    size, found, err = db.GetIntoBuffer(ro, key, buffer)
    if err != nil {
        return err
    }
}
if found && size <= len(buffer) {
    process(buffer[:size])
}
```

The usable buffer size is its **length**, not capacity. A missing key has
`found=false, size=0`; an empty value has `found=true, size=0`, even with a nil
buffer. An undersized buffer is untouched, returns the full required size and
`found=true`, and is not an error. A retry is a separate read: use a snapshot for
stable results, or handle another size change. This removes the C API's output
malloc/copy/free, but still copies into your buffer; RocksDB can allocate
internally. With `SetNoGoCallbacks(true)`, the wrapper's output arguments stay on
the stack, and reads with reused key/buffer memory have zero Go allocations. The
public callback-capable dispatch still makes the compiler conservatively escape
caller key/buffer memory; this API does not promise that fresh caller buffers
stay on the stack.

`WriteBatchBuilder` encodes ordinary records in Go and materializes a native batch
in one cgo call. Its zero value works, and `Clear` retains capacity for reuse:

```go
var builder grocksdb.WriteBatchBuilder
cfID := cf.ID() // Cache once; calling ID inside the record loop crosses cgo.
builder.Put(key, value)
builder.PutCF(cfID, otherKey, otherValue)
batch := builder.Build()
defer batch.Destroy()
if err := db.Write(wo, batch); err != nil {
    return err
}
builder.Clear()
```

The builder copies inputs and supports put, merge, delete, single delete, delete
range, their CF variants, and log data. CF methods accept `uint32` IDs belonging
to the receiving DB; ID 0 means the default CF. Single-delete restrictions are the
same as `WriteBatch.SingleDelete`. `Build` copies into an independent native batch;
the builder needs no `Destroy`. It is not safe for concurrent use. Timestamps,
per-key protection bytes, transaction markers and savepoints are excluded; use
`WriteBatch` when those are needed. Encoding follows RocksDB 11.1.2 and is tested
byte-for-byte against native batches.

**Reuse the builder** across batches (`Clear` between them): that's where the gain
comes from. In local benchmarks a reused builder was 2.6–3.6× faster than
`WriteBatch` from 16 records per batch. A fresh builder per batch can be slower
than `WriteBatch`, and single-record batches should stay on `WriteBatch`.

`MultiGet`, `MultiGetCF`, `BatchedMultiGetCF` and their transaction variants copy
all keys into one C allocation instead of one per key.

Prefer native comparators and merge operators on hot paths when they provide
your required semantics. Go implementations cross back into Go during reads,
writes and iterator movement; use the default callback-capable mode for them.
`SetNoGoCallbacks(true)` is a promise about every DB/CF callback, including
callbacks hidden behind native objects; violating it can panic.

## API Support

Almost C API, excepts:
- [ ] get_db_identity
- [ ] putv/mergev/deletev/delete_rangev
- [ ] compaction_filter/compaction_filter_factory/compaction_filter_context
- [ ] transactiondb_property_value/transactiondb_property_int
- [ ] optimistictransactiondb_property_value/optimistictransactiondb_property_int
- [ ] writebatch_update_timestamps/writebatch_wi_update_timestamps/writebatch_iterate_cf
- [ ] approximate_sizes_cf_with_flags
- [ ] logger_create_callback_logger
- [ ] event listeners:
  - [ ] flushjobinfo*
  - [ ] compactionjobinfo*
  - [ ] subcompactionjobinfo*
  - [ ] externalfileingestioninfo*
  - [ ] writestallinfo*
  - [ ] memtableinfo*
  - [ ] onbackground_error_cb
- [ ] compactionservice
- [ ] rocksdb_open_and_compact*
- [ ] rocksdb_writebatch_iterate*
- [ ] rocksdb_compaction_service_options_override_t
- [ ] rocksdb_file_checksum_gen_factory_t
- [ ] rocksdb_sst_partitioner_factory_t
- [ ] rocksdb_table_properties_collector_factory_t


