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
