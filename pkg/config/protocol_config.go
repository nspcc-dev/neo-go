package config

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

// ProtocolConfiguration represents the protocol config.
type (
	ProtocolConfiguration struct {
		// CommitteeHistory stores committee size change history (height: size).
		CommitteeHistory map[uint32]uint32 `yaml:"CommitteeHistory"`
		// Genesis stores genesis-related settings including a set of NeoGo
		// extensions that should be included into genesis block or be enabled
		// at the moment of native contracts initialization.
		Genesis Genesis `yaml:"Genesis"`

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
		// P2PSigExtensions enables additional signature-related logic.
		P2PSigExtensions bool `yaml:"P2PSigExtensions"`
		// P2PStateExchangeExtensions enables additional P2P MPT state data exchange logic.
		P2PStateExchangeExtensions bool `yaml:"P2PStateExchangeExtensions"`
		// NeoFSStateSyncExtensions enables state data exchange logic via NeoFS.
		NeoFSStateSyncExtensions bool `yaml:"NeoFSStateSyncExtensions"`
		// ReservedAttributes allows to have reserved attributes range for experimental or private purposes.
		ReservedAttributes bool `yaml:"ReservedAttributes"`

		SeedList         []string `yaml:"SeedList"`
		StandbyCommittee []string `yaml:"StandbyCommittee"`
		// StateRooInHeader enables storing state root in block header.
		StateRootInHeader bool `yaml:"StateRootInHeader"`
		// StateSyncInterval is the number of blocks between state heights available for MPT state data synchronization.
		// It is valid only if P2PStateExchangeExtensions are enabled.
		StateSyncInterval int `yaml:"StateSyncInterval"`
		// TimePerBlock is the time interval between blocks that consensus nodes work with.
		// It must be an integer number of milliseconds. This value is applicable till
		// HFEchidna; use Genesis-level configuration to set up further block acceptance
		// interval.
		TimePerBlock    time.Duration `yaml:"TimePerBlock"`
		ValidatorsCount uint32        `yaml:"ValidatorsCount"`
		// Validators stores history of changes to consensus node number (height: number).
		ValidatorsHistory map[uint32]uint32 `yaml:"ValidatorsHistory"`
		// Whether to verify transactions in the received blocks.
		VerifyTransactions bool `yaml:"VerifyTransactions"`
	}
)

// heightNumber is an auxiliary structure for configuration checks.
type heightNumber struct {
	h uint32
	n uint32
}

// Validate checks ProtocolConfiguration for internal consistency and returns
// an error if anything inappropriate found. Other methods can rely on protocol
// validity after this.
func (p *ProtocolConfiguration) Validate() error {
	var err error

	if p.TimePerBlock%time.Millisecond != 0 {
		return errors.New("TimePerBlock must be an integer number of milliseconds")
	}
	if p.Genesis.TimePerBlock%time.Millisecond != 0 {
		return errors.New("Genesis TimePerBlock must be an integer number of milliseconds")
	}
	for name := range p.Hardforks {
		if !IsHardforkValid(name) {
			return fmt.Errorf("Hardforks configuration section contains unexpected hardfork: %s", name)
		}
	}
	var (
		prev             uint32
		shouldBeDisabled bool
	)
	for _, cfgHf := range Hardforks {
		h := p.Hardforks[cfgHf.String()]
		if h != 0 && shouldBeDisabled {
			return fmt.Errorf("missing previous hardfork configuration with %s present", cfgHf.String())
		}
		if h != 0 && h < prev {
			return fmt.Errorf("hardfork %s has inconsistent enabling height %d (lower than the previouse one)", cfgHf.String(), h)
		}
		if h != 0 {
			prev = h
		} else if prev != 0 {
			shouldBeDisabled = true
		}
	}
	if p.ValidatorsCount != 0 && len(p.ValidatorsHistory) != 0 || p.ValidatorsCount == 0 && len(p.ValidatorsHistory) == 0 {
		return errors.New("configuration should either have one of ValidatorsCount or ValidatorsHistory, not both")
	}

	if len(p.StandbyCommittee) == 0 {
		return errors.New("configuration should include StandbyCommittee")
	}
	if len(p.StandbyCommittee) < int(p.ValidatorsCount) {
		return errors.New("validators count can't exceed the size of StandbyCommittee")
	}
	var arr = make([]heightNumber, 0, len(p.CommitteeHistory))
	for h, n := range p.CommitteeHistory {
		if n == 0 {
			return fmt.Errorf("invalid CommitteeHistory: bad members count (%d) for height %d", n, h)
		}
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
		if n == 0 {
			return fmt.Errorf("invalid ValidatorsHistory: bad members count (%d) for height %d", n, h)
		}
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
	slices.SortFunc(arr, func(a, b heightNumber) int { return cmp.Compare(a.h, b.h) })
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
	return int(getBestFromMap(p.CommitteeHistory, height))
}

func getBestFromMap(dict map[uint32]uint32, height uint32) uint32 {
	var res uint32
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
		return int(p.ValidatorsCount)
	}
	return int(getBestFromMap(p.ValidatorsHistory, height))
}

// ShouldUpdateCommitteeAt answers the question of whether the committee
// should be updated at the given height.
func (p *ProtocolConfiguration) ShouldUpdateCommitteeAt(height uint32) bool {
	return height%uint32(p.GetCommitteeSize(height)) == 0
}

// Equals allows to compare two ProtocolConfiguration instances, returns true if
// they're equal.
func (p *ProtocolConfiguration) Equals(o *ProtocolConfiguration) bool {
	if p.InitialGASSupply != o.InitialGASSupply ||
		p.Magic != o.Magic ||
		p.MaxBlockSize != o.MaxBlockSize ||
		p.MaxBlockSystemFee != o.MaxBlockSystemFee ||
		p.MaxTraceableBlocks != o.MaxTraceableBlocks ||
		p.MaxTransactionsPerBlock != o.MaxTransactionsPerBlock ||
		p.MaxValidUntilBlockIncrement != o.MaxValidUntilBlockIncrement ||
		p.MemPoolSize != o.MemPoolSize ||
		p.P2PNotaryRequestPayloadPoolSize != o.P2PNotaryRequestPayloadPoolSize ||
		p.P2PSigExtensions != o.P2PSigExtensions ||
		p.P2PStateExchangeExtensions != o.P2PStateExchangeExtensions ||
		p.ReservedAttributes != o.ReservedAttributes ||
		p.StateRootInHeader != o.StateRootInHeader ||
		p.StateSyncInterval != o.StateSyncInterval ||
		p.TimePerBlock != o.TimePerBlock ||
		p.Genesis.MaxValidUntilBlockIncrement != o.Genesis.MaxValidUntilBlockIncrement ||
		p.Genesis.TimePerBlock != o.Genesis.TimePerBlock ||
		p.ValidatorsCount != o.ValidatorsCount ||
		p.VerifyTransactions != o.VerifyTransactions ||
		!maps.Equal(p.CommitteeHistory, o.CommitteeHistory) ||
		!maps.Equal(p.Hardforks, o.Hardforks) ||
		!slices.Equal(p.SeedList, o.SeedList) ||
		!slices.Equal(p.StandbyCommittee, o.StandbyCommittee) ||
		!maps.Equal(p.ValidatorsHistory, o.ValidatorsHistory) {
		return false
	}
	return true
}
