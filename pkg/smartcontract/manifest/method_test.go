package manifest

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestMethod_ToStackItemFromStackItem(t *testing.T) {
	m := &Method{
		Name:       "mur",
		Offset:     5,
		Parameters: []Parameter{{Name: "p1", Type: smartcontract.BoolType}},
		ReturnType: smartcontract.StringType,
		Safe:       true,
	}
	expected := stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray([]byte(m.Name)),
		stackitem.NewArray([]stackitem.Item{
			stackitem.NewStruct([]stackitem.Item{
				stackitem.NewByteArray([]byte(m.Parameters[0].Name)),
				stackitem.NewBigInteger(big.NewInt(int64(m.Parameters[0].Type))),
			}),
		}),
		stackitem.NewBigInteger(big.NewInt(int64(m.ReturnType))),
		stackitem.NewBigInteger(big.NewInt(int64(m.Offset))),
		stackitem.NewBool(m.Safe),
	})
	CheckToFromStackItem(t, m, expected)
}

func TestMethod_FromStackItemErrors(t *testing.T) {
	errCases := map[string]stackitem.Item{
		"not a struct":            stackitem.NewArray([]stackitem.Item{}),
		"invalid length":          stackitem.NewStruct([]stackitem.Item{}),
		"invalid name type":       stackitem.NewStruct([]stackitem.Item{stackitem.NewInterop(nil), stackitem.Null{}, stackitem.Null{}, stackitem.Null{}, stackitem.Null{}}),
		"invalid parameters type": stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.Null{}, stackitem.Null{}, stackitem.Null{}, stackitem.Null{}}),
		"invalid parameter":       stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.NewArray([]stackitem.Item{stackitem.NewStruct([]stackitem.Item{})}), stackitem.Null{}, stackitem.Null{}, stackitem.Null{}}),
		"invalid return type":     stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.NewArray([]stackitem.Item{}), stackitem.Null{}, stackitem.Null{}, stackitem.Null{}}),
		"invalid offset":          stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.NewArray([]stackitem.Item{}), stackitem.NewBigInteger(big.NewInt(1)), stackitem.NewInterop(nil), stackitem.Null{}}),
		"invalid safe":            stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.NewArray([]stackitem.Item{}), stackitem.NewBigInteger(big.NewInt(1)), stackitem.NewBigInteger(big.NewInt(5)), stackitem.NewInterop(nil)}),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			p := new(Method)
			require.Error(t, p.FromStackItem(errCase))
		})
	}
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
		"not a struct":       stackitem.NewArray([]stackitem.Item{}),
		"invalid length":     stackitem.NewStruct([]stackitem.Item{}),
		"invalid name type":  stackitem.NewStruct([]stackitem.Item{stackitem.NewInterop(nil), stackitem.Null{}}),
		"invalid type type":  stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.Null{}}),
		"invalid type value": stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.NewBigInteger(big.NewInt(-100500))}),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			p := new(Parameter)
			require.Error(t, p.FromStackItem(errCase))
		})
	}
}

func TestGroup_ToStackItemFromStackItem(t *testing.T) {
	pk, _ := keys.NewPrivateKey()
	g := &Group{
		PublicKey: pk.PublicKey(),
		Signature: make([]byte, keys.SignatureLen),
	}
	expected := stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray(pk.PublicKey().Bytes()),
		stackitem.NewByteArray(make([]byte, keys.SignatureLen)),
	})
	CheckToFromStackItem(t, g, expected)
}

func TestGroup_FromStackItemErrors(t *testing.T) {
	pk, _ := keys.NewPrivateKey()
	errCases := map[string]stackitem.Item{
		"not a struct":      stackitem.NewArray([]stackitem.Item{}),
		"invalid length":    stackitem.NewStruct([]stackitem.Item{}),
		"invalid pub type":  stackitem.NewStruct([]stackitem.Item{stackitem.NewInterop(nil), stackitem.Null{}}),
		"invalid pub bytes": stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{1}), stackitem.Null{}}),
		"invalid sig type":  stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray(pk.Bytes()), stackitem.NewInterop(nil)}),
		"invalid sig len":   stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray(pk.Bytes()), stackitem.NewByteArray([]byte{1})}),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			p := new(Group)
			require.Error(t, p.FromStackItem(errCase))
		})
	}
}
