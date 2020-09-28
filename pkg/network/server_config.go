package network

import (
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"go.uber.org/zap/zapcore"
)

type (
	// ServerConfig holds the server configuration.
	ServerConfig struct {
		// MinPeers is the minimum number of peers for normal operation,
		// when the node has less than this number of peers it tries to
		// connect with some new ones.
		MinPeers int

		// AttemptConnPeers it the number of connection to try to
		// establish when the connection count drops below the MinPeers
		// value.
		AttemptConnPeers int

		// MaxPeers it the maximum numbers of peers that can
		// be connected to the server.
		MaxPeers int

		// The user agent of the server.
		UserAgent string

		// Address. Example: "127.0.0.1".
		Address string

		// Port. Example: 20332.
		Port uint16

		// The network mode the server will operate on.
		// ModePrivNet docker private network.
		// ModeTestNet NEO test network.
		// ModeMainNet NEO main network.
		Net netmode.Magic

		// Relay determines whether the server is forwarding its inventory.
		Relay bool

		// Seeds are a list of initial nodes used to establish connectivity.
		Seeds []string

		// Maximum duration a single dial may take.
		DialTimeout time.Duration

		// The duration between protocol ticks with each connected peer.
		// When this is 0, the default interval of 5 seconds will be used.
		ProtoTickInterval time.Duration

		// Interval used in pinging mechanism for syncing blocks.
		PingInterval time.Duration
		// Time to wait for pong(response for sent ping request).
		PingTimeout time.Duration

		// Level of the internal logger.
		LogLevel zapcore.Level

		// Wallet is a wallet configuration.
		Wallet *config.Wallet

		// TimePerBlock is an interval which should pass between two successive blocks.
		TimePerBlock time.Duration

		// OracleCfg is oracle module configuration.
		OracleCfg config.OracleConfiguration
	}
)

// NewServerConfig creates a new ServerConfig struct
// using the main applications config.
func NewServerConfig(cfg config.Config) ServerConfig {
	appConfig := cfg.ApplicationConfiguration
	protoConfig := cfg.ProtocolConfiguration

	var wc *config.Wallet
	if appConfig.UnlockWallet.Path != "" {
		wc = &appConfig.UnlockWallet
	}

	return ServerConfig{
		UserAgent:         cfg.GenerateUserAgent(),
		Address:           appConfig.Address,
		Port:              appConfig.NodePort,
		Net:               protoConfig.Magic,
		Relay:             appConfig.Relay,
		Seeds:             protoConfig.SeedList,
		DialTimeout:       appConfig.DialTimeout * time.Second,
		ProtoTickInterval: appConfig.ProtoTickInterval * time.Second,
		PingInterval:      appConfig.PingInterval * time.Second,
		PingTimeout:       appConfig.PingTimeout * time.Second,
		MaxPeers:          appConfig.MaxPeers,
		AttemptConnPeers:  appConfig.AttemptConnPeers,
		MinPeers:          appConfig.MinPeers,
		Wallet:            wc,
		TimePerBlock:      time.Duration(protoConfig.SecondsPerBlock) * time.Second,
		OracleCfg:         appConfig.Oracle,
	}
}
