package manifest

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestExtendedType_UnmarshalYAML(t *testing.T) {
	testCases := []struct {
		name        string
		src         string
		expected    ExtendedType
		expectedErr string
	}{
		{
			name:        "invalid yaml",
			expectedErr: "line 1: cannot unmarshal !!str `invalid` into map[string]interface",
			src:         `invalid`,
		},
		{
			name:        "invalid field type",
			expectedErr: "bad parameter type: invalid",
			src:         `type: invalid`,
		},
		{
			name: "difficult extended type",
			expected: ExtendedType{
				Base:       smartcontract.MapType,
				Key:        smartcontract.ArrayType,
				ForbidNull: true,
				Value: &ExtendedType{
					Base: smartcontract.ArrayType,
					Name: "SomeStruct",
					Fields: []Parameter{
						{
							Name: "N",
							Type: smartcontract.IntegerType,
						},
						{
							Name: "S",
							Type: smartcontract.ArrayType,
							ExtendedType: &ExtendedType{
								Base: smartcontract.ArrayType,
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
			src: `
base: Map
forbidnull: true
key: Array
value:
    base: Array
    name: SomeStruct
    fields:
        - name: "N"
          type: Integer
        - name: S
          type: Array
          extendedtype:
            base: Array
            name: InternalStruct
            fields:
                - name: B
                  type: Boolean
`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				et  ExtendedType
				err = yaml.Unmarshal([]byte(tc.src), &et)
			)
			if tc.expectedErr != "" {
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.True(t, et.Equals(&tc.expected))
			}
		})
	}
}

func TestExtendedType_UnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name        string
		src         string
		expected    ExtendedType
		expectedErr string
	}{
		{
			name:        "invalid json",
			expectedErr: "invalid character",
			src:         `invalid`,
		},
		{
			name:        "invalid field type",
			expectedErr: "bad parameter type: invalid",
			src:         `{"type":"invalid"}`,
		},
		{
			name: "difficult extended type",
			expected: ExtendedType{
				Base:       smartcontract.MapType,
				Key:        smartcontract.ArrayType,
				ForbidNull: true,
				Value: &ExtendedType{
					Base: smartcontract.ArrayType,
					Name: "SomeStruct",
					Fields: []Parameter{
						{
							Name: "N",
							Type: smartcontract.IntegerType,
						},
						{
							Name: "S",
							Type: smartcontract.ArrayType,
							ExtendedType: &ExtendedType{
								Base: smartcontract.ArrayType,
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
			src: `
{
	"base": "Map",
	"forbidnull": true,
	"key": "Array",
	"value": {
		"base": "Array",
		"name": "SomeStruct",
		"fields": [
			{
				"name": "N",
				"type": "Integer"
			},
			{
				"name": "S",
				"type": "Array",
				"extendedtype": {
					"base": "Array",
					"name": "InternalStruct",
					"fields": [
						{
							"name": "B",
							"type": "Boolean"
						}
					]
				}
			}
		]
	}
}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				et  ExtendedType
				err = json.Unmarshal([]byte(tc.src), &et)
			)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.True(t, et.Equals(&tc.expected))
			}
		})
	}
}

func TestExtendedType_ToStackItemFromStackItem(t *testing.T) {
	testCases := []struct {
		et       *ExtendedType
		expected stackitem.Item
	}{
		{
			et: &ExtendedType{
				Base:       smartcontract.MapType,
				Key:        smartcontract.IntegerType,
				ForbidNull: true,
				Value: &ExtendedType{
					Base:   smartcontract.ByteArrayType,
					Length: 30,
				},
			},
			expected: stackitem.NewStruct([]stackitem.Item{
				stackitem.Make(int(smartcontract.MapType)),
				stackitem.Make(""),
				stackitem.Make(0),
				stackitem.Make(true),
				stackitem.Make(""),
				stackitem.Make(int(smartcontract.IntegerType)),
				stackitem.NewStruct([]stackitem.Item{
					stackitem.Make(int(smartcontract.ByteArrayType)),
					stackitem.Make(""),
					stackitem.Make(30),
					stackitem.Make(false),
					stackitem.Make(""),
					stackitem.Make(0),
					stackitem.Make(nil),
					stackitem.Make(nil),
				}),
				stackitem.Make(nil),
			}),
		},
		{
			et: &ExtendedType{
				Base: smartcontract.ArrayType,
				Name: "SomeStruct",
				Fields: []Parameter{
					{
						Name: "S",
						Type: smartcontract.ArrayType,
						ExtendedType: &ExtendedType{
							Base: smartcontract.ArrayType,
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
			expected: stackitem.NewStruct([]stackitem.Item{
				stackitem.Make(int(smartcontract.ArrayType)),
				stackitem.Make("SomeStruct"),
				stackitem.Make(0),
				stackitem.Make(false),
				stackitem.Make(""),
				stackitem.Make(0),
				stackitem.Make(nil),
				stackitem.NewArray([]stackitem.Item{
					stackitem.NewStruct([]stackitem.Item{
						stackitem.Make("S"),
						stackitem.Make(int(smartcontract.ArrayType)),
						stackitem.NewStruct([]stackitem.Item{
							stackitem.Make(int(smartcontract.ArrayType)),
							stackitem.Make("InternalStruct"),
							stackitem.Make(0),
							stackitem.Make(false),
							stackitem.Make(""),
							stackitem.Make(0),
							stackitem.Make(nil),
							stackitem.NewArray([]stackitem.Item{
								stackitem.NewStruct([]stackitem.Item{
									stackitem.Make("B"),
									stackitem.Make(int(smartcontract.BoolType)),
								}),
							}),
						}),
					}),
				}),
			}),
		},
	}
	for _, tc := range testCases {
		CheckToFromStackItem(t, tc.et, tc.expected)
	}
}

func TestExtendedType_FromStackItemErrors(t *testing.T) {
	var (
		item stackitem.Item
		et   ExtendedType
	)
	require.ErrorContains(t, et.FromStackItem(item), "expected non-nil item")
	item = stackitem.NewMap()
	require.ErrorContains(t, et.FromStackItem(item), "invalid ExtendedType stackitem type")
	item = stackitem.NewStruct(nil)
	require.ErrorContains(t, et.FromStackItem(item), "invalid ExtendedType stackitem length")
	items := make([]stackitem.Item, 8)
	item = stackitem.NewStruct(items)
	items[0] = &stackitem.Null{}
	require.ErrorContains(t, et.FromStackItem(item), "can't get ExtendedType.Base")
	items[0] = stackitem.Make(0)
	items[1] = &stackitem.Null{}
	require.ErrorContains(t, et.FromStackItem(item), "can't get ExtendedType.Name")
	items[1] = stackitem.Make("")
	items[2] = &stackitem.Null{}
	require.ErrorContains(t, et.FromStackItem(item), "can't get ExtendedType.Length")
	items[2] = stackitem.Make(0)
	items[3] = stackitem.NewByteArray(make([]byte, stackitem.MaxBigIntegerSizeBits))
	require.ErrorContains(t, et.FromStackItem(item), "can't get ExtendedType.ForbidNull")
	items[3] = stackitem.Make(false)
	items[4] = &stackitem.Null{}
	require.ErrorContains(t, et.FromStackItem(item), "can't get ExtendedType.Interface")
	items[4] = stackitem.Make("")
	items[5] = &stackitem.Null{}
	require.ErrorContains(t, et.FromStackItem(item), "can't get ExtendedType.Key")
	items[5] = stackitem.Make(0)
	items[6] = &stackitem.Null{}
	items[7] = &stackitem.Null{}
	require.NoError(t, et.FromStackItem(item))
	require.Nil(t, et.Value)
	items[6] = stackitem.NewMap()
	require.ErrorContains(t, et.FromStackItem(item), "can't get ExtendedType.Value")
	items[6] = &stackitem.Null{}
	items[7] = stackitem.NewMap()
	require.ErrorContains(t, et.FromStackItem(item), "invalid ExtendedType fields stackitem type")
	items[7] = stackitem.NewArray([]stackitem.Item{stackitem.Make(nil)})
	require.Error(t, et.FromStackItem(item))
}

func TestExtendedType_IsValid(t *testing.T) {
	et := ExtendedType{Base: smartcontract.ParamType(-42)}
	require.Error(t, et.IsValid())
	et.Base = smartcontract.UnknownType
	et.Name = "SomeStruct"
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Name` field can not be specified")
	et.Name = ""
	et.Length = 42
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Length` field can not be specified")
	et.Length = 0
	et.ForbidNull = true
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.ForbidNull` field can not be specified")
	et.ForbidNull = false
	et.Interface = "SomeInterface"
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Interface` field can not be specified")
	et.Base = smartcontract.InteropInterfaceType
	require.ErrorContains(t, et.IsValid(), "invalid value for `ExtendedType.Interface` field")
	et.Interface = ""
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Interface` field is required")
	et.Base = smartcontract.UnknownType
	et.Key = smartcontract.UnknownType
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Key` field can not be specified")
	et.Base = smartcontract.MapType
	require.ErrorContains(t, et.IsValid(), "not allowed for map definitions")
	et.Key = smartcontract.AnyType
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Key` field is required")
	et.Base = smartcontract.ArrayType
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Value` field is required")
	et.Value = &ExtendedType{Base: smartcontract.ParamType(-42)}
	require.Error(t, et.IsValid())
	et.Base = smartcontract.UnknownType
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Value` field can not be specified")
	et.Value = nil
	et.Fields = []Parameter{{Name: "p", Type: smartcontract.ParamType(-42)}}
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Fields` field can not be specified")
	et.Base = smartcontract.ArrayType
	et.Value = &ExtendedType{Base: smartcontract.UnknownType}
	require.Error(t, et.IsValid())
	et.Fields = nil
	require.Nil(t, et.IsValid())
}
