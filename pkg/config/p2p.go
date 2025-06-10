package config

import "time"

// P2P holds P2P node settings.
type P2P struct {
	// Addresses stores the node address list in the form of "[host]:[port][:announcedPort]".
	Addresses        []string `yaml:"Addresses"`
	AttemptConnPeers int      `yaml:"AttemptConnPeers"`
	// BroadcastFactor is the factor (0-100) controlling gossip fan-out number optimization.
	BroadcastFactor int `yaml:"BroadcastFactor"`
	// BroadcastTxsBatchDelay is a time for txs batch collection before broadcasting them.
	BroadcastTxsBatchDelay time.Duration `yaml:"BroadcastTxsBatchDelay"`
	DialTimeout            time.Duration `yaml:"DialTimeout"`
	DisableCompression     bool          `yaml:"DisableCompression"`
	ExtensiblePoolSize     int           `yaml:"ExtensiblePoolSize"`
	MaxPeers               int           `yaml:"MaxPeers"`
	MinPeers               int           `yaml:"MinPeers"`
	PingInterval           time.Duration `yaml:"PingInterval"`
	PingTimeout            time.Duration `yaml:"PingTimeout"`
	ProtoTickInterval      time.Duration `yaml:"ProtoTickInterval"`
}
