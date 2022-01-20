package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

var nep11Base = &Standard{
	Base: decimalTokenBase,
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "balanceOf",
					Parameters: []manifest.Parameter{
						{Name: "owner", Type: smartcontract.Hash160Type},
					},
					ReturnType: smartcontract.IntegerType,
					Safe:       true,
				},
				{
					Name: "tokensOf",
					Parameters: []manifest.Parameter{
						{Name: "owner", Type: smartcontract.Hash160Type},
					},
					ReturnType: smartcontract.InteropInterfaceType,
					Safe:       true,
				},
				{
					Name: "transfer",
					Parameters: []manifest.Parameter{
						{Name: "to", Type: smartcontract.Hash160Type},
						{Name: "tokenId", Type: smartcontract.ByteArrayType},
						{Name: "data", Type: smartcontract.AnyType},
					},
					ReturnType: smartcontract.BoolType,
				},
			},
			Events: []manifest.Event{
				{
					Name: "Transfer",
					Parameters: []manifest.Parameter{
						{Name: "from", Type: smartcontract.Hash160Type},
						{Name: "to", Type: smartcontract.Hash160Type},
						{Name: "amount", Type: smartcontract.IntegerType},
						{Name: "tokenId", Type: smartcontract.ByteArrayType},
					},
				},
			},
		},
	},
	Optional: []manifest.Method{
		{
			Name: "properties",
			Parameters: []manifest.Parameter{
				{Name: "tokenId", Type: smartcontract.ByteArrayType},
			},
			ReturnType: smartcontract.MapType,
			Safe:       true,
		},
		{
			Name:       "tokens",
			ReturnType: smartcontract.InteropInterfaceType,
			Safe:       true,
		},
	},
}

var nep11NonDivisible = &Standard{
	Base: nep11Base,
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "ownerOf",
					Parameters: []manifest.Parameter{
						{Name: "tokenId", Type: smartcontract.ByteArrayType},
					},
					ReturnType: smartcontract.Hash160Type,
					Safe:       true,
				},
			},
		},
	},
}

var nep11Divisible = &Standard{
	Base: nep11Base,
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "balanceOf",
					Parameters: []manifest.Parameter{
						{Name: "owner", Type: smartcontract.Hash160Type},
						{Name: "tokenId", Type: smartcontract.ByteArrayType},
					},
					ReturnType: smartcontract.IntegerType,
					Safe:       true,
				},
				{
					Name: "ownerOf",
					Parameters: []manifest.Parameter{
						{Name: "tokenId", Type: smartcontract.ByteArrayType},
					},
					ReturnType: smartcontract.InteropInterfaceType, // iterator
					Safe:       true,
				},
				{
					Name: "transfer",
					Parameters: []manifest.Parameter{
						{Name: "from", Type: smartcontract.Hash160Type},
						{Name: "to", Type: smartcontract.Hash160Type},
						{Name: "amount", Type: smartcontract.IntegerType},
						{Name: "tokenId", Type: smartcontract.ByteArrayType},
						{Name: "data", Type: smartcontract.AnyType},
					},
					ReturnType: smartcontract.BoolType,
				},
			},
		},
	},
}
