package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

var nep17 = &Standard{
	Base: decimalTokenBase,
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "balanceOf",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.Hash160Type},
					},
					ReturnType: smartcontract.IntegerType,
					Safe:       true,
				},
				{
					Name: "transfer",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.Hash160Type},
						{Type: smartcontract.Hash160Type},
						{Type: smartcontract.IntegerType},
						{Type: smartcontract.AnyType},
					},
					ReturnType: smartcontract.BoolType,
				},
			},
			Events: []manifest.Event{
				{
					Name: "Transfer",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.Hash160Type},
						{Type: smartcontract.Hash160Type},
						{Type: smartcontract.IntegerType},
					},
				},
			},
		},
	},
}
