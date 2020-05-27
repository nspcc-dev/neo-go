package core

import (
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

type storageWrapper struct {
	next       dao.StorageIteratorFunc
	key, value vm.StackItem
	finished   bool
}

// newStorageIterator returns new storage iterator from the `next()` callback.
func newStorageIterator(next dao.StorageIteratorFunc) *storageWrapper {
	return &storageWrapper{
		next: next,
	}
}

// Next implements iterator interface.
func (s *storageWrapper) Next() bool {
	if s.finished {
		return false
	}
	key, value, err := s.next()
	if err != nil {
		s.finished = true
		return false
	}
	s.key = vm.NewByteArrayItem(key)
	s.value = vm.NewByteArrayItem(value)
	return true
}

// Value implements iterator interface.
func (s *storageWrapper) Value() vm.StackItem {
	return s.value
}

// Key implements iterator interface.
func (s *storageWrapper) Key() vm.StackItem {
	return s.key
}
