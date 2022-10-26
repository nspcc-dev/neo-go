package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Nep17 is a NEP-17 Standard.
var Nep17 = &Standard{
	Base: DecimalTokenBase,
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "balanceOf",
					Parameters: []manifest.Parameter{
						{Name: "account", Type: smartcontract.Hash160Type},
					},
					ReturnType: smartcontract.IntegerType,
					Safe:       true,
				},
				{
					Name: "transfer",
					Parameters: []manifest.Parameter{
						{Name: "from", Type: smartcontract.Hash160Type},
						{Name: "to", Type: smartcontract.Hash160Type},
						{Name: "amount", Type: smartcontract.IntegerType},
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
					},
				},
			},
		},
	},
}
