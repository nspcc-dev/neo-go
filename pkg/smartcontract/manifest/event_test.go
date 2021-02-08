package manifest

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestEventIsValid(t *testing.T) {
	e := Event{}
	require.Error(t, e.IsValid())

	e.Name = "some"
	require.NoError(t, e.IsValid())

	e.Parameters = make([]Parameter, 0)
	require.NoError(t, e.IsValid())

	e.Parameters = append(e.Parameters, NewParameter("p1", smartcontract.BoolType))
	require.NoError(t, e.IsValid())

	e.Parameters = append(e.Parameters, NewParameter("p2", smartcontract.IntegerType))
	require.NoError(t, e.IsValid())

	e.Parameters = append(e.Parameters, NewParameter("p3", smartcontract.IntegerType))
	require.NoError(t, e.IsValid())

	e.Parameters = append(e.Parameters, NewParameter("p1", smartcontract.IntegerType))
	require.Error(t, e.IsValid())
}

func TestEvent_ToStackItemFromStackItem(t *testing.T) {
	m := &Event{
		Name:       "mur",
		Parameters: []Parameter{{Name: "p1", Type: smartcontract.BoolType}},
	}
	expected := stackitem.NewStruct([]stackitem.Item{
		stackitem.NewByteArray([]byte(m.Name)),
		stackitem.NewArray([]stackitem.Item{
			stackitem.NewStruct([]stackitem.Item{
				stackitem.NewByteArray([]byte(m.Parameters[0].Name)),
				stackitem.NewBigInteger(big.NewInt(int64(m.Parameters[0].Type))),
			}),
		}),
	})
	CheckToFromStackItem(t, m, expected)
}

func TestEvent_FromStackItemErrors(t *testing.T) {
	errCases := map[string]stackitem.Item{
		"not a struct":            stackitem.NewArray([]stackitem.Item{}),
		"invalid length":          stackitem.NewStruct([]stackitem.Item{}),
		"invalid name type":       stackitem.NewStruct([]stackitem.Item{stackitem.NewInterop(nil), stackitem.Null{}}),
		"invalid parameters type": stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.Null{}}),
		"invalid parameter":       stackitem.NewStruct([]stackitem.Item{stackitem.NewByteArray([]byte{}), stackitem.NewArray([]stackitem.Item{stackitem.NewStruct([]stackitem.Item{})})}),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			p := new(Event)
			require.Error(t, p.FromStackItem(errCase))
		})
	}
}
