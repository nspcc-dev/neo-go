package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

var decimalTokenBase = &Standard{
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
