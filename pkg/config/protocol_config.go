package config

import (
	"errors"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

// ProtocolConfiguration represents the protocol config.
type (
	ProtocolConfiguration struct {
		// CommitteeHistory stores committee size change history (height: size).
		CommitteeHistory map[uint32]int `yaml:"CommitteeHistory"`
		// DBFTWatchdogThresholdMultiplier is a dBFT reinitialization threshold multiplier. If
		// no block is accepted within DBFTWatchdogThresholdMultiplier*SecondsPerBlock seconds
		// since the last accepted block, dBFT service is forced to completely reset its state.
		// DBFTWatchdogThresholdMultiplier is valid only if EnableDBFTWatchdog
		// setting is on. DBFTWatchdogThresholdMultiplier must be the same within the whole set
		// of consensus nodes in a single network.
		DBFTWatchdogThresholdMultiplier int `yaml:"DBFTWatchdogThresholdMultiplier"`
		// EnableDBFTWatchdog states whether dBFT watchdog should be enabled. If true,
		// then dBFT service is forced to reset after DBFTWatchdogThresholdMultiplier*SecondsPerBlock
		// seconds after the last accepted block timestamp if no block is accepted. DO NOT use this
		// feature on public networks.
		EnableDBFTWatchdog bool `yaml:"EnableDBFTWatchdog"`

		// GarbageCollectionPeriod sets the number of blocks to wait before
		// starting the next MPT garbage collection cycle when RemoveUntraceableBlocks
		// option is used.
		GarbageCollectionPeriod uint32 `yaml:"GarbageCollectionPeriod"`

		Magic       netmode.Magic `yaml:"Magic"`
		MemPoolSize int           `yaml:"MemPoolSize"`

		// Hardforks is a map of hardfork names that enables version-specific application
		// logic dependent on the specified height.
		Hardforks map[string]uint32 `yaml:"Hardforks"`
		// InitialGASSupply is the amount of GAS generated in the genesis block.
		InitialGASSupply fixedn.Fixed8 `yaml:"InitialGASSupply"`
		// P2PNotaryRequestPayloadPoolSize specifies the memory pool size for P2PNotaryRequestPayloads.
		// It is valid only if P2PSigExtensions are enabled.
		P2PNotaryRequestPayloadPoolSize int `yaml:"P2PNotaryRequestPayloadPoolSize"`
		// KeepOnlyLatestState specifies if MPT should only store the latest state.
		// If true, DB size will be smaller, but older roots won't be accessible.
		// This value should remain the same for the same database.
		KeepOnlyLatestState bool `yaml:"KeepOnlyLatestState"`
		// RemoveUntraceableBlocks specifies if old data should be removed.
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
		// NativeUpdateHistories is a list of histories of native contracts updates.
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
		// Validators stores history of changes to consensus node number (height: number).
		ValidatorsHistory map[uint32]int `yaml:"ValidatorsHistory"`
		// Whether to verify received blocks.
		VerifyBlocks bool `yaml:"VerifyBlocks"`
		// Whether to verify transactions in the received blocks.
		VerifyTransactions bool `yaml:"VerifyTransactions"`
	}
)

// heightNumber is an auxiliary structure for configuration checks.
type heightNumber struct {
	h uint32
	n int
}

// Validate checks ProtocolConfiguration for internal consistency and returns
// an error if anything inappropriate found. Other methods can rely on protocol
// validity after this.
func (p *ProtocolConfiguration) Validate() error {
	var err error

	if p.KeepOnlyLatestState && p.P2PStateExchangeExtensions {
		return errors.New("can't have both KeepOnlyLatestState and P2PStateExchangeExtensions")
	}
	for name := range p.NativeUpdateHistories {
		if !nativenames.IsValid(name) {
			return fmt.Errorf("NativeActivations configuration section contains unexpected native contract name: %s", name)
		}
	}
	for name := range p.Hardforks {
		if !IsHardforkValid(name) {
			return fmt.Errorf("Hardforks configuration section contains unexpected hardfork: %s", name)
		}
	}
	if p.EnableDBFTWatchdog && p.DBFTWatchdogThresholdMultiplier < 0 {
		return errors.New("DBFTWatchdogThresholdMultiplier must not be negative")
	}
	if p.ValidatorsCount != 0 && len(p.ValidatorsHistory) != 0 {
		return errors.New("configuration should either have ValidatorsCount or ValidatorsHistory, not both")
	}
	if len(p.StandbyCommittee) < p.ValidatorsCount {
		return errors.New("validators count can't exceed the size of StandbyCommittee")
	}
	var arr = make([]heightNumber, 0, len(p.CommitteeHistory))
	for h, n := range p.CommitteeHistory {
		if int(n) > len(p.StandbyCommittee) {
			return fmt.Errorf("too small StandbyCommittee for required number of committee members at %d", h)
		}
		arr = append(arr, heightNumber{h, n})
	}
	if len(arr) != 0 {
		err = sortCheckZero(arr, "CommitteeHistory")
		if err != nil {
			return err
		}
		for i, hn := range arr[1:] {
			if int64(hn.h)%int64(hn.n) != 0 || int64(hn.h)%int64(arr[i].n) != 0 {
				return fmt.Errorf("invalid CommitteeHistory: bad %d height for %d and %d committee", hn.h, hn.n, arr[i].n)
			}
		}
	}
	arr = arr[:0]
	for h, n := range p.ValidatorsHistory {
		if int(n) > len(p.StandbyCommittee) {
			return fmt.Errorf("too small StandbyCommittee for required number of validators at %d", h)
		}
		arr = append(arr, heightNumber{h, n})
	}
	if len(arr) != 0 {
		err = sortCheckZero(arr, "ValidatorsHistory")
		if err != nil {
			return err
		}
		for _, hn := range arr {
			if int64(hn.n) > int64(p.GetCommitteeSize(hn.h)) {
				return fmt.Errorf("requested number of validators is too big: %d at %d", hn.n, hn.h)
			}
			if int64(hn.h)%int64(p.GetCommitteeSize(hn.h)) != 0 {
				return fmt.Errorf("validators number change is not aligned with committee change at %d", hn.h)
			}
		}
	}
	return nil
}

// sortCheckZero sorts heightNumber array and checks for zero height presence.
func sortCheckZero(arr []heightNumber, field string) error {
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].h < arr[j].h
	})
	if arr[0].h != 0 {
		return fmt.Errorf("invalid %s: no height 0 specified", field)
	}
	return nil
}

// GetCommitteeSize returns the committee size for the given height. It implies
// valid configuration file.
func (p *ProtocolConfiguration) GetCommitteeSize(height uint32) int {
	if len(p.CommitteeHistory) == 0 {
		return len(p.StandbyCommittee)
	}
	return getBestFromMap(p.CommitteeHistory, height)
}

func getBestFromMap(dict map[uint32]int, height uint32) int {
	var res int
	var bestH = uint32(0)
	for h, n := range dict {
		if h >= bestH && h <= height {
			res = n
			bestH = h
		}
	}
	return res
}

// GetNumOfCNs returns the number of validators for the given height.
// It implies valid configuration file.
func (p *ProtocolConfiguration) GetNumOfCNs(height uint32) int {
	if len(p.ValidatorsHistory) == 0 {
		return p.ValidatorsCount
	}
	return getBestFromMap(p.ValidatorsHistory, height)
}

// ShouldUpdateCommitteeAt answers the question of whether the committee
// should be updated at the given height.
func (p *ProtocolConfiguration) ShouldUpdateCommitteeAt(height uint32) bool {
	return height%uint32(p.GetCommitteeSize(height)) == 0
}
