package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Nep22 is a NEP-22 Standard describing smart contract update functionality.
var Nep22 = &Standard{
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "update",
					Parameters: []manifest.Parameter{
						{Name: "nefFile", Type: smartcontract.ByteArrayType},
						{Name: "manifest", Type: smartcontract.ByteArrayType},
						{Name: "data", Type: smartcontract.AnyType},
					},
					ReturnType: smartcontract.VoidType,
				},
			},
		},
	},
}
