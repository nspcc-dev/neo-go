package transaction

//go:generate stringer -type=AttrType -linecomment

// AttrType represents the purpose of the attribute.
type AttrType uint8

// List of valid attribute types.
const (
	HighPriority    AttrType = 1
	OracleResponseT AttrType = 0x11 // OracleResponse
	NotValidBeforeT AttrType = 0xe0 // NotValidBefore
)

func (a AttrType) allowMultiple() bool {
	return false
}
