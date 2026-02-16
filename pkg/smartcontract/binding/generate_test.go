package binding

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/stretchr/testify/assert"
)

func TestExtendedType_Equals(t *testing.T) {
	crazyT := manifest.ExtendedType{
		Type:      smartcontract.StringType,
		Name:      "qwertyu",
		Interface: "qwerty",
		Key:       smartcontract.BoolType,
		Value: &manifest.ExtendedType{
			Type: smartcontract.IntegerType,
		},
		Fields: []manifest.Parameter{
			{
				Name: "qwe",
				ExtendedType: &manifest.ExtendedType{
					Type:      smartcontract.IntegerType,
					Name:      "qwer",
					Interface: "qw",
					Key:       smartcontract.ArrayType,
					Fields: []manifest.Parameter{
						{
							Name: "as",
						},
					},
				},
			},
			{
				Name: "asf",
				ExtendedType: &manifest.ExtendedType{
					Type: smartcontract.BoolType,
				},
			},
			{
				Name: "sffg",
				ExtendedType: &manifest.ExtendedType{
					Type: smartcontract.AnyType,
				},
			},
		},
	}
	tcs := map[string]struct {
		a           *manifest.ExtendedType
		b           *manifest.ExtendedType
		expectedRes bool
	}{
		"both nil": {
			a:           nil,
			b:           nil,
			expectedRes: true,
		},
		"a is nil": {
			a:           nil,
			b:           &manifest.ExtendedType{},
			expectedRes: false,
		},
		"b is nil": {
			a:           &manifest.ExtendedType{},
			b:           nil,
			expectedRes: false,
		},
		"base mismatch": {
			a: &manifest.ExtendedType{
				Type: smartcontract.StringType,
			},
			b: &manifest.ExtendedType{
				Type: smartcontract.IntegerType,
			},
			expectedRes: false,
		},
		"name mismatch": {
			a: &manifest.ExtendedType{
				Type: smartcontract.ArrayType,
				Name: "q",
			},
			b: &manifest.ExtendedType{
				Type: smartcontract.ArrayType,
				Name: "w",
			},
			expectedRes: false,
		},
		"number of fields mismatch": {
			a: &manifest.ExtendedType{
				Type: smartcontract.ArrayType,
				Name: "q",
				Fields: []manifest.Parameter{
					{
						Name:         "IntField",
						ExtendedType: &manifest.ExtendedType{Type: smartcontract.IntegerType},
					},
				},
			},
			b: &manifest.ExtendedType{
				Type: smartcontract.ArrayType,
				Name: "w",
				Fields: []manifest.Parameter{
					{
						Name:         "IntField",
						ExtendedType: &manifest.ExtendedType{Type: smartcontract.IntegerType},
					},
					{
						Name:         "BoolField",
						ExtendedType: &manifest.ExtendedType{Type: smartcontract.BoolType},
					},
				},
			},
			expectedRes: false,
		},
		"field names mismatch": {
			a: &manifest.ExtendedType{
				Type: smartcontract.ArrayType,
				Fields: []manifest.Parameter{
					{
						Name:         "IntField",
						ExtendedType: &manifest.ExtendedType{Type: smartcontract.IntegerType},
					},
				},
			},
			b: &manifest.ExtendedType{
				Type: smartcontract.ArrayType,
				Fields: []manifest.Parameter{
					{
						Name:         "BoolField",
						ExtendedType: &manifest.ExtendedType{Type: smartcontract.BoolType},
					},
				},
			},
			expectedRes: false,
		},
		"field types mismatch": {
			a: &manifest.ExtendedType{
				Type: smartcontract.ArrayType,
				Fields: []manifest.Parameter{
					{
						Name:         "Field",
						ExtendedType: &manifest.ExtendedType{Type: smartcontract.IntegerType},
					},
				},
			},
			b: &manifest.ExtendedType{
				Type: smartcontract.ArrayType,
				Fields: []manifest.Parameter{
					{
						Name:         "Field",
						ExtendedType: &manifest.ExtendedType{Type: smartcontract.BoolType},
					},
				},
			},
			expectedRes: false,
		},
		"interface mismatch": {
			a:           &manifest.ExtendedType{Interface: "iterator"},
			b:           &manifest.ExtendedType{Interface: "unknown"},
			expectedRes: false,
		},
		"value is nil": {
			a: &manifest.ExtendedType{
				Type: smartcontract.StringType,
			},
			b: &manifest.ExtendedType{
				Type: smartcontract.StringType,
			},
			expectedRes: true,
		},
		"a value is not nil": {
			a: &manifest.ExtendedType{
				Type:  smartcontract.ArrayType,
				Value: &manifest.ExtendedType{},
			},
			b: &manifest.ExtendedType{
				Type: smartcontract.ArrayType,
			},
			expectedRes: false,
		},
		"b value is not nil": {
			a: &manifest.ExtendedType{
				Type: smartcontract.ArrayType,
			},
			b: &manifest.ExtendedType{
				Type:  smartcontract.ArrayType,
				Value: &manifest.ExtendedType{},
			},
			expectedRes: false,
		},
		"byte array tolerance for a": {
			a: &manifest.ExtendedType{
				Type: smartcontract.StringType,
			},
			b: &manifest.ExtendedType{
				Type: smartcontract.ByteArrayType,
			},
			expectedRes: true,
		},
		"byte array tolerance for b": {
			a: &manifest.ExtendedType{
				Type: smartcontract.ByteArrayType,
			},
			b: &manifest.ExtendedType{
				Type: smartcontract.StringType,
			},
			expectedRes: true,
		},
		"key mismatch": {
			a: &manifest.ExtendedType{
				Key: smartcontract.StringType,
			},
			b: &manifest.ExtendedType{
				Key: smartcontract.IntegerType,
			},
			expectedRes: false,
		},
		"good nested": {
			a:           &crazyT,
			b:           &crazyT,
			expectedRes: true,
		},
	}
	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expectedRes, tc.a.Equals(tc.b))
		})
	}
}
