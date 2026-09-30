package grocksdb

import (
	"encoding/binary"
	"math"
)

// WriteBatchBuilder encodes ordinary RocksDB WriteBatch records in Go, avoiding
// a cgo call for each record. Its zero value is ready to use. It copies input
// bytes and is not safe for concurrent use.
//
// Reuse one builder across batches, calling Clear between them: the gain comes
// from amortizing its buffer. A reused builder beat WriteBatch from about 16
// records per batch. A fresh builder per batch can be slower than WriteBatch,
// and single-record batches are always better served by WriteBatch.
//
// Column family methods take IDs from ColumnFamilyHandle.ID. Cache those IDs
// before building a batch; ID 0 is the default column family. IDs must belong to
// the DB receiving the batch. User-defined timestamps, per-key protection bytes,
// and transaction markers are not supported; use WriteBatch for those features.
// The encoding matches RocksDB 11.1.2's db/write_batch.cc.
type WriteBatchBuilder struct {
	data  []byte
	count uint32
}

// NewWriteBatchBuilder creates an empty Go-side batch builder.
func NewWriteBatchBuilder() *WriteBatchBuilder {
	return &WriteBatchBuilder{data: make([]byte, 12)}
}

func (b *WriteBatchBuilder) init() {
	if len(b.data) == 0 {
		b.data = append(b.data, make([]byte, 12)...)
	}
}

func (b *WriteBatchBuilder) appendRecord(tag, cfTag WriteBatchRecordType, cfID uint32, key, value []byte, hasValue bool) {
	if uint64(len(key)) > math.MaxUint32 || uint64(len(value)) > math.MaxUint32 {
		panic("grocksdb: write batch record exceeds uint32 length")
	}
	if b.count == math.MaxUint32 {
		panic("grocksdb: write batch update count exceeds uint32")
	}
	b.init()
	if cfID == 0 {
		b.data = append(b.data, byte(tag))
	} else {
		b.data = append(b.data, byte(cfTag))
		b.data = binary.AppendUvarint(b.data, uint64(cfID))
	}
	b.data = binary.AppendUvarint(b.data, uint64(len(key)))
	b.data = append(b.data, key...)
	if hasValue {
		b.data = binary.AppendUvarint(b.data, uint64(len(value)))
		b.data = append(b.data, value...)
	}
	b.count++
	binary.LittleEndian.PutUint32(b.data[8:12], b.count)
}

// Put queues a key-value pair in the default column family.
func (b *WriteBatchBuilder) Put(key, value []byte) { b.PutCF(0, key, value) }

// PutCF queues a key-value pair in the column family with cfID.
func (b *WriteBatchBuilder) PutCF(cfID uint32, key, value []byte) {
	b.appendRecord(WriteBatchValueRecord, WriteBatchCFValueRecord, cfID, key, value, true)
}

// Merge queues a merge in the default column family.
func (b *WriteBatchBuilder) Merge(key, value []byte) { b.MergeCF(0, key, value) }

// MergeCF queues a merge in the column family with cfID.
func (b *WriteBatchBuilder) MergeCF(cfID uint32, key, value []byte) {
	b.appendRecord(WriteBatchMergeRecord, WriteBatchCFMergeRecord, cfID, key, value, true)
}

// Delete queues a deletion in the default column family.
func (b *WriteBatchBuilder) Delete(key []byte) { b.DeleteCF(0, key) }

// DeleteCF queues a deletion in the column family with cfID.
func (b *WriteBatchBuilder) DeleteCF(cfID uint32, key []byte) {
	b.appendRecord(WriteBatchDeletionRecord, WriteBatchCFDeletionRecord, cfID, key, nil, false)
}

// SingleDelete queues a single deletion. The same restrictions as
// WriteBatch.SingleDelete apply.
func (b *WriteBatchBuilder) SingleDelete(key []byte) { b.SingleDeleteCF(0, key) }

// SingleDeleteCF queues a single deletion in the column family with cfID.
// The same restrictions as WriteBatch.SingleDelete apply.
func (b *WriteBatchBuilder) SingleDeleteCF(cfID uint32, key []byte) {
	b.appendRecord(WriteBatchSingleDeletionRecord, WriteBatchCFSingleDeletionRecord, cfID, key, nil, false)
}

// DeleteRange queues deletion of keys in [startKey, endKey).
func (b *WriteBatchBuilder) DeleteRange(startKey, endKey []byte) {
	b.DeleteRangeCF(0, startKey, endKey)
}

// DeleteRangeCF queues deletion of keys in [startKey, endKey) in cfID.
func (b *WriteBatchBuilder) DeleteRangeCF(cfID uint32, startKey, endKey []byte) {
	b.appendRecord(WriteBatchRangeDeletion, WriteBatchCFRangeDeletion, cfID, startKey, endKey, true)
}

// PutLogData appends a blob without increasing the batch's update count.
func (b *WriteBatchBuilder) PutLogData(blob []byte) {
	if uint64(len(blob)) > math.MaxUint32 {
		panic("grocksdb: write batch record exceeds uint32 length")
	}
	b.init()
	b.data = append(b.data, byte(WriteBatchLogDataRecord))
	b.data = binary.AppendUvarint(b.data, uint64(len(blob)))
	b.data = append(b.data, blob...)
}

// Count returns the number of updates, excluding log data.
func (b *WriteBatchBuilder) Count() int { return int(b.count) }

// Clear removes all records and retains the buffer for reuse. Native batches
// previously returned by Build remain independent and unchanged.
func (b *WriteBatchBuilder) Clear() {
	b.init()
	b.data = b.data[:12]
	clear(b.data)
	b.count = 0
}

// Build copies the encoded records into a native WriteBatch in one cgo call.
// The caller must Destroy the returned batch. Build leaves the builder intact.
func (b *WriteBatchBuilder) Build() *WriteBatch {
	b.init()
	return WriteBatchFrom(b.data)
}
