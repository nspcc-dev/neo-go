package vm

type (
	enumerator interface {
		Next() bool
		Value() StackItem
	}

	arrayWrapper struct {
		index int
		value []StackItem
	}

	concatEnum struct {
		current enumerator
		second  enumerator
	}
)

type (
	iterator interface {
		enumerator
		Key() StackItem
	}

	mapWrapper struct {
		index int
		keys  []interface{}
		m     map[interface{}]StackItem
	}

	concatIter struct {
		current iterator
		second  iterator
	}

	keysWrapper struct {
		iter iterator
	}

	valuesWrapper struct {
		iter iterator
	}
)

func (a *arrayWrapper) Next() bool {
	if next := a.index + 1; next < len(a.value) {
		a.index = next
		return true
	}

	return false
}

func (a *arrayWrapper) Value() StackItem {
	return a.value[a.index]
}

func (a *arrayWrapper) Key() StackItem {
	return makeStackItem(a.index)
}

func (c *concatEnum) Next() bool {
	if c.current.Next() {
		return true
	}
	c.current = c.second

	return c.current.Next()
}

func (c *concatEnum) Value() StackItem {
	return c.current.Value()
}

func (i *concatIter) Next() bool {
	if i.current.Next() {
		return true
	}
	i.current = i.second

	return i.second.Next()
}

func (i *concatIter) Value() StackItem {
	return i.current.Value()
}

func (i *concatIter) Key() StackItem {
	return i.current.Key()
}

func (m *mapWrapper) Next() bool {
	if next := m.index + 1; next < len(m.keys) {
		m.index = next
		return true
	}

	return false
}

func (m *mapWrapper) Value() StackItem {
	return m.m[m.keys[m.index]]
}

func (m *mapWrapper) Key() StackItem {
	return makeStackItem(m.keys[m.index])
}

func (e *keysWrapper) Next() bool {
	return e.iter.Next()
}

func (e *keysWrapper) Value() StackItem {
	return e.iter.Key()
}

func (e *valuesWrapper) Next() bool {
	return e.iter.Next()
}

func (e *valuesWrapper) Value() StackItem {
	return e.iter.Value()
}
