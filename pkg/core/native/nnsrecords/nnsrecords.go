package nnsrecords

// Type represents name record type.
type Type byte

// Pre-defined record types.
const (
	A     Type = 1
	CNAME Type = 5
	TXT   Type = 16
	AAAA  Type = 28
)
