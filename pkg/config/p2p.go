package config

import "time"

// P2P holds P2P node settings.
type P2P struct {
	// Addresses stores the node address list in the form of "[host]:[port][:announcedPort]".
	Addresses        []string `yaml:"Addresses"`
	AttemptConnPeers int      `yaml:"AttemptConnPeers"`
	// BroadcastFactor is the factor (0-100) controlling gossip fan-out number optimization.
	BroadcastFactor    int           `yaml:"BroadcastFactor"`
	DialTimeout        time.Duration `yaml:"DialTimeout"`
	ExtensiblePoolSize int           `yaml:"ExtensiblePoolSize"`
	MaxPeers           int           `yaml:"MaxPeers"`
	MinPeers           int           `yaml:"MinPeers"`
	PingInterval       time.Duration `yaml:"PingInterval"`
	PingTimeout        time.Duration `yaml:"PingTimeout"`
	ProtoTickInterval  time.Duration `yaml:"ProtoTickInterval"`
}
