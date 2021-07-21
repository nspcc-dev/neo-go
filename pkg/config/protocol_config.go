package config

import (
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

// ProtocolConfiguration represents the protocol config.
type (
	ProtocolConfiguration struct {
		Magic       netmode.Magic `yaml:"Magic"`
		MemPoolSize int           `yaml:"MemPoolSize"`

		// InitialGASSupply is the amount of GAS generated in the genesis block.
		InitialGASSupply fixedn.Fixed8 `yaml:"InitialGASSupply"`
		// P2PNotaryRequestPayloadPoolSize specifies the memory pool size for P2PNotaryRequestPayloads.
		// It is valid only if P2PSigExtensions are enabled.
		P2PNotaryRequestPayloadPoolSize int `yaml:"P2PNotaryRequestPayloadPoolSize"`
		// KeepOnlyLatestState specifies if MPT should only store latest state.
		// If true, DB size will be smaller, but older roots won't be accessible.
		// This value should remain the same for the same database.
		KeepOnlyLatestState bool `yaml:"KeepOnlyLatestState"`
		// RemoveUntraceableBlocks specifies if old blocks should be removed.
		RemoveUntraceableBlocks bool `yaml:"RemoveUntraceableBlocks"`
		// MaxBlockSize is the maximum block size in bytes.
		MaxBlockSize uint32 `yaml:"MaxBlockSize"`
		// MaxBlockSystemFee is the maximum overall system fee per block.
		MaxBlockSystemFee int64 `yaml:"MaxBlockSystemFee"`
		// MaxTraceableBlocks is the length of the chain accessible to smart contracts.
		MaxTraceableBlocks uint32 `yaml:"MaxTraceableBlocks"`
		// MaxTransactionsPerBlock is the maximum amount of transactions per block.
		MaxTransactionsPerBlock uint16 `yaml:"MaxTransactionsPerBlock"`
		// MaxValidUntilBlockIncrement is the upper increment size of blockchain height in blocks
		// exceeding that a transaction should fail validation. It is set to estimated daily number
		// of blocks with 15s interval.
		MaxValidUntilBlockIncrement uint32 `yaml:"MaxValidUntilBlockIncrement"`
		// NativeUpdateHistories is the list of histories of native contracts updates.
		NativeUpdateHistories map[string][]uint32 `yaml:"NativeActivations"`
		// P2PSigExtensions enables additional signature-related logic.
		P2PSigExtensions bool `yaml:"P2PSigExtensions"`
		// P2PStateExchangeExtensions enables additional P2P MPT state data exchange logic.
		P2PStateExchangeExtensions bool `yaml:"P2PStateExchangeExtensions"`
		// ReservedAttributes allows to have reserved attributes range for experimental or private purposes.
		ReservedAttributes bool `yaml:"ReservedAttributes"`
		// SaveStorageBatch enables storage batch saving before every persist.
		SaveStorageBatch bool     `yaml:"SaveStorageBatch"`
		SecondsPerBlock  int      `yaml:"SecondsPerBlock"`
		SeedList         []string `yaml:"SeedList"`
		StandbyCommittee []string `yaml:"StandbyCommittee"`
		// StateRooInHeader enables storing state root in block header.
		StateRootInHeader bool `yaml:"StateRootInHeader"`
		// StateSyncInterval is the number of blocks between state heights available for MPT state data synchronization.
		// It is valid only if P2PStateExchangeExtensions are enabled.
		StateSyncInterval int `yaml:"StateSyncInterval"`
		ValidatorsCount   int `yaml:"ValidatorsCount"`
		// Whether to verify received blocks.
		VerifyBlocks bool `yaml:"VerifyBlocks"`
		// Whether to verify transactions in received blocks.
		VerifyTransactions bool `yaml:"VerifyTransactions"`
	}
)
