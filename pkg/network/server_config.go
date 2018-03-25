package network

import (
	"time"

	"github.com/CityOfZion/neo-go/config"
	log "github.com/sirupsen/logrus"
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
		Net config.NetMode

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
func NewServerConfig(cfg config.Config) ServerConfig {
	appConfig := cfg.ApplicationConfiguration
	protoConfig := cfg.ProtocolConfiguration

	return ServerConfig{
		UserAgent:         cfg.GenerateUserAgent(),
		ListenTCP:         appConfig.NodePort,
		Net:               protoConfig.Magic,
		Relay:             appConfig.Relay,
		Seeds:             protoConfig.SeedList,
		DialTimeout:       (appConfig.DialTimeout * time.Second),
		ProtoTickInterval: (appConfig.ProtoTickInterval * time.Second),
		MaxPeers:          appConfig.MaxPeers,
	}
}
