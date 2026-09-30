#include "grocksdb.h"
#include "_cgo_export.h"

/* Base */

void gorocksdb_destruct_handler(void* state) { }

/* Comparator */

rocksdb_comparator_t* gorocksdb_comparator_create(uintptr_t idx) {
    return rocksdb_comparator_create(
        (void*)idx,
        gorocksdb_destruct_handler,
        (int (*)(void*, const char*, size_t, const char*, size_t))(gorocksdb_comparator_compare),
        (const char *(*)(void*))(gorocksdb_comparator_name));
}

rocksdb_comparator_t* gorocksdb_comparator_with_ts_create(uintptr_t idx, size_t ts_size) {
    return rocksdb_comparator_with_ts_create(
        (void*)idx,
        gorocksdb_destruct_handler,
        (int (*)(void*, const char*, size_t, const char*, size_t))(gorocksdb_comparator_compare),
        (int (*)(void*, const char*, size_t, const char*, size_t))(gorocksdb_comparator_compare_ts),
        (int (*)(void*, const char*, size_t, unsigned char, const char*, size_t, unsigned char))(gorocksdb_comparator_compare_without_ts),
        (const char* (*)(void*))(gorocksdb_comparator_name),
        ts_size);
}

/* CompactionFilter */

rocksdb_compactionfilter_t* gorocksdb_compactionfilter_create(uintptr_t idx) {
    return rocksdb_compactionfilter_create(
        (void*)idx,
        gorocksdb_destruct_handler,
        (unsigned char (*)(void*, int, const char*, size_t, const char*, size_t, char**, size_t*, unsigned char*))(gorocksdb_compactionfilter_filter),
        (const char *(*)(void*))(gorocksdb_compactionfilter_name));
}

/* Merge Operator */

rocksdb_mergeoperator_t* gorocksdb_mergeoperator_create(uintptr_t idx) {
    return rocksdb_mergeoperator_create(
        (void*)idx,
        gorocksdb_destruct_handler,
        (char* (*)(void*, const char*, size_t, const char*, size_t, const char* const*, const size_t*, int, unsigned char*, size_t*))(gorocksdb_mergeoperator_full_merge),
        (char* (*)(void*, const char*, size_t, const char* const*, const size_t*, int, unsigned char*, size_t*))(gorocksdb_mergeoperator_partial_merge_multi),
        gorocksdb_mergeoperator_delete_value,
        (const char* (*)(void*))(gorocksdb_mergeoperator_name));
}

void gorocksdb_mergeoperator_delete_value(void* id, const char* v, size_t s) {
    free((char*)v);
}

/* Slice Transform */

rocksdb_slicetransform_t* gorocksdb_slicetransform_create(uintptr_t idx) {
    return rocksdb_slicetransform_create(
    	(void*)idx,
    	gorocksdb_destruct_handler,
    	(char* (*)(void*, const char*, size_t, size_t*))(gorocksdb_slicetransform_transform),
    	(unsigned char (*)(void*, const char*, size_t))(gorocksdb_slicetransform_in_domain),
    	(const char* (*)(void*))(gorocksdb_slicetransform_name));
}

/* noescape/nocallback forwarders, see grocksdb.h */

char* grocksdb_nocb_get(rocksdb_t* db, const rocksdb_readoptions_t* options, const char* key, size_t keylen, size_t* vallen, char** errptr) {
  return rocksdb_get(db, options, key, keylen, vallen, errptr);
}

char* grocksdb_nocb_get_cf(rocksdb_t* db, const rocksdb_readoptions_t* options, rocksdb_column_family_handle_t* column_family, const char* key, size_t keylen, size_t* vallen, char** errptr) {
  return rocksdb_get_cf(db, options, column_family, key, keylen, vallen, errptr);
}

rocksdb_pinnableslice_t* grocksdb_nocb_get_pinned(rocksdb_t* db, const rocksdb_readoptions_t* options, const char* key, size_t keylen, char** errptr) {
  return rocksdb_get_pinned(db, options, key, keylen, errptr);
}

rocksdb_pinnableslice_t* grocksdb_nocb_get_pinned_cf(rocksdb_t* db, const rocksdb_readoptions_t* options, rocksdb_column_family_handle_t* column_family, const char* key, size_t keylen, char** errptr) {
  return rocksdb_get_pinned_cf(db, options, column_family, key, keylen, errptr);
}

void grocksdb_nocb_put(rocksdb_t* db, const rocksdb_writeoptions_t* options, const char* key, size_t keylen, const char* val, size_t vallen, char** errptr) {
  rocksdb_put(db, options, key, keylen, val, vallen, errptr);
}

void grocksdb_nocb_put_cf(rocksdb_t* db, const rocksdb_writeoptions_t* options, rocksdb_column_family_handle_t* column_family, const char* key, size_t keylen, const char* val, size_t vallen, char** errptr) {
  rocksdb_put_cf(db, options, column_family, key, keylen, val, vallen, errptr);
}

void grocksdb_nocb_delete(rocksdb_t* db, const rocksdb_writeoptions_t* options, const char* key, size_t keylen, char** errptr) {
  rocksdb_delete(db, options, key, keylen, errptr);
}

void grocksdb_nocb_delete_cf(rocksdb_t* db, const rocksdb_writeoptions_t* options, rocksdb_column_family_handle_t* column_family, const char* key, size_t keylen, char** errptr) {
  rocksdb_delete_cf(db, options, column_family, key, keylen, errptr);
}

void grocksdb_nocb_singledelete(rocksdb_t* db, const rocksdb_writeoptions_t* options, const char* key, size_t keylen, char** errptr) {
  rocksdb_singledelete(db, options, key, keylen, errptr);
}

void grocksdb_nocb_singledelete_cf(rocksdb_t* db, const rocksdb_writeoptions_t* options, rocksdb_column_family_handle_t* column_family, const char* key, size_t keylen, char** errptr) {
  rocksdb_singledelete_cf(db, options, column_family, key, keylen, errptr);
}

void grocksdb_nocb_merge(rocksdb_t* db, const rocksdb_writeoptions_t* options, const char* key, size_t keylen, const char* val, size_t vallen, char** errptr) {
  rocksdb_merge(db, options, key, keylen, val, vallen, errptr);
}

void grocksdb_nocb_merge_cf(rocksdb_t* db, const rocksdb_writeoptions_t* options, rocksdb_column_family_handle_t* column_family, const char* key, size_t keylen, const char* val, size_t vallen, char** errptr) {
  rocksdb_merge_cf(db, options, column_family, key, keylen, val, vallen, errptr);
}

void grocksdb_nocb_write(rocksdb_t* db, const rocksdb_writeoptions_t* options, rocksdb_writebatch_t* batch, char** errptr) {
  rocksdb_write(db, options, batch, errptr);
}
