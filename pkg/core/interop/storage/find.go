package storage

import "github.com/nspcc-dev/neo-go/pkg/vm/stackitem"

// Storage iterator options.
const (
	FindDefault      = 0
	FindKeysOnly     = 1 << 0
	FindRemovePrefix = 1 << 1
	FindValuesOnly   = 1 << 2
	FindDeserialize  = 1 << 3

	FindAll = FindDefault | FindKeysOnly | FindRemovePrefix | FindValuesOnly |
		FindDeserialize
)

type Iterator struct {
	m     []stackitem.MapElement
	opts  int64
	index int
}

func NewIterator(m *stackitem.Map, opts int64) *Iterator {
	return &Iterator{
		m:     m.Value().([]stackitem.MapElement),
		opts:  opts,
		index: -1,
	}
}

func (s *Iterator) Next() bool {
	if s.index < len(s.m) {
		s.index += 1
	}
	return s.index < len(s.m)
}

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
	if s.opts&FindValuesOnly != 0 {
		return value
	}
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray(key),
		value,
	})
}
