package config

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Ledger contains core node-specific settings that are not
// a part of the ProtocolConfiguration (which is common for every node on the
// network).
type Ledger struct {
	// GarbageCollectionPeriod sets the number of blocks to wait before
	// starting the next MPT garbage collection cycle when RemoveUntraceableBlocks
	// option is used.
	GarbageCollectionPeriod uint32
	// KeepOnlyLatestState specifies if MPT should only store the latest state.
	// If true, DB size will be smaller, but older roots won't be accessible.
	// This value should remain the same for the same database.
	KeepOnlyLatestState bool
	// RemoveUntraceableBlocks specifies if old data (blocks, headers,
	// transactions, execution results, transfer logs and MPT data) should be
	// removed.
	RemoveUntraceableBlocks bool
	// SaveStorageBatch enables storage batch saving before every persist.
	SaveStorageBatch bool
	// SkipBlockVerification allows to disable verification of received
	// blocks (including cryptographic checks).
	SkipBlockVerification bool
	// SaveInvocations enables smart contract invocation data saving.
	SaveInvocations bool
	// TrustedHeader is an index/hash of header that can be used to start
	// light node headers synchronisation from (without additional verification).
	// It's valid iff RemoveUntraceableBlocks is enabled.
	TrustedHeader HashIndex
}

type ledgerAux struct {
	GarbageCollectionPeriod uint32                  `yaml:"GarbageCollectionPeriod"`
	KeepOnlyLatestState     bool                    `yaml:"KeepOnlyLatestState"`
	RemoveUntraceableBlocks bool                    `yaml:"RemoveUntraceableBlocks"`
	SaveStorageBatch        bool                    `yaml:"SaveStorageBatch"`
	SkipBlockVerification   bool                    `yaml:"SkipBlockVerification"`
	SaveInvocations         bool                    `yaml:"SaveInvocations"`
	StartSyncFrom           map[uint32]util.Uint256 `yaml:"StartSyncFrom"`
}

// HashIndex is a structure representing hash and index of block/header.
type HashIndex struct {
	Hash  util.Uint256
	Index uint32
}

// Blockchain is a set of settings for core.Blockchain to use, it includes protocol
// settings and local node-specific ones.
type Blockchain struct {
	ProtocolConfiguration
	Ledger
	NeoFSBlockFetcher
	NeoFSStateFetcher
}

// Validate checks Ledger for internal consistency and returns an error if any
// invalid settings are found.
func (l Ledger) Validate() error {
	if l.TrustedHeader.Index != 0 && !l.RemoveUntraceableBlocks {
		return errors.New("TrustedHeader is set, but RemoveUntraceableBlocks is disabled")
	}
	return nil
}

// MarshalYAML implements the YAML marshaller interface.
func (l Ledger) MarshalYAML() (any, error) {
	var startSyncFrom = make(map[uint32]util.Uint256)
	if l.TrustedHeader.Index != 0 {
		startSyncFrom[l.TrustedHeader.Index] = l.TrustedHeader.Hash
	}
	return ledgerAux{
		GarbageCollectionPeriod: l.GarbageCollectionPeriod,
		KeepOnlyLatestState:     l.KeepOnlyLatestState,
		RemoveUntraceableBlocks: l.RemoveUntraceableBlocks,
		SaveStorageBatch:        l.SaveStorageBatch,
		SkipBlockVerification:   l.SkipBlockVerification,
		SaveInvocations:         l.SaveInvocations,
		StartSyncFrom:           startSyncFrom,
	}, nil
}

// UnmarshalYAML implements the YAML Unmarshaler interface.
func (l *Ledger) UnmarshalYAML(unmarshal func(any) error) error {
	var aux ledgerAux

	err := unmarshal(&aux)
	if err != nil {
		return err
	}
	if len(aux.StartSyncFrom) > 1 {
		return fmt.Errorf("only one synchronization start height is supported, StartSyncFrom contains %d entries", len(aux.StartSyncFrom))
	}

	var trustedHeader HashIndex
	if len(aux.StartSyncFrom) > 0 {
		for i, h := range aux.StartSyncFrom {
			trustedHeader = HashIndex{
				Hash:  h,
				Index: i,
			}
		}
	}
	*l = Ledger{
		GarbageCollectionPeriod: aux.GarbageCollectionPeriod,
		KeepOnlyLatestState:     aux.KeepOnlyLatestState,
		RemoveUntraceableBlocks: aux.RemoveUntraceableBlocks,
		SaveStorageBatch:        aux.SaveStorageBatch,
		SkipBlockVerification:   aux.SkipBlockVerification,
		SaveInvocations:         aux.SaveInvocations,
		TrustedHeader:           trustedHeader,
	}
	return nil
}
