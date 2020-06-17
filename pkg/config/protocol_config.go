package config

import (
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// ModeMainNet contains magic code used in the NEO main official network.
	ModeMainNet NetMode = 0x00746e41 // 7630401
	// ModeTestNet contains magic code used in the NEO testing network.
	ModeTestNet NetMode = 0x74746e41 // 1953787457
	// ModePrivNet contains magic code usually used for NEO private networks.
	ModePrivNet NetMode = 56753 // docker privnet
	// ModeUnitTestNet is a stub magic code used for testing purposes.
	ModeUnitTestNet NetMode = 0
)

// ProtocolConfiguration represents the protocol config.
type (
	ProtocolConfiguration struct {
		// FeePerExtraByte sets the expected per-byte fee for
		// transactions exceeding the MaxFreeTransactionSize.
		FeePerExtraByte float64 `yaml:"FeePerExtraByte"`
		// FreeGasLimit is an amount of GAS which can be spent for free.
		FreeGasLimit            util.Fixed8 `yaml:"FreeGasLimit"`
		LowPriorityThreshold    float64     `yaml:"LowPriorityThreshold"`
		Magic                   NetMode     `yaml:"Magic"`
		MaxTransactionsPerBlock int         `yaml:"MaxTransactionsPerBlock"`
		// Maximum size of low priority transaction in bytes.
		MaxFreeTransactionSize int `yaml:"MaxFreeTransactionSize"`
		// Maximum number of low priority transactions accepted into block.
		MaxFreeTransactionsPerBlock int `yaml:"MaxFreeTransactionsPerBlock"`
		MemPoolSize                 int `yaml:"MemPoolSize"`
		// SaveStorageBatch enables storage batch saving before every persist.
		SaveStorageBatch  bool     `yaml:"SaveStorageBatch"`
		SecondsPerBlock   int      `yaml:"SecondsPerBlock"`
		SeedList          []string `yaml:"SeedList"`
		StandbyValidators []string `yaml:"StandbyValidators"`
		// Whether to verify received blocks.
		VerifyBlocks bool `yaml:"VerifyBlocks"`
		// Whether to verify transactions in received blocks.
		VerifyTransactions bool `yaml:"VerifyTransactions"`
	}

	// NetMode describes the mode the blockchain will operate on.
	NetMode uint32
)

// String implements the stringer interface.
func (n NetMode) String() string {
	switch n {
	case ModePrivNet:
		return "privnet"
	case ModeTestNet:
		return "testnet"
	case ModeMainNet:
		return "mainnet"
	case ModeUnitTestNet:
		return "unit_testnet"
	default:
		return "net unknown"
	}
}
