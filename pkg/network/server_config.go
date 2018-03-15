package network

import (
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	maxPeers      = 50
	minPeers      = 5
	maxBlockBatch = 200
	minPoolCount  = 30
)

var (
	protoTickInterval = 10 * time.Second
	dialTimeout       = 3 * time.Second
)

type (
	// ServerConfig holds the server configuration.
	ServerConfig struct {
		// MaxPeers it the maximum numbers of peers that can
		// be connected to the server.
		MaxPeers int

		// The user agent of the server.
		UserAgent string

		// The listen address of the TCP server.
		ListenTCP uint16

		// The network mode the server will operate on.
		// ModePrivNet docker private network.
		// ModeTestNet NEO test network.
		// ModeMainNet NEO main network.
		Net NetMode

		// Relay determins whether the server is forwarding its inventory.
		Relay bool

		// Seeds are a list of initial nodes used to establish connectivity.
		Seeds []string

		// Maximum duration a single dial may take.
		DialTimeout time.Duration

		// The duration between protocol ticks with each connected peer.
		// When this is 0, the default interval of 5 seconds will be used.
		ProtoTickInterval time.Duration

		// Level of the internal logger.
		LogLevel log.Level
	}
)

// NewServerConfig creates a new ServerConfig struct
// using the main applications config.
func NewServerConfig(config Config) ServerConfig {
	return ServerConfig{
		UserAgent:         config.GenerateUserAgent(),
		ListenTCP:         uint16(config.ApplicationConfiguration.NodePort),
		Net:               config.ProtocolConfiguration.Magic,
		Relay:             config.ApplicationConfiguration.Relay,
		Seeds:             config.ProtocolConfiguration.SeedList,
		DialTimeout:       dialTimeout,
		ProtoTickInterval: protoTickInterval,
		MaxPeers:          maxPeers,
	}
}
