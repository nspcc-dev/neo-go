package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Nep30 is a NEP-30 Standard describing smart contract verify method functionality.
var Nep30 = &Standard{
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					// nil parameters implies no parameters check, NEP-30 allows an
					// undefined number of parameters for verify.
					Name:       "verify",
					Safe:       true,
					ReturnType: smartcontract.BoolType,
				},
			},
		},
	},
}
