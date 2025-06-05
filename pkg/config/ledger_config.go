package config

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"gopkg.in/yaml.v3"
)

// Ledger contains core node-specific settings that are not
// a part of the ProtocolConfiguration (which is common for every node on the
// network).
type Ledger struct {
	// GarbageCollectionPeriod sets the number of blocks to wait before
	// starting the next MPT garbage collection cycle when RemoveUntraceableBlocks
	// option is used.
	GarbageCollectionPeriod uint32 `yaml:"GarbageCollectionPeriod"`
	// KeepOnlyLatestState specifies if MPT should only store the latest state.
	// If true, DB size will be smaller, but older roots won't be accessible.
	// This value should remain the same for the same database.
	KeepOnlyLatestState bool `yaml:"KeepOnlyLatestState"`
	// RemoveUntraceableBlocks specifies if old data (blocks, headers,
	// transactions, execution results, transfer logs and MPT data) should be
	// removed.
	RemoveUntraceableBlocks bool `yaml:"RemoveUntraceableBlocks"`
	// SaveStorageBatch enables storage batch saving before every persist.
	SaveStorageBatch bool `yaml:"SaveStorageBatch"`
	// SkipBlockVerification allows to disable verification of received
	// blocks (including cryptographic checks).
	SkipBlockVerification bool `yaml:"SkipBlockVerification"`
	// SaveInvocations enables smart contract invocation data saving.
	SaveInvocations bool `yaml:"SaveInvocations"`
	// TrustedHeader is an index/hash of header that can be used to start
	// light node headers synchronisation from (without additional verification).
	// It's valid iff RemoveUntraceableBlocks is enabled along with one of
	// P2PStateExchangeExtensions or NeoFSStateSyncExtensions.
	TrustedHeader HashIndex `yaml:"TrustedHeader"`
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
func (h HashIndex) MarshalYAML() (any, error) {
	var startSyncFrom = make(map[uint32]util.Uint256)
	if h.Index != 0 {
		startSyncFrom[h.Index] = h.Hash
	}
	return startSyncFrom, nil
}

// UnmarshalYAML implements the YAML Unmarshaler interface.
func (h *HashIndex) UnmarshalYAML(node *yaml.Node) error {
	var aux map[uint32]util.Uint256

	err := node.Decode(&aux)
	if err != nil {
		return err
	}
	if len(aux) > 1 {
		return fmt.Errorf("only one trusted height is supported, got %d entries", len(aux))
	}

	if len(aux) > 0 {
		for i, hh := range aux {
			*h = HashIndex{
				Hash:  hh,
				Index: i,
			}
		}
	}

	return nil
}
