package storage

import (
	"context"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Storage iterator options.
const (
	FindDefault      = 0
	FindKeysOnly     = 1 << 0
	FindRemovePrefix = 1 << 1
	FindValuesOnly   = 1 << 2
	FindDeserialize  = 1 << 3
	FindPick0        = 1 << 4
	FindPick1        = 1 << 5

	FindAll = FindDefault | FindKeysOnly | FindRemovePrefix | FindValuesOnly |
		FindDeserialize | FindPick0 | FindPick1
)

// Iterator is an iterator state representation.
type Iterator struct {
	seekCh chan storage.KeyValue
	cancel context.CancelFunc
	curr   storage.KeyValue
	next   bool
	opts   int64
	prefix []byte
}

// NewIterator creates a new Iterator with given options for a given channel of store.Seek results.
func NewIterator(seekCh chan storage.KeyValue, cancel context.CancelFunc, prefix []byte, opts int64) *Iterator {
	return &Iterator{
		seekCh: seekCh,
		cancel: cancel,
		opts:   opts,
		prefix: slice.Copy(prefix),
	}
}

// Next advances the iterator and returns true if Value can be called at the
// current position.
func (s *Iterator) Next() bool {
	s.curr, s.next = <-s.seekCh
	return s.next
}

// Value returns current iterators value (exact type depends on options this
// iterator was created with).
func (s *Iterator) Value() stackitem.Item {
	if !s.next {
		panic("iterator index out of range")
	}
	key := s.curr.Key
	if s.opts&FindRemovePrefix == 0 {
		key = append(s.prefix, key...)
	}
	if s.opts&FindKeysOnly != 0 {
		return stackitem.NewByteArray(key)
	}
	value := stackitem.Item(stackitem.NewByteArray(s.curr.Value))
	if s.opts&FindDeserialize != 0 {
		bs := s.curr.Value
		var err error
		value, err = stackitem.Deserialize(bs)
		if err != nil {
			panic(err)
		}
	}
	if s.opts&FindPick0 != 0 {
		value = value.Value().([]stackitem.Item)[0]
	} else if s.opts&FindPick1 != 0 {
		value = value.Value().([]stackitem.Item)[1]
	}
	if s.opts&FindValuesOnly != 0 {
		return value
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray(key),
		value,
	})
}

// Close releases resources occupied by the Iterator.
// TODO: call this method on program unloading.
func (s *Iterator) Close() {
	s.cancel()
}
