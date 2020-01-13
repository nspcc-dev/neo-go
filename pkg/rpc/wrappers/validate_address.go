package wrappers

// ValidateAddressResponse represents response to validate address call. Notice
// Address is an interface{} here because server echoes back whatever address
// value user has sent to it, even if it's not a string.
type ValidateAddressResponse struct {
	Address interface{} `json:"address"`
	IsValid bool        `json:"isvalid"`
}
