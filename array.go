package grocksdb

// #include "stdlib.h"
// #include "rocksdb/c.h"
import "C"

import (
	"reflect"
	"unsafe"
)

type (
	charsSlice         []*C.char
	sizeTSlice         []C.size_t
	columnFamilySlice  []*C.rocksdb_column_family_handle_t
	pinnableSliceSlice []*C.rocksdb_pinnableslice_t
	optimizeSliceSlice []C.rocksdb_slice_t
)

func (s charsSlice) c() **C.char {
	sH := (*reflect.SliceHeader)(unsafe.Pointer(&s))
	return (**C.char)(unsafe.Pointer(sH.Data))
}

func (s sizeTSlice) c() *C.size_t {
	sH := (*reflect.SliceHeader)(unsafe.Pointer(&s))
	return (*C.size_t)(unsafe.Pointer(sH.Data))
}

func (s columnFamilySlice) c() **C.rocksdb_column_family_handle_t {
	sH := (*reflect.SliceHeader)(unsafe.Pointer(&s))
	return (**C.rocksdb_column_family_handle_t)(unsafe.Pointer(sH.Data))
}

func (s pinnableSliceSlice) c() **C.rocksdb_pinnableslice_t {
	sH := (*reflect.SliceHeader)(unsafe.Pointer(&s))
	return (**C.rocksdb_pinnableslice_t)(unsafe.Pointer(sH.Data))
}

func (s optimizeSliceSlice) c() *C.rocksdb_slice_t {
	sH := (*reflect.SliceHeader)(unsafe.Pointer(&s))
	return (*C.rocksdb_slice_t)(unsafe.Pointer(sH.Data))
}

func (s pinnableSliceSlice) destroy() {
	for _, v := range s {
		if v != nil {
			C.rocksdb_pinnableslice_destroy(v)
		}
	}
}

// byteSlicesToCSlices copies a slice of byte slices into a single C allocation
// and returns pointers into it, plus their sizes. One malloc replaces one per
// key, which matters for MultiGet with many keys.
// IMPORTANT: the copies must be freed with the Destroy method of cKeyArray.
func byteSlicesToCSlices(vals [][]byte) (cKeyArray, sizeTSlice) {
	if len(vals) == 0 {
		return cKeyArray{}, nil
	}

	// Reserve one byte so every pointer is valid C storage even if all keys are empty.
	total := 1
	for _, val := range vals {
		total += len(val)
	}
	base := C.malloc(C.size_t(total))

	chars := make(charsSlice, len(vals))
	sizes := make(sizeTSlice, len(vals))
	offset := 0
	for i, val := range vals {
		p := unsafe.Add(base, offset)
		copy(unsafe.Slice((*byte)(p), len(val)), val)
		chars[i] = (*C.char)(p)
		sizes[i] = C.size_t(len(val))
		offset += len(val)
	}

	return cKeyArray{chars: chars, base: base}, sizes
}

// cKeyArray holds pointers into one C allocation made by byteSlicesToCSlices.
type cKeyArray struct {
	chars charsSlice
	base  unsafe.Pointer
}

func (a cKeyArray) c() **C.char { return a.chars.c() }

func (a cKeyArray) Destroy() {
	if a.base != nil {
		C.free(a.base)
	}
}

func (s charsSlice) Destroy() {
	for _, chars := range s {
		if chars != nil {
			C.free(unsafe.Pointer(chars))
		}
	}
}
