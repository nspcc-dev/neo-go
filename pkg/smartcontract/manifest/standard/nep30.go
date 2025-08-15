package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// Nep30 is a NEP-30 Standard describing smart contract verify method functionality.
// Parameters = nil for Verify, because NEP-30 allows an undefined number of
// parameters for this method. This tells the checkMethod that the exact parameter
// count is unspecified and must not be strictly enforced.
var Nep30 = &Standard{
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name:       "verify",
					Safe:       true,
					ReturnType: smartcontract.BoolType,
				},
			},
		},
	},
}
