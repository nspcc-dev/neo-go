package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Nep27 is a NEP-27 Standard.
var Nep27 = &Standard{
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{{
				Name: manifest.MethodOnNEP17Payment,
				Parameters: []manifest.Parameter{
					{Name: "from", Type: smartcontract.Hash160Type},
					{Name: "amount", Type: smartcontract.IntegerType},
					{Name: "data", Type: smartcontract.AnyType},
				},
				ReturnType: smartcontract.VoidType,
			}},
		},
	},
}
