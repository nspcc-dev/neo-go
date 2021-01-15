package storage

import "github.com/nspcc-dev/neo-go/pkg/vm/stackitem"

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
	m     []stackitem.MapElement
	opts  int64
	index int
}

// NewIterator creates a new Iterator with given options for a given map.
func NewIterator(m *stackitem.Map, opts int64) *Iterator {
	return &Iterator{
		m:     m.Value().([]stackitem.MapElement),
		opts:  opts,
		index: -1,
	}
}

// Next advances the iterator and returns true if Value can be called at the
// current position.
func (s *Iterator) Next() bool {
	if s.index < len(s.m) {
		s.index++
	}
	return s.index < len(s.m)
}

// Value returns current iterators value (exact type depends on options this
// iterator was created with).
func (s *Iterator) Value() stackitem.Item {
	key := s.m[s.index].Key.Value().([]byte)
	if s.opts&FindRemovePrefix != 0 {
		key = key[1:]
	}
	if s.opts&FindKeysOnly != 0 {
		return stackitem.NewByteArray(key)
	}
	value := s.m[s.index].Value
	if s.opts&FindDeserialize != 0 {
		bs := s.m[s.index].Value.Value().([]byte)
		var err error
		value, err = stackitem.DeserializeItem(bs)
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
