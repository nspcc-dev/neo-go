package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Nep31 is a NEP-31 Standard describing smart contract destroy functionality.
var Nep31 = &Standard{
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name:       "destroy",
					Parameters: []manifest.Parameter{},
					ReturnType: smartcontract.VoidType,
				},
			},
		},
	},
}
