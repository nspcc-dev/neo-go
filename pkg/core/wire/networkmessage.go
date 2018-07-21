package wire

// Constant for the MessageSizes
const (
	MagicMaxSize    uint32 = 4
	CommandMaxSize  uint32 = 12
	LengthMaxSize   uint32 = 4
	ChecksumMaxSize uint32 = 4
)

// If the networkMessage is less than this, then one of those constants is missing.
func minNetworkMessageSize(pver uint32) uint32 {
	if pver == 0 {
		return MagicMaxSize + CommandMaxSize + LengthMaxSize + ChecksumMaxSize
	}
	return 0
}
