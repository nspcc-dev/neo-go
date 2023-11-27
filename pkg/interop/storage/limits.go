package storage

// Contract storage limits.
const (
	// MaxKeyLen is the maximum length of a key for storage items.
	// Contracts can't use keys longer than that in their requests to the DB.
	MaxKeyLen = 64
	// MaxValueLen is the maximum length of a value for storage items.
	// It is set to be the maximum value for uint16, contracts can't put
	// values longer than that into the DB.
	MaxValueLen = 65535
)
