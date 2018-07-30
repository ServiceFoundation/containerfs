package gorocksdb

// #include "rocksdb/c.h"
import "C"

// A MergeOperator specifies the SEMANTICS of a merge, which only
// client knows. It could be numeric addition, list append, string
// concatenation, edit data structure, ... , anything.
// The library, on the other hand, is concerned with the exercise of this
// interface, at the right time (during get, iteration, compaction...)
//
// Please read the RocksDB documentation <http://rocksdb.org/> for
// more details and example implementations.
type MergeOperator interface {
	// Gives the client a way to express the read -> modify -> write semantics
	// key:           The key that's associated with this merge operation.
	//                Client could multiplex the merge operator based on it
	//                if the key space is partitioned and different subspaces
	//                refer to different types of data which have different
	//                merge operation semantics.
	// existingValue: null indicates that the key does not exist before this op.
	// operands:      the sequence of merge operations to apply, front() first.
	//
	// Return true on success.
	//
	// All values passed in will be client-specific values. So if this method
	// returns false, it is because client specified bad data or there was
	// internal corruption. This will be treated as an error by the library.
	FullMerge(key, existingValue []byte, operands [][]byte) ([]byte, bool)

	// This function performs merge(left_op, right_op)
	// when both the operands are themselves merge operation types
	// that you would have passed to a db.Merge() call in the same order
	// (i.e.: db.Merge(key,left_op), followed by db.Merge(key,right_op)).
	//
	// PartialMerge should combine them into a single merge operation.
	// The return value should be constructed such that a call to
	// db.Merge(key, new_value) would yield the same result as a call
	// to db.Merge(key, left_op) followed by db.Merge(key, right_op).
	//
	// If it is impossible or infeasible to combine the two operations, return false.
	// The library will internally keep track of the operations, and apply them in the
	// correct order once a base-value (a Put/Delete/End-of-Database) is seen.
	PartialMerge(key, leftOperand, rightOperand []byte) ([]byte, bool)

	// The name of the MergeOperator.
	Name() string
}

// NewNativeMergeOperator creates a MergeOperator object.
func NewNativeMergeOperator(c *C.rocksdb_mergeoperator_t) MergeOperator {
	return nativeMergeOperator{c}
}

type nativeMergeOperator struct {
	c *C.rocksdb_mergeoperator_t
}

func (mo nativeMergeOperator) FullMerge(key, existingValue []byte, operands [][]byte) ([]byte, bool) {
	return nil, false
}
func (mo nativeMergeOperator) PartialMerge(key, leftOperand, rightOperand []byte) ([]byte, bool) {
	return nil, false
}
func (mo nativeMergeOperator) Name() string { return "" }

// Hold references to merge operators.
var mergeOperators = NewCOWList()

func registerMergeOperator(merger MergeOperator) int {
	return mergeOperators.Append(merger)
}

//export gorocksdb_mergeoperator_full_merge
func gorocksdb_mergeoperator_full_merge(idx int, cKey *C.char, cKeyLen C.size_t, cExistingValue *C.char, cExistingValueLen C.size_t, cOperands **C.char, cOperandsLen *C.size_t, cNumOperands C.int, cSuccess *C.uchar, cNewValueLen *C.size_t) *C.char {
	key := charToByte(cKey, cKeyLen)
	rawOperands := charSlice(cOperands, cNumOperands)
	operandsLen := sizeSlice(cOperandsLen, cNumOperands)
	existingValue := charToByte(cExistingValue, cExistingValueLen)
	operands := make([][]byte, int(cNumOperands))
	for i, len := range operandsLen {
		operands[i] = charToByte(rawOperands[i], len)
	}

	newValue, success := mergeOperators.Get(idx).(MergeOperator).FullMerge(key, existingValue, operands)
	newValueLen := len(newValue)

	*cNewValueLen = C.size_t(newValueLen)
	*cSuccess = boolToChar(success)

	return cByteSlice(newValue)
}

//export gorocksdb_mergeoperator_partial_merge_multi
func gorocksdb_mergeoperator_partial_merge_multi(idx int, cKey *C.char, cKeyLen C.size_t, cOperands **C.char, cOperandsLen *C.size_t, cNumOperands C.int, cSuccess *C.uchar, cNewValueLen *C.size_t) *C.char {
	key := charToByte(cKey, cKeyLen)
	rawOperands := charSlice(cOperands, cNumOperands)
	operandsLen := sizeSlice(cOperandsLen, cNumOperands)
	operands := make([][]byte, int(cNumOperands))
	for i, len := range operandsLen {
		operands[i] = charToByte(rawOperands[i], len)
	}

	var newValue []byte
	success := true

	merger := mergeOperators.Get(idx).(MergeOperator)
	leftOperand := operands[0]
	for i := 1; i < int(cNumOperands); i++ {
		newValue, success = merger.PartialMerge(key, leftOperand, operands[i])
		if !success {
			break
		}
		leftOperand = newValue
	}

	newValueLen := len(newValue)
	*cNewValueLen = C.size_t(newValueLen)
	*cSuccess = boolToChar(success)

	return cByteSlice(newValue)
}

//export gorocksdb_mergeoperator_name
func gorocksdb_mergeoperator_name(idx int) *C.char {
	return stringToChar(mergeOperators.Get(idx).(MergeOperator).Name())
}