package result

// ValidateAddress represents result of the `validateaddress` call. Notice that
// Address is an interface{} here because server echoes back whatever address
// value user has sent to it, even if it's not a string.
type ValidateAddress struct {
	Address interface{} `json:"address"`
	IsValid bool        `json:"isvalid"`
}
