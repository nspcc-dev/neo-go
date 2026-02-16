package manifest

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParametersAreValid(t *testing.T) {
	ps := Parameters{}
	require.NoError(t, ps.AreValid()) // No parameters.

	ps = append(ps, Parameter{})
	require.Error(t, ps.AreValid())

	ps[0].Name = "qwerty"
	require.NoError(t, ps.AreValid())

	ps[0].Type = 0x42 // Invalid type.
	require.Error(t, ps.AreValid())

	ps[0].Type = smartcontract.VoidType
	require.Error(t, ps.AreValid())

	ps[0].Type = smartcontract.BoolType
	require.NoError(t, ps.AreValid())

	ps[0].ExtendedType = &ExtendedType{Type: 0x42}
	require.Error(t, ps.AreValid())

	ps[0].ExtendedType = &ExtendedType{Type: smartcontract.IntegerType}
	require.NoError(t, ps.AreValid())

	ps = append(ps, Parameter{Name: "qwerty"})
	require.Error(t, ps.AreValid())
}

func TestParameter_ToStackItemFromStackItem(t *testing.T) {
	p := &Parameter{
		Name: "param",
		Type: smartcontract.StringType,
	}
	expected := stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray([]byte(p.Name)),
		stackitem.NewBigInteger(big.NewInt(int64(p.Type))),
	})
	CheckToFromStackItem(t, p, expected)
}

func TestParameter_FromStackItemErrors(t *testing.T) {
	errCases := map[string]stackitem.Item{
		"not a struct":                        stackitem.NewArray([]stackitem.Item{}),
		"invalid length":                      stackitem.NewStruct([]stackitem.Item{}),
		"invalid name type":                   stackitem.NewStruct([]stackitem.Item{stackitem.NewInterop(nil), stackitem.Null{}}),
		"invalid type type":                   stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.Null{}}),
		"invalid type value":                  stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.NewBigInteger(big.NewInt(-100500))}),
		"invalid ExtendedType stackitem type": stackitem.NewStruct([]stackitem.Item{stackitem.Make([]byte{}), stackitem.Make(int(smartcontract.IntegerType)), stackitem.NewMap()}),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			p := new(Parameter)
			require.Error(t, p.FromStackItem(errCase))
		})
	}
}

func TestParameter_UnmarshalYAML(t *testing.T) {
	testCases := []struct {
		name        string
		src         string
		expected    Parameter
		expectedErr string
	}{
		{
			name:        "invalid yaml",
			src:         `invalid`,
			expectedErr: "cannot unmarshal",
		},
		{
			name:        "invalid field type",
			src:         `type: invalid`,
			expectedErr: "bad parameter type",
		},
		{
			name:        "invalid extended type",
			src:         `extendedtype: invalid`,
			expectedErr: "cannot unmarshal !!str `invalid`",
		},
		{
			name: "difficult parameter",
			src: `
name: p
extendedtype:
  type: Array
  namedtype: SomeStruct
  fields:
    - field: S
      extendedtype:
        type: Array
        namedtype: InternalStruct
        fields:
        - name: B
          type: Boolean
`,
			expectedErr: "",
			expected: Parameter{
				Name: "p",
				Type: smartcontract.ArrayType, // should be set from extendedtype.base
				ExtendedType: &ExtendedType{
					Type: smartcontract.ArrayType,
					Name: "SomeStruct",
					Fields: []Parameter{
						{
							Name: "S",
							Type: smartcontract.ArrayType,
							ExtendedType: &ExtendedType{
								Type: smartcontract.ArrayType,
								Name: "InternalStruct",
								Fields: []Parameter{
									{
										Name: "B",
										Type: smartcontract.BoolType,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "conflicting type and extendedtype",
			src: `
type: Integer
extendedtype:
  type: String
`,
			expectedErr: "conflicting types",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var p Parameter
			err := yaml.Unmarshal([]byte(tc.src), &p)

			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, p)
			}
		})
	}
}

func TestParameter_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name        string
		src         string
		expected    Parameters
		expectedErr string
	}{
		{
			name:        "invalid json",
			src:         `invalid`,
			expectedErr: "cannot unmarshal",
		},
		{
			name:        "invalid field type",
			src:         `{"type":"invalid"}`,
			expectedErr: "bad parameter type",
		},
		{
			name:        "invalid extended type",
			src:         `{"extendedtype":"invalid"}`,
			expectedErr: "cannot unmarshal !!str `invalid`",
		},
		{
			name: "conflicting type and extendedtype",
			src: `
type: Integer
extendedtype:
  type: String
`,
			expectedErr: "conflicting types",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var p Parameter
			err := yaml.Unmarshal([]byte(tc.src), &p)

			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, p)
			}
		})
	}
}
