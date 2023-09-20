package policy

// AttributeType represents a transaction attribute type.
type AttributeType byte

// List of valid transaction attribute types.
const (
	HighPriorityT   AttributeType = 1
	OracleResponseT AttributeType = 0x11
	NotValidBeforeT AttributeType = 0x20
	ConflictsT      AttributeType = 0x21
	// NotaryAssistedT is an extension of Neo protocol available on specifically configured NeoGo networks.
	NotaryAssistedT AttributeType = 0x22
)
