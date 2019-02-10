package wrappers

import (
	"github.com/CityOfZion/neo-go/pkg/crypto"
)

type ValidateAddressResponse struct {
	Address string `json:"address"`
	IsValid bool   `json:"isvalid"`
}

// ValidateAddress verifies that the address is a correct NEO address
// see https://docs.neo.org/en-us/node/cli/2.9.4/api/validateaddress.html
func ValidateAddress(address string) (*ValidateAddressResponse, error) {
	_, err := crypto.Uint160DecodeAddress(address)
	return &ValidateAddressResponse{
		Address: address,
		IsValid: err == nil,
	}, nil
}
