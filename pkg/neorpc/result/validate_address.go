package result

// ValidateAddress represents a result of the `validateaddress` call. Notice that
// Address is an interface{} here because the server echoes back whatever address
// value a user has sent to it, even if it's not a string.
type ValidateAddress struct {
	Address any  `json:"address"`
	IsValid bool `json:"isvalid"`
}
