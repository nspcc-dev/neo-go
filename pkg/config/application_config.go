package config

import (
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
)

// ApplicationConfiguration config specific to the node.
type ApplicationConfiguration struct {
	Address           string `yaml:"Address"`
	AnnouncedNodePort uint16 `yaml:"AnnouncedPort"`
	AttemptConnPeers  int    `yaml:"AttemptConnPeers"`
	// BroadcastFactor is the factor (0-100) controlling gossip fan-out number optimization.
	BroadcastFactor   int                      `yaml:"BroadcastFactor"`
	DBConfiguration   dbconfig.DBConfiguration `yaml:"DBConfiguration"`
	DialTimeout       int64                    `yaml:"DialTimeout"`
	LogPath           string                   `yaml:"LogPath"`
	MaxPeers          int                      `yaml:"MaxPeers"`
	MinPeers          int                      `yaml:"MinPeers"`
	NodePort          uint16                   `yaml:"NodePort"`
	PingInterval      int64                    `yaml:"PingInterval"`
	PingTimeout       int64                    `yaml:"PingTimeout"`
	Pprof             BasicService             `yaml:"Pprof"`
	Prometheus        BasicService             `yaml:"Prometheus"`
	ProtoTickInterval int64                    `yaml:"ProtoTickInterval"`
	Relay             bool                     `yaml:"Relay"`
	RPC               RPC                      `yaml:"RPC"`
	UnlockWallet      Wallet                   `yaml:"UnlockWallet"`
	Oracle            OracleConfiguration      `yaml:"Oracle"`
	P2PNotary         P2PNotary                `yaml:"P2PNotary"`
	StateRoot         StateRoot                `yaml:"StateRoot"`
	// ExtensiblePoolSize is the maximum amount of the extensible payloads from a single sender.
	ExtensiblePoolSize int `yaml:"ExtensiblePoolSize"`
}

// EqualsButServices returns true when the o is the same as a except for services
// (Oracle, P2PNotary, Pprof, Prometheus, RPC, StateRoot and UnlockWallet sections).
func (a *ApplicationConfiguration) EqualsButServices(o *ApplicationConfiguration) bool {
	if a.Address != o.Address ||
		a.AnnouncedNodePort != o.AnnouncedNodePort ||
		a.AttemptConnPeers != o.AttemptConnPeers ||
		a.BroadcastFactor != o.BroadcastFactor ||
		a.DBConfiguration != o.DBConfiguration ||
		a.DialTimeout != o.DialTimeout ||
		a.ExtensiblePoolSize != o.ExtensiblePoolSize ||
		a.LogPath != o.LogPath ||
		a.MaxPeers != o.MaxPeers ||
		a.MinPeers != o.MinPeers ||
		a.NodePort != o.NodePort ||
		a.PingInterval != o.PingInterval ||
		a.PingTimeout != o.PingTimeout ||
		a.ProtoTickInterval != o.ProtoTickInterval ||
		a.Relay != o.Relay {
		return false
	}
	return true
}
