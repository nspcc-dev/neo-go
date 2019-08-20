package protocol

//Version represents the latest protocol version for the neo node
type Version uint32

const (
	// DefaultVersion is the nodes default protocol version
	DefaultVersion Version = 0
	// UserAgent is the nodes user agent or human-readable name
	UserAgent = "/NEO-GO/"
)

// ServiceFlag indicates the services provided by the node. 1 = P2P Full Node
type ServiceFlag uint64

// List of Services offered by the node
const (
	NodePeerService ServiceFlag = 1
	// BloomFilerService ServiceFlag = 2 // Not implemented
	// PrunedNode        ServiceFlag = 3 // Not implemented
	// LightNode         ServiceFlag = 4 // Not implemented

)

// Magic is the network that NEO is running on
type Magic uint32

// List of possible networks
const (
	MainNet Magic = 7630401
	TestNet Magic = 0x74746e41
)

// String implements the stringer interface
func (m Magic) String() string {
	switch m {
	case MainNet:
		return "Mainnet"
	case TestNet:
		return "Testnet"
	default:
		return "UnknownNet"
	}
}
