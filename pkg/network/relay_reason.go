package network

//go:generate stringer -type=RelayReason -output=relay_reason_string.go

// RelayReason is the type which describes the different relay outcome.
type RelayReason uint8

// List of valid RelayReason.
const (
	RelaySucceed RelayReason = iota
	RelayAlreadyExists
	RelayOutOfMemory
	RelayUnableToVerify
	RelayInvalid
	RelayPolicyFail
	RelayUnknown
)
