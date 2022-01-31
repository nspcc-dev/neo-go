package nns

// RecordState is a type that registered entities are saved to.
type RecordState struct {
	Name string
	Type RecordType
	Data string
}

// RecordType is domain name service record types.
type RecordType byte

// Record types defined in [RFC 1035](https://tools.ietf.org/html/rfc1035)
const (
	// A represents address record type.
	A RecordType = 1
	// CNAME represents canonical name record type.
	CNAME RecordType = 5
	// TXT represents text record type.
	TXT RecordType = 16
)

// Record types defined in [RFC 3596](https://tools.ietf.org/html/rfc3596)
const (
	// AAAA represents IPv6 address record type.
	AAAA RecordType = 28
)
