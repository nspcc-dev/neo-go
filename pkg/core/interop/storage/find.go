package storage

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
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
	FindBackwards    = 1 << 7

	FindAll = FindDefault | FindKeysOnly | FindRemovePrefix | FindValuesOnly |
		FindDeserialize | FindPick0 | FindPick1 | FindBackwards
)

// Iterator is an iterator state representation.
type Iterator struct {
	seekCh chan storage.KeyValue
	curr   storage.KeyValue
	next   bool
	opts   int64
	// prefix is the storage item key prefix Find is performed with. It must be
	// copied if no FindRemovePrefix option specified since it's shared between all
	// iterator items.
	prefix []byte
}

// NewIterator creates a new Iterator with the given options for the given channel of store.Seek results.
func NewIterator(seekCh chan storage.KeyValue, prefix []byte, opts int64) *Iterator {
	return &Iterator{
		seekCh: seekCh,
		opts:   opts,
		prefix: bytes.Clone(prefix),
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
		key = slices.Concat(s.prefix, key)
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

// Find finds stored key-value pair.
func Find(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*Context)
	if !ok {
		return fmt.Errorf("%T is not a storage,Context", stcInterface)
	}
	prefix := ic.VM.Estack().Pop().Bytes()
	opts := ic.VM.Estack().Pop().BigInt().Int64()
	if opts&^FindAll != 0 {
		return fmt.Errorf("%w: unknown flag", errFindInvalidOptions)
	}
	if opts&FindKeysOnly != 0 &&
		opts&(FindDeserialize|FindPick0|FindPick1) != 0 {
		return fmt.Errorf("%w KeysOnly conflicts with other options", errFindInvalidOptions)
	}
	if opts&FindValuesOnly != 0 &&
		opts&(FindKeysOnly|FindRemovePrefix) != 0 {
		return fmt.Errorf("%w: KeysOnly conflicts with ValuesOnly", errFindInvalidOptions)
	}
	if opts&FindPick0 != 0 && opts&FindPick1 != 0 {
		return fmt.Errorf("%w: Pick0 conflicts with Pick1", errFindInvalidOptions)
	}
	if opts&FindDeserialize == 0 && (opts&FindPick0 != 0 || opts&FindPick1 != 0) {
		return fmt.Errorf("%w: PickN is specified without Deserialize", errFindInvalidOptions)
	}
	bkwrds := opts&FindBackwards != 0
	ctx, cancel := context.WithCancel(context.Background())
	seekres := ic.DAO.SeekAsync(ctx, stc.ID, storage.SeekRange{Prefix: prefix, Backwards: bkwrds})
	item := NewIterator(seekres, prefix, opts)
	ic.VM.Estack().PushItem(stackitem.NewInterop(item))
	ic.RegisterCancelFunc(func() {
		cancel()
		// Underlying persistent store is likely to be a private MemCachedStore. Thus,
		// to avoid concurrent map iteration and map write we need to wait until internal
		// seek goroutine is finished, because it can access underlying persistent store.
		for range seekres { //nolint:revive //empty-block
		}
	})

	return nil
}
