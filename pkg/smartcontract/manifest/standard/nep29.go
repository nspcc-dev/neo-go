package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Nep29 is a NEP-29 Standard describing smart contract _deploy method functionality.
var Nep29 = &Standard{
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "_deploy",
					Parameters: []manifest.Parameter{
						{Name: "data", Type: smartcontract.AnyType},
						{Name: "update", Type: smartcontract.BoolType},
					},
					ReturnType: smartcontract.VoidType,
				},
			},
		},
	},
}
