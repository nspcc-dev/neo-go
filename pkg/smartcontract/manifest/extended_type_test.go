package manifest

import (
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestExtendedType_ToStackItemFromStackItem(t *testing.T) {
	testCases := []struct {
		et       *ExtendedType
		expected stackitem.Item
	}{
		{
			et: &ExtendedType{
				Type:       smartcontract.MapType,
				Key:        smartcontract.IntegerType,
				ForbidNull: true,
				Value: &ExtendedType{
					Type:   smartcontract.ByteArrayType,
					Length: 30,
				},
			},
			expected: stackitem.NewMapWithValue([]stackitem.MapElement{
				{Key: stackitem.Make("type"), Value: stackitem.Make(int(smartcontract.MapType))},
				{Key: stackitem.Make("forbidnull"), Value: stackitem.Make(true)},
				{Key: stackitem.Make("key"), Value: stackitem.Make(int(smartcontract.IntegerType))},
				{Key: stackitem.Make("value"), Value: stackitem.NewMapWithValue([]stackitem.MapElement{
					{Key: stackitem.Make("type"), Value: stackitem.Make(int(smartcontract.ByteArrayType))},
					{Key: stackitem.Make("length"), Value: stackitem.Make(30)},
				})},
			}),
		},
		{
			et: &ExtendedType{
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
			expected: stackitem.NewMapWithValue([]stackitem.MapElement{
				{Key: stackitem.Make("type"), Value: stackitem.Make(int(smartcontract.ArrayType))},
				{Key: stackitem.Make("namedtype"), Value: stackitem.Make("SomeStruct")},
				{Key: stackitem.Make("fields"), Value: stackitem.NewArray([]stackitem.Item{
					stackitem.NewStruct([]stackitem.Item{
						stackitem.Make("S"),
						stackitem.Make(int(smartcontract.ArrayType)),
						stackitem.NewMapWithValue([]stackitem.MapElement{
							{Key: stackitem.Make("type"), Value: stackitem.Make(int(smartcontract.ArrayType))},
							{Key: stackitem.Make("namedtype"), Value: stackitem.Make("InternalStruct")},
							{Key: stackitem.Make("fields"), Value: stackitem.NewArray([]stackitem.Item{
								stackitem.NewStruct([]stackitem.Item{
									stackitem.Make("B"),
									stackitem.Make(int(smartcontract.BoolType)),
								}),
							})},
						}),
					}),
				})},
			}),
		},
	}
	for _, tc := range testCases {
		CheckToFromStackItem(t, tc.et, tc.expected)
	}
}

func TestExtendedType_FromStackItemErrors(t *testing.T) {
	errCases := map[string]stackitem.Item{
		"expected non-nil item":               nil,
		"invalid ExtendedType stackitem type": stackitem.Make(nil),
		"incorrect type":                      stackitem.NewMap(),
		"type must be integer": stackitem.NewMapWithValue([]stackitem.MapElement{
			{Key: stackitem.Make("type"), Value: stackitem.Make("invalidtype")}},
		),
		"can't get namedtype": stackitem.NewMapWithValue([]stackitem.MapElement{
			{Key: stackitem.Make("type"), Value: stackitem.Make(0)},
			{Key: stackitem.Make("namedtype"), Value: stackitem.Make(nil)}},
		),
		"length must be integer or null": stackitem.NewMapWithValue([]stackitem.MapElement{
			{Key: stackitem.Make("type"), Value: stackitem.Make(0)},
			{Key: stackitem.Make("length"), Value: stackitem.Make(nil)}},
		),
		"forbidnull must be boolean or null": stackitem.NewMapWithValue([]stackitem.MapElement{
			{Key: stackitem.Make("type"), Value: stackitem.Make(0)},
			{Key: stackitem.Make("forbidnull"), Value: stackitem.Make(nil)}},
		),
		"interface must be bytearray or null": stackitem.NewMapWithValue([]stackitem.MapElement{
			{Key: stackitem.Make("type"), Value: stackitem.Make(0)},
			{Key: stackitem.Make("interface"), Value: stackitem.Make(nil)}},
		),
		"key must be integer or null": stackitem.NewMapWithValue([]stackitem.MapElement{
			{Key: stackitem.Make("type"), Value: stackitem.Make(0)},
			{Key: stackitem.Make("key"), Value: stackitem.Make(nil)}},
		),
		"can't get value": stackitem.NewMapWithValue([]stackitem.MapElement{
			{Key: stackitem.Make("type"), Value: stackitem.Make(0)},
			{Key: stackitem.Make("value"), Value: stackitem.Make(nil)}},
		),
		"fields must be array or null": stackitem.NewMapWithValue([]stackitem.MapElement{
			{Key: stackitem.Make("type"), Value: stackitem.Make(0)},
			{Key: stackitem.Make("fields"), Value: stackitem.Make(nil)}},
		),
		"invalid Parameter stackitem type": stackitem.NewMapWithValue([]stackitem.MapElement{
			{Key: stackitem.Make("type"), Value: stackitem.Make(0)},
			{Key: stackitem.Make("fields"), Value: stackitem.NewArray([]stackitem.Item{
				stackitem.Make(nil),
			})}},
		),
	}
	for err, errCase := range errCases {
		t.Run(err, func(t *testing.T) {
			e := new(ExtendedType)
			require.ErrorContains(t, e.FromStackItem(errCase), err)
		})
	}
}

func TestExtendedType_IsValid(t *testing.T) {
	et := ExtendedType{Type: smartcontract.ParamType(-42)}
	require.Error(t, et.IsValid())
	et.Type = smartcontract.UnknownType
	et.Name = strings.Repeat("a", 65)
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Name` field can not be specified")
	et.Type = smartcontract.ArrayType
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Name` must not be longer than 64 characters")
	et.Name = "42"
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Name` must start with a letter and contain only letters, digits and dots")
	et.Name = "a_"
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Name` must start with a letter and contain only letters, digits and dots")
	et.Type = smartcontract.UnknownType
	et.Name = ""
	et.Length = 42
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Length` field can not be specified")
	et.Length = 0
	et.ForbidNull = true
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.ForbidNull` field can not be specified")
	et.ForbidNull = false
	et.Interface = "SomeInterface"
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Interface` field can not be specified")
	et.Type = smartcontract.InteropInterfaceType
	require.ErrorContains(t, et.IsValid(), "invalid value for `ExtendedType.Interface` field")
	et.Interface = ""
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Interface` field is required")
	et.Type = smartcontract.UnknownType
	et.Key = smartcontract.UnknownType
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Key` field can not be specified")
	et.Type = smartcontract.MapType
	require.ErrorContains(t, et.IsValid(), "not allowed for map definitions")
	et.Key = smartcontract.AnyType
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Key` field is required")
	et.Type = smartcontract.ArrayType
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Value` field is required")
	et.Value = &ExtendedType{Type: smartcontract.ParamType(-42)}
	require.Error(t, et.IsValid())
	et.Type = smartcontract.UnknownType
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Value` field can not be specified")
	et.Value = nil
	et.Fields = []Parameter{{Name: "p", Type: smartcontract.ParamType(-42)}}
	require.ErrorContains(t, et.IsValid(), "`ExtendedType.Fields` field can not be specified")
	et.Type = smartcontract.ArrayType
	et.Value = &ExtendedType{Type: smartcontract.UnknownType}
	require.Error(t, et.IsValid())
	et.Fields = nil
	require.Nil(t, et.IsValid())
}
