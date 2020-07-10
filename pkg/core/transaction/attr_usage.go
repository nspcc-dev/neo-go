package transaction

//go:generate stringer -type=AttrUsage

// AttrUsage represents the purpose of the attribute.
type AttrUsage uint8

// List of valid attribute usages.
const (
	DescriptionURL AttrUsage = 0x81
)
