package grocksdb

import "errors"

// ErrGoCallbacksNotAllowed is returned when a DB or column family opted in with
// Options.SetNoGoCallbacks uses Go-implemented callbacks.
var ErrGoCallbacksNotAllowed = errors.New("grocksdb: Go callbacks are not allowed with SetNoGoCallbacks(true)")

// SetNoGoCallbacks promises that no Go code runs on the calling thread while
// RocksDB serves a foreground call on this DB. That means no comparator, merge
// operator, prefix extractor or compaction filter implemented in Go, on the DB or
// any of its column families.
//
// In exchange, Get, GetCF, GetPinned, GetPinnedCF, Put, PutCF, Delete, DeleteCF,
// SingleDelete, SingleDeleteCF, Merge, MergeCF, Write, GetIntoBuffer and
// GetIntoBufferCF skip the heap allocations cgo otherwise makes for their
// output arguments.
//
// The mode is read once, when the DB is opened. Opening fails with
// ErrGoCallbacksNotAllowed if Go callbacks were installed through this package's
// setters, and so does CreateColumnFamily* with such options on the opened DB.
// grocksdb can't inspect native objects passed through SetNativeComparator,
// NewNativeMergeOperator and similar. If one of them calls into Go, the program
// panics.
//
// Default: false
func (opts *Options) SetNoGoCallbacks(value bool) {
	opts.noGoCallbacks = value
}

// NoGoCallbacks reports whether SetNoGoCallbacks(true) was called.
func (opts *Options) NoGoCallbacks() bool {
	return opts.noGoCallbacks
}

// checkNoGoCallbacks rejects Go callbacks when opts opted in with SetNoGoCallbacks.
func checkNoGoCallbacks(opts *Options, cfOpts []*Options) error {
	if !opts.noGoCallbacks {
		return nil
	}
	if opts.goCallbacks {
		return ErrGoCallbacksNotAllowed
	}
	for _, o := range cfOpts {
		if o.goCallbacks {
			return ErrGoCallbacksNotAllowed
		}
	}
	return nil
}
