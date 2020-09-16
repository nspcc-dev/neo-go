package transaction

//go:generate stringer -type=AttrType -linecomment

// AttrType represents the purpose of the attribute.
type AttrType uint8

// List of valid attribute types.
const (
	HighPriority    AttrType = 1
	OracleResponseT AttrType = 0x11 // OracleResponse
)

func (a AttrType) allowMultiple() bool {
	return false
}
