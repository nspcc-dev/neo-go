package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Nep11WithRoyalty is a NEP-24 Standard for NFT royalties.
var Nep11WithRoyalty = &Standard{
	Base: Nep11Base,
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "RoyaltyInfo",
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
