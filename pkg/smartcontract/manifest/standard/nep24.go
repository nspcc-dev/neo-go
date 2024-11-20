package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// MethodRoyaltyInfo is the name of the method that returns royalty information.
const MethodRoyaltyInfo = "royaltyInfo"

// Nep24 is a NEP-24 Standard for NFT royalties.
var Nep24 = &Standard{
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: MethodRoyaltyInfo,
					Parameters: []manifest.Parameter{
						{Name: "tokenId", Type: smartcontract.ByteArrayType},
						{Name: "royaltyToken", Type: smartcontract.Hash160Type},
						{Name: "salePrice", Type: smartcontract.IntegerType},
					},
					ReturnType: smartcontract.ArrayType,
					Safe:       true,
				},
			},
		},
	},
	Required: []string{manifest.NEP11StandardName},
}

// Nep24Payable contains an event that MUST be triggered after marketplaces
// transferring royalties to the royalty recipient if royaltyInfo method is implemented.
var Nep24Payable = &Standard{
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Events: []manifest.Event{
				{
					Name: "RoyaltiesTransferred",
					Parameters: []manifest.Parameter{
						{Name: "royaltyToken", Type: smartcontract.Hash160Type},
						{Name: "royaltyRecipient", Type: smartcontract.Hash160Type},
						{Name: "buyer", Type: smartcontract.Hash160Type},
						{Name: "tokenId", Type: smartcontract.ByteArrayType},
						{Name: "amount", Type: smartcontract.IntegerType},
					},
				},
			},
		},
	},
}
