package protocol

type Version uint32

const (
	DefaultVersion Version = 0
	UserAgent              = "/NEO-GO/" // TODO: This may be relocated to a config file
)

// ServiceFlag indicates the services provided by the node. 1 = P2P Full Node
type ServiceFlag uint64

const (
	NodePeerService ServiceFlag = 1
	// BloomFilerService ServiceFlag = 2 // Not implemented
	// PrunedNode        ServiceFlag = 3 // Not implemented
	// LightNode         ServiceFlag = 4 // Not implemented

)

// Magic is the network that NEO is running on
type Magic uint32

const (
	MainNet Magic = 7630401
	TestNet Magic = 0x74746e41
)
