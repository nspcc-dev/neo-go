package transaction

//go:generate stringer -type=AttrType -linecomment

// AttrType represents the purpose of the attribute.
type AttrType uint8

const (
	// ReservedLowerBound is the lower bound of reserved attribute types.
	ReservedLowerBound = 0xe0
	// ReservedUpperBound is the upper bound of reserved attribute types.
	ReservedUpperBound = 0xff
)

// List of valid attribute types.
const (
	HighPriority         AttrType = 1
	OracleResponseT      AttrType = 0x11 // OracleResponse
	NotValidBeforeT      AttrType = 0x20 // NotValidBefore
	ConflictsT           AttrType = 0x21 // Conflicts
	NotaryAssistedT      AttrType = 0x22 // NotaryAssisted
	RefundableSystemFeeT AttrType = 0x30 // RefundableSystemFee
)

func (a AttrType) allowMultiple() bool {
	switch a {
	case ConflictsT:
		return true
	default:
		return false
	}
}
