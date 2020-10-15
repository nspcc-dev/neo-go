package transaction

//go:generate stringer -type=AttrType -linecomment

// AttrType represents the purpose of the attribute.
type AttrType uint8

const (
	// ReservedLowerBound is the lower bound of reserved attribute types
	ReservedLowerBound = 0xe0
	// ReservedUpperBound is the upper bound of reserved attribute types
	ReservedUpperBound = 0xff
)

// List of valid attribute types.
const (
	HighPriority    AttrType = 1
	OracleResponseT AttrType = 0x11               // OracleResponse
	NotValidBeforeT AttrType = ReservedLowerBound // NotValidBefore
)

func (a AttrType) allowMultiple() bool {
	return false
}
