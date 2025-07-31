package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Nep31 is a NEP-31 Standard.
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
			Events: []manifest.Event{
				{
					Name: "Destroy",
					Parameters: manifest.Parameters{
						{
							Name: "contract",
							Type: smartcontract.Hash160Type,
						},
					},
				},
			},
		},
	},
}
