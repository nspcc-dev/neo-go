package config

import (
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
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
		AddressVersion byte `yaml:"AddressVersion"`
		// EnableStateRoot specifies if exchange of state roots should be enabled.
		EnableStateRoot bool `yaml:"EnableStateRoot"`
		// KeepOnlyLatestState specifies if MPT should only store latest state.
		// If true, DB size will be smaller, but older roots won't be accessible.
		// This value should remain the same for the same database.
		KeepOnlyLatestState bool `yaml:"KeepOnlyLatestState"`
		// FeePerExtraByte sets the expected per-byte fee for
		// transactions exceeding the MaxFreeTransactionSize.
		FeePerExtraByte float64 `yaml:"FeePerExtraByte"`
		// FreeGasLimit is an amount of GAS which can be spent for free.
		// It can change over time, thus it's a map of block height to the
		// respective GAS limit.
		FreeGasLimit         map[uint32]util.Fixed8 `yaml:"FreeGasLimit"`
		LowPriorityThreshold float64                `yaml:"LowPriorityThreshold"`
		Magic                NetMode                `yaml:"Magic"`
		// Maximum number of transactions allowed to be packed into block.
		MaxTransactionsPerBlock map[uint32]int `yaml:"MaxTransactionsPerBlock"`
		// Maximum size of low priority transaction in bytes.
		MaxFreeTransactionSize int `yaml:"MaxFreeTransactionSize"`
		// Maximum number of low priority transactions accepted into block.
		MaxFreeTransactionsPerBlock map[uint32]int `yaml:"MaxFreeTransactionsPerBlock"`
		// MinimumNetworkFee sets the minimum required network fee for transaction to pass validation.
		MinimumNetworkFee util.Fixed8 `yaml:"MinimumNetworkFee"`
		MemPoolSize       int         `yaml:"MemPoolSize"`
		// NoBonusHeight is the height where GAS generation stops.
		NoBonusHeight uint32 `yaml:"NoBonusHeight"`
		// SaveStorageBatch enables storage batch saving before every persist.
		SaveStorageBatch  bool     `yaml:"SaveStorageBatch"`
		SecondsPerBlock   int      `yaml:"SecondsPerBlock"`
		SeedList          []string `yaml:"SeedList"`
		StandbyValidators []string `yaml:"StandbyValidators"`
		// StateRootEnableIndex specifies starting height for state root calculations and exchange.
		StateRootEnableIndex uint32    `yaml:"StateRootEnableIndex"`
		SystemFee            SystemFee `yaml:"SystemFee"`
		// Whether to verify received blocks.
		VerifyBlocks bool `yaml:"VerifyBlocks"`
		// Whether to verify transactions in received blocks.
		VerifyTransactions bool `yaml:"VerifyTransactions"`
	}

	// SystemFee fees related to system.
	SystemFee struct {
		EnrollmentTransaction int64 `yaml:"EnrollmentTransaction"`
		IssueTransaction      int64 `yaml:"IssueTransaction"`
		PublishTransaction    int64 `yaml:"PublishTransaction"`
		RegisterTransaction   int64 `yaml:"RegisterTransaction"`
	}

	// NetMode describes the mode the blockchain will operate on.
	NetMode uint32
)

// GetFreeGas returns FreeGasLimit value for given block height.
func (p *ProtocolConfiguration) GetFreeGas(block uint32) util.Fixed8 {
	var gas util.Fixed8
	var height uint32
	for h, g := range p.FreeGasLimit {
		if h > block || h < height {
			continue
		}
		gas = g
		height = h
	}
	return gas
}

func getIntFromMap(m map[uint32]int, block uint32) int {
	var res int
	var height uint32
	for h, i := range m {
		if h > block || h < height || i < 0 {
			continue
		}
		res = i
		height = h
	}
	return res
}

// GetMaxTxPerBlock returns MaxTransactionsPerBlock value for given block height.
func (p *ProtocolConfiguration) GetMaxTxPerBlock(block uint32) int {
	return getIntFromMap(p.MaxTransactionsPerBlock, block)
}

// GetMaxFreeTxPerBlock returns MaxFreeTransactionsPerBlock value for given block height.
func (p *ProtocolConfiguration) GetMaxFreeTxPerBlock(block uint32) int {
	return getIntFromMap(p.MaxFreeTransactionsPerBlock, block)
}

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

// TryGetValue returns the system fee base on transaction type.
func (s SystemFee) TryGetValue(txType transaction.TXType) util.Fixed8 {
	switch txType {
	case transaction.EnrollmentType:
		return util.Fixed8FromInt64(s.EnrollmentTransaction)
	case transaction.IssueType:
		return util.Fixed8FromInt64(s.IssueTransaction)
	case transaction.PublishType:
		return util.Fixed8FromInt64(s.PublishTransaction)
	case transaction.RegisterType:
		return util.Fixed8FromInt64(s.RegisterTransaction)
	default:
		return util.Fixed8FromInt64(0)
	}
}
