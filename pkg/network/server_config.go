package network

import (
	"fmt"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"go.uber.org/zap/zapcore"
)

type (
	// ServerConfig holds the server configuration.
	ServerConfig struct {
		// MinPeers is the minimum number of peers for normal operation.
		// When a node has less than this number of peers, it tries to
		// connect with some new ones.
		MinPeers int

		// AttemptConnPeers is the number of connection to try to
		// establish when the connection count drops below the MinPeers
		// value.
		AttemptConnPeers int

		// MaxPeers is the maximum number of peers that can
		// be connected to the server.
		MaxPeers int

		// The user agent of the server.
		UserAgent string

		// Addresses stores the list of bind addresses for the node.
		Addresses []config.AnnounceableAddress

		// The network mode the server will operate on.
		// ModePrivNet docker private network.
		// ModeTestNet Neo test network.
		// ModeMainNet Neo main network.
		Net netmode.Magic

		// Relay determines whether the server is forwarding its inventory.
		Relay bool

		// Seeds is a list of initial nodes used to establish connectivity.
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

		// TimePerBlock is an interval which should pass between two successive blocks.
		TimePerBlock time.Duration

		// OracleCfg is oracle module configuration.
		OracleCfg config.OracleConfiguration

		// P2PNotaryCfg is notary module configuration.
		P2PNotaryCfg config.P2PNotary

		// StateRootCfg is stateroot module configuration.
		StateRootCfg config.StateRoot

		// ExtensiblePoolSize is the size of the pool for extensible payloads from a single sender.
		ExtensiblePoolSize int

		// BroadcastFactor is the factor (0-100) for fan-out optimization.
		BroadcastFactor int
	}
)

// NewServerConfig creates a new ServerConfig struct
// using the main applications config.
func NewServerConfig(cfg config.Config) (ServerConfig, error) {
	appConfig := cfg.ApplicationConfiguration
	protoConfig := cfg.ProtocolConfiguration
	timePerBlock := protoConfig.TimePerBlock
	if timePerBlock == 0 && protoConfig.SecondsPerBlock > 0 { //nolint:staticcheck // SA1019: protoConfig.SecondsPerBlock is deprecated
		timePerBlock = time.Duration(protoConfig.SecondsPerBlock) * time.Second //nolint:staticcheck // SA1019: protoConfig.SecondsPerBlock is deprecated
	}
	dialTimeout := appConfig.P2P.DialTimeout
	if dialTimeout == 0 && appConfig.DialTimeout > 0 { //nolint:staticcheck // SA1019: appConfig.DialTimeout is deprecated
		dialTimeout = time.Duration(appConfig.DialTimeout) * time.Second //nolint:staticcheck // SA1019: appConfig.DialTimeout is deprecated
	}
	protoTickInterval := appConfig.P2P.ProtoTickInterval
	if protoTickInterval == 0 && appConfig.ProtoTickInterval > 0 { //nolint:staticcheck // SA1019: appConfig.ProtoTickInterval is deprecated
		protoTickInterval = time.Duration(appConfig.ProtoTickInterval) * time.Second //nolint:staticcheck // SA1019: appConfig.ProtoTickInterval is deprecated
	}
	pingInterval := appConfig.P2P.PingInterval
	if pingInterval == 0 && appConfig.PingInterval > 0 { //nolint:staticcheck // SA1019: appConfig.PingInterval is deprecated
		pingInterval = time.Duration(appConfig.PingInterval) * time.Second //nolint:staticcheck // SA1019: appConfig.PingInterval is deprecated
	}
	pingTimeout := appConfig.P2P.PingTimeout
	if pingTimeout == 0 && appConfig.PingTimeout > 0 { //nolint:staticcheck // SA1019: appConfig.PingTimeout is deprecated
		pingTimeout = time.Duration(appConfig.PingTimeout) * time.Second //nolint:staticcheck // SA1019: appConfig.PingTimeout is deprecated
	}
	maxPeers := appConfig.P2P.MaxPeers
	if maxPeers == 0 && appConfig.MaxPeers > 0 { //nolint:staticcheck // SA1019: appConfig.MaxPeers is deprecated
		maxPeers = appConfig.MaxPeers //nolint:staticcheck // SA1019: appConfig.MaxPeers is deprecated
	}
	attemptConnPeers := appConfig.P2P.AttemptConnPeers
	if attemptConnPeers == 0 && appConfig.AttemptConnPeers > 0 { //nolint:staticcheck // SA1019: appConfig.AttemptConnPeers is deprecated
		attemptConnPeers = appConfig.AttemptConnPeers //nolint:staticcheck // SA1019: appConfig.AttemptConnPeers is deprecated
	}
	minPeers := appConfig.P2P.MinPeers
	if minPeers == 0 && appConfig.MinPeers > 0 { //nolint:staticcheck // SA1019: appConfig.MinPeers is deprecated
		minPeers = appConfig.MinPeers //nolint:staticcheck // SA1019: appConfig.MinPeers is deprecated
	}
	extPoolSize := appConfig.P2P.ExtensiblePoolSize
	if extPoolSize == 0 && appConfig.ExtensiblePoolSize > 0 { //nolint:staticcheck // SA1019: appConfig.ExtensiblePoolSize is deprecated
		extPoolSize = appConfig.ExtensiblePoolSize //nolint:staticcheck // SA1019: appConfig.ExtensiblePoolSize is deprecated
	}
	broadcastFactor := appConfig.P2P.BroadcastFactor
	if broadcastFactor > 0 && appConfig.BroadcastFactor > 0 { //nolint:staticcheck // SA1019: appConfig.BroadcastFactor is deprecated
		broadcastFactor = appConfig.BroadcastFactor //nolint:staticcheck // SA1019: appConfig.BroadcastFactor is deprecated
	}
	addrs, err := appConfig.GetAddresses()
	if err != nil {
		return ServerConfig{}, fmt.Errorf("failed to parse addresses: %w", err)
	}
	c := ServerConfig{
		UserAgent:          cfg.GenerateUserAgent(),
		Addresses:          addrs,
		Net:                protoConfig.Magic,
		Relay:              appConfig.Relay,
		Seeds:              protoConfig.SeedList,
		DialTimeout:        dialTimeout,
		ProtoTickInterval:  protoTickInterval,
		PingInterval:       pingInterval,
		PingTimeout:        pingTimeout,
		MaxPeers:           maxPeers,
		AttemptConnPeers:   attemptConnPeers,
		MinPeers:           minPeers,
		TimePerBlock:       timePerBlock,
		OracleCfg:          appConfig.Oracle,
		P2PNotaryCfg:       appConfig.P2PNotary,
		StateRootCfg:       appConfig.StateRoot,
		ExtensiblePoolSize: extPoolSize,
		BroadcastFactor:    broadcastFactor,
	}
	return c, nil
}
