package config

import (
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
)

// ApplicationConfiguration config specific to the node.
type ApplicationConfiguration struct {
	Address           string                   `yaml:"Address"`
	AnnouncedNodePort uint16                   `yaml:"AnnouncedPort"`
	AttemptConnPeers  int                      `yaml:"AttemptConnPeers"`
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
