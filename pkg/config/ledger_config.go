package config

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
	// RemoveUntraceableBlocks specifies if old data should be removed.
	RemoveUntraceableBlocks bool `yaml:"RemoveUntraceableBlocks"`
	// SaveStorageBatch enables storage batch saving before every persist.
	SaveStorageBatch bool `yaml:"SaveStorageBatch"`
	// VerifyBlocks controls block verification checks (including cryptography).
	VerifyBlocks bool `yaml:"VerifyBlocks"`
}

// Blockchain is a set of settings for core.Blockchain to use, it includes protocol
// settings and local node-specific ones.
type Blockchain struct {
	ProtocolConfiguration
	Ledger
}
