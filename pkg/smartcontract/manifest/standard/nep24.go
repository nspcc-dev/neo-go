package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Nep24Royalty is a NEP-24 Standard for NFT royalties.
var Nep24Royalty = &Standard{
	//Base: DecimalTokenBase,
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "royaltyInfo",
					Parameters: []manifest.Parameter{
						{Name: "tokenId", Type: smartcontract.ByteArrayType},
						{Name: "royaltyToken", Type: smartcontract.Hash160Type},
						{Name: "salePrice", Type: smartcontract.IntegerType},
					},
					ReturnType: smartcontract.ArrayType,
					Safe:       true,
				},
			},
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
