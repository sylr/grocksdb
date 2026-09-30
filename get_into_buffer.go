package grocksdb

// #include "grocksdb.h"
import "C"

// GetIntoBuffer reads a value into buffer, using its length (not capacity).
// size is the full value length; found distinguishes a missing key from an empty
// value. If size > len(buffer), found is true and buffer is unchanged: allocate a
// larger buffer and retry. A retry is a separate read; use a snapshot if needed.
// On a missing key or error, size and found are zero and buffer is unchanged.
// No native value needs freeing. RocksDB does not retain key or buffer.
func (db *DB) GetIntoBuffer(opts *ReadOptions, key, buffer []byte) (size int, found bool, err error) {
	if !db.noGoCallbacks {
		return db.getIntoBufferSlow(opts, key, buffer)
	}

	var cSize C.size_t
	var cFound C.uchar
	var cErr *C.char
	// Even for an empty value, c.cc calls memcpy on success. Supply a non-null
	// destination for zero-length buffers rather than rely on memcpy(NULL, ..., 0).
	var empty [1]byte
	cBuffer := refGoBytes(buffer)
	if len(buffer) == 0 {
		cBuffer = refGoBytes(empty[:])
	}
	C.grocksdb_nocb_get_into_buffer(db.c, opts.c, refGoBytes(key), C.size_t(len(key)), cBuffer, C.size_t(len(buffer)), &cSize, &cFound, &cErr)

	return int(cSize), cFound != 0, fromCError(cErr)
}

// getIntoBufferSlow isolates escaping output arguments from the annotated path.
func (db *DB) getIntoBufferSlow(opts *ReadOptions, key, buffer []byte) (size int, found bool, err error) {
	var cSize C.size_t
	var cFound C.uchar
	var cErr *C.char
	// Even for an empty value, c.cc calls memcpy on success. Supply a non-null
	// destination for zero-length buffers rather than rely on memcpy(NULL, ..., 0).
	var empty [1]byte
	cBuffer := refGoBytes(buffer)
	if len(buffer) == 0 {
		cBuffer = refGoBytes(empty[:])
	}
	C.rocksdb_get_into_buffer(db.c, opts.c, refGoBytes(key), C.size_t(len(key)), cBuffer, C.size_t(len(buffer)), &cSize, &cFound, &cErr)
	return int(cSize), cFound != 0, fromCError(cErr)
}

// GetIntoBufferCF is GetIntoBuffer for a column family, with the same size,
// found, buffer and snapshot semantics.
func (db *DB) GetIntoBufferCF(opts *ReadOptions, cf *ColumnFamilyHandle, key, buffer []byte) (size int, found bool, err error) {
	if !db.noGoCallbacks {
		return db.getIntoBufferCFSlow(opts, cf, key, buffer)
	}

	var cSize C.size_t
	var cFound C.uchar
	var cErr *C.char
	var empty [1]byte
	cBuffer := refGoBytes(buffer)
	if len(buffer) == 0 {
		cBuffer = refGoBytes(empty[:])
	}
	C.grocksdb_nocb_get_into_buffer_cf(db.c, opts.c, cf.c, refGoBytes(key), C.size_t(len(key)), cBuffer, C.size_t(len(buffer)), &cSize, &cFound, &cErr)

	return int(cSize), cFound != 0, fromCError(cErr)
}

// getIntoBufferCFSlow isolates escaping output arguments from the annotated path.
func (db *DB) getIntoBufferCFSlow(opts *ReadOptions, cf *ColumnFamilyHandle, key, buffer []byte) (size int, found bool, err error) {
	var cSize C.size_t
	var cFound C.uchar
	var cErr *C.char
	var empty [1]byte
	cBuffer := refGoBytes(buffer)
	if len(buffer) == 0 {
		cBuffer = refGoBytes(empty[:])
	}
	C.rocksdb_get_into_buffer_cf(db.c, opts.c, cf.c, refGoBytes(key), C.size_t(len(key)), cBuffer, C.size_t(len(buffer)), &cSize, &cFound, &cErr)
	return int(cSize), cFound != 0, fromCError(cErr)
}
