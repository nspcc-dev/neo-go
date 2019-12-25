package wrappers

import (
	"github.com/CityOfZion/neo-go/pkg/encoding/address"
)

// ValidateAddressResponse represents response to validate address call.
type ValidateAddressResponse struct {
	Address interface{} `json:"address"`
	IsValid bool        `json:"isvalid"`
}

// ValidateAddress verifies that the address is a correct NEO address
// see https://docs.neo.org/en-us/node/cli/2.9.4/api/validateaddress.html
func ValidateAddress(addr interface{}) ValidateAddressResponse {
	resp := ValidateAddressResponse{Address: addr}
	if addr, ok := addr.(string); ok {
		_, err := address.DecodeUint160(addr)
		resp.IsValid = err == nil
	}
	return resp
}
