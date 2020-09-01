package transaction

//go:generate stringer -type=AttrType

// AttrType represents the purpose of the attribute.
type AttrType uint8

// List of valid attribute types.
const (
	HighPriority AttrType = 1
)
