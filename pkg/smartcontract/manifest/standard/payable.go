package standard

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
)

var nep11payable = &Standard{
	Manifest: manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{{
				Name: manifest.MethodOnNEP11Payment,
				Parameters: []manifest.Parameter{
					{Name: "from", Type: smartcontract.Hash160Type},
					{Name: "amount", Type: smartcontract.IntegerType},
					{Name: "tokenid", Type: smartcontract.ByteArrayType},
				},
				ReturnType: smartcontract.VoidType,
			}},
		},
	},
}

var nep17payable = &Standard{
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
