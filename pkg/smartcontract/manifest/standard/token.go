package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

// DecimalTokenBase contains methods common to NEP-11 and NEP-17 token standards.
var DecimalTokenBase = &Standard{
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name:       "decimals",
					ReturnType: smartcontract.IntegerType,
					Safe:       true,
				},
				{
					Name:       "symbol",
					ReturnType: smartcontract.StringType,
					Safe:       true,
				},
				{
					Name:       "totalSupply",
					ReturnType: smartcontract.IntegerType,
					Safe:       true,
				},
			},
		},
	},
}
