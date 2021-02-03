package manifest

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestNewPermission(t *testing.T) {
	require.Panics(t, func() { NewPermission(PermissionWildcard, util.Uint160{}) })
	require.Panics(t, func() { NewPermission(PermissionHash) })
	require.Panics(t, func() { NewPermission(PermissionHash, 1) })
	require.Panics(t, func() { NewPermission(PermissionGroup) })
	require.Panics(t, func() { NewPermission(PermissionGroup, util.Uint160{}) })
}

func TestPermission_MarshalJSON(t *testing.T) {
	t.Run("wildcard", func(t *testing.T) {
		expected := NewPermission(PermissionWildcard)
		expected.Methods.Restrict()
		testMarshalUnmarshal(t, expected, NewPermission(PermissionWildcard))
	})

	t.Run("group", func(t *testing.T) {
		expected := NewPermission(PermissionWildcard)
		expected.Contract.Type = PermissionGroup
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		expected.Contract.Value = priv.PublicKey()
		expected.Methods.Add("method1")
		expected.Methods.Add("method2")
		testMarshalUnmarshal(t, expected, NewPermission(PermissionWildcard))
	})

	t.Run("hash", func(t *testing.T) {
		expected := NewPermission(PermissionWildcard)
		expected.Contract.Type = PermissionHash
		expected.Contract.Value = random.Uint160()
		testMarshalUnmarshal(t, expected, NewPermission(PermissionWildcard))
	})
}

func TestPermissionDesc_MarshalJSON(t *testing.T) {
	t.Run("uint160 with 0x", func(t *testing.T) {
		u := random.Uint160()
		s := u.StringLE()
		js := []byte(fmt.Sprintf(`"0x%s"`, s))
		d := new(PermissionDesc)
		require.NoError(t, json.Unmarshal(js, d))
		require.Equal(t, u, d.Value.(util.Uint160))
	})

	t.Run("invalid uint160", func(t *testing.T) {
		d := new(PermissionDesc)
		s := random.String(util.Uint160Size * 2)
		js := []byte(fmt.Sprintf(`"ok%s"`, s))
		require.Error(t, json.Unmarshal(js, d))

		js = []byte(fmt.Sprintf(`"%s"`, s))
		require.Error(t, json.Unmarshal(js, d))
	})

	t.Run("invalid public key", func(t *testing.T) {
		d := new(PermissionDesc)
		s := random.String(65)
		s = "k" + s // not a hex
		js := []byte(fmt.Sprintf(`"%s"`, s))
		require.Error(t, json.Unmarshal(js, d))
	})

	t.Run("not a string", func(t *testing.T) {
		d := new(PermissionDesc)
		js := []byte(`123`)
		require.Error(t, json.Unmarshal(js, d))
	})

	t.Run("invalid string", func(t *testing.T) {
		d := new(PermissionDesc)
		js := []byte(`"invalid length"`)
		require.Error(t, json.Unmarshal(js, d))
	})
}

func testMarshalUnmarshal(t *testing.T, expected, actual interface{}) {
	data, err := json.Marshal(expected)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, actual))
	require.Equal(t, expected, actual)
}

func TestPermission_ToStackItemFromStackItem(t *testing.T) {
	t.Run("wildcard", func(t *testing.T) {
		p := NewPermission(PermissionWildcard)
		expected := stackitem.NewStruct([]stackitem.Item{
			stackitem.Null{},
			stackitem.Null{},
		})
		CheckToFromStackItem(t, p, expected)
	})

	t.Run("hash", func(t *testing.T) {
		p := NewPermission(PermissionHash, util.Uint160{1, 2, 3})
		p.Methods = WildStrings{Value: []string{"a"}}
		expected := stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray(util.Uint160{1, 2, 3}.BytesBE()),
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewByteArray([]byte("a")),
			}),
		})
		CheckToFromStackItem(t, p, expected)
	})

	t.Run("group", func(t *testing.T) {
		pk, _ := keys.NewPrivateKey()
		p := NewPermission(PermissionGroup, pk.PublicKey())
		expected := stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray(pk.PublicKey().Bytes()),
			stackitem.Null{},
		})
		CheckToFromStackItem(t, p, expected)
	})
}

type Interoperable interface {
	ToStackItem() stackitem.Item
	FromStackItem(stackitem.Item) error
}

func CheckToFromStackItem(t *testing.T, source Interoperable, expected stackitem.Item) {
	actual := source.ToStackItem()
	require.Equal(t, expected, actual)
	actualSource := reflect.New(reflect.TypeOf(source).Elem()).Interface().(Interoperable)
	require.NoError(t, actualSource.FromStackItem(actual))
	require.Equal(t, source, actualSource)
}

func TestPermission_FromStackItemErrors(t *testing.T) {
	errCases := map[string]stackitem.Item{
		"not a struct":            stackitem.NewArray([]stackitem.Item{}),
		"invalid length":          stackitem.NewStruct([]stackitem.Item{}),
		"invalid contract type":   stackitem.NewStruct([]stackitem.Item{stackitem.NewArray([]stackitem.Item{}), stackitem.NewBool(false)}),
		"invalid contract length": stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{1, 2, 3}), stackitem.NewBool(false)}),
		"invalid contract pubkey": stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray(make([]byte, 33)), stackitem.NewBool(false)}),
		"invalid methods type":    stackitem.NewStruct([]stackitem.Item{stackitem.Null{}, stackitem.NewBool(false)}),
		"invalid method name":     stackitem.NewStruct([]stackitem.Item{stackitem.Null{}, stackitem.NewArray([]stackitem.Item{stackitem.NewArray([]stackitem.Item{})})}),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			p := new(Permission)
			require.Error(t, p.FromStackItem(errCase))
		})
	}
}
