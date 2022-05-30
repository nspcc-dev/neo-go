package stackitem

type ro struct {
	isReadOnly bool
}

// IsReadOnly implements Immutable interface.
func (r *ro) IsReadOnly() bool {
	return r.isReadOnly
}

// MarkAsReadOnly implements immutable interface.
func (r *ro) MarkAsReadOnly() {
	r.isReadOnly = true
}

// Immutable is an interface supported by compound types (Array, Map, Struct).
type Immutable interface {
	IsReadOnly() bool
	MarkAsReadOnly()
}
