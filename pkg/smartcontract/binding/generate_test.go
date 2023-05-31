package binding

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/stretchr/testify/assert"
)

func TestExtendedType_Equals(t *testing.T) {
	crazyT := ExtendedType{
		Base:      smartcontract.StringType,
		Name:      "qwertyu",
		Interface: "qwerty",
		Key:       smartcontract.BoolType,
		Value: &ExtendedType{
			Base: smartcontract.IntegerType,
		},
		Fields: []FieldExtendedType{
			{
				Field: "qwe",
				ExtendedType: ExtendedType{
					Base:      smartcontract.IntegerType,
					Name:      "qwer",
					Interface: "qw",
					Key:       smartcontract.ArrayType,
					Fields: []FieldExtendedType{
						{
							Field: "as",
						},
					},
				},
			},
			{
				Field: "asf",
				ExtendedType: ExtendedType{
					Base: smartcontract.BoolType,
				},
			},
			{
				Field: "sffg",
				ExtendedType: ExtendedType{
					Base: smartcontract.AnyType,
				},
			},
		},
	}
	tcs := map[string]struct {
		a           *ExtendedType
		b           *ExtendedType
		expectedRes bool
	}{
		"both nil": {
			a:           nil,
			b:           nil,
			expectedRes: true,
		},
		"a is nil": {
			a:           nil,
			b:           &ExtendedType{},
			expectedRes: false,
		},
		"b is nil": {
			a:           &ExtendedType{},
			b:           nil,
			expectedRes: false,
		},
		"base mismatch": {
			a: &ExtendedType{
				Base: smartcontract.StringType,
			},
			b: &ExtendedType{
				Base: smartcontract.IntegerType,
			},
			expectedRes: false,
		},
		"name mismatch": {
			a: &ExtendedType{
				Base: smartcontract.ArrayType,
				Name: "q",
			},
			b: &ExtendedType{
				Base: smartcontract.ArrayType,
				Name: "w",
			},
			expectedRes: false,
		},
		"number of fields mismatch": {
			a: &ExtendedType{
				Base: smartcontract.ArrayType,
				Name: "q",
				Fields: []FieldExtendedType{
					{
						Field:        "IntField",
						ExtendedType: ExtendedType{Base: smartcontract.IntegerType},
					},
				},
			},
			b: &ExtendedType{
				Base: smartcontract.ArrayType,
				Name: "w",
				Fields: []FieldExtendedType{
					{
						Field:        "IntField",
						ExtendedType: ExtendedType{Base: smartcontract.IntegerType},
					},
					{
						Field:        "BoolField",
						ExtendedType: ExtendedType{Base: smartcontract.BoolType},
					},
				},
			},
			expectedRes: false,
		},
		"field names mismatch": {
			a: &ExtendedType{
				Base: smartcontract.ArrayType,
				Fields: []FieldExtendedType{
					{
						Field:        "IntField",
						ExtendedType: ExtendedType{Base: smartcontract.IntegerType},
					},
				},
			},
			b: &ExtendedType{
				Base: smartcontract.ArrayType,
				Fields: []FieldExtendedType{
					{
						Field:        "BoolField",
						ExtendedType: ExtendedType{Base: smartcontract.BoolType},
					},
				},
			},
			expectedRes: false,
		},
		"field types mismatch": {
			a: &ExtendedType{
				Base: smartcontract.ArrayType,
				Fields: []FieldExtendedType{
					{
						Field:        "Field",
						ExtendedType: ExtendedType{Base: smartcontract.IntegerType},
					},
				},
			},
			b: &ExtendedType{
				Base: smartcontract.ArrayType,
				Fields: []FieldExtendedType{
					{
						Field:        "Field",
						ExtendedType: ExtendedType{Base: smartcontract.BoolType},
					},
				},
			},
			expectedRes: false,
		},
		"interface mismatch": {
			a:           &ExtendedType{Interface: "iterator"},
			b:           &ExtendedType{Interface: "unknown"},
			expectedRes: false,
		},
		"value is nil": {
			a: &ExtendedType{
				Base: smartcontract.StringType,
			},
			b: &ExtendedType{
				Base: smartcontract.StringType,
			},
			expectedRes: true,
		},
		"a value is not nil": {
			a: &ExtendedType{
				Base:  smartcontract.ArrayType,
				Value: &ExtendedType{},
			},
			b: &ExtendedType{
				Base: smartcontract.ArrayType,
			},
			expectedRes: false,
		},
		"b value is not nil": {
			a: &ExtendedType{
				Base: smartcontract.ArrayType,
			},
			b: &ExtendedType{
				Base:  smartcontract.ArrayType,
				Value: &ExtendedType{},
			},
			expectedRes: false,
		},
		"byte array tolerance for a": {
			a: &ExtendedType{
				Base: smartcontract.StringType,
			},
			b: &ExtendedType{
				Base: smartcontract.ByteArrayType,
			},
			expectedRes: true,
		},
		"byte array tolerance for b": {
			a: &ExtendedType{
				Base: smartcontract.ByteArrayType,
			},
			b: &ExtendedType{
				Base: smartcontract.StringType,
			},
			expectedRes: true,
		},
		"key mismatch": {
			a: &ExtendedType{
				Key: smartcontract.StringType,
			},
			b: &ExtendedType{
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
