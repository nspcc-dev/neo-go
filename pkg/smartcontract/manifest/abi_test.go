package manifest

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/stretchr/testify/require"
)

func TestABIIsValid(t *testing.T) {
	a := &ABI{}
	require.Error(t, a.IsValid()) // No methods.

	a.Methods = append(a.Methods, Method{Name: "qwe"})
	require.NoError(t, a.IsValid())

	a.Methods = append(a.Methods, Method{Name: "qaz"})
	require.NoError(t, a.IsValid())

	a.Methods = append(a.Methods, Method{Name: "qaz", Offset: -42})
	require.Error(t, a.IsValid())

	a.Methods = append(a.Methods[:len(a.Methods)-1], Method{Name: "qwe", Parameters: []Parameter{NewParameter("param", smartcontract.BoolType)}})
	require.NoError(t, a.IsValid())

	a.Methods = append(a.Methods, Method{Name: "qwe"})
	require.Error(t, a.IsValid())
	a.Methods = a.Methods[:len(a.Methods)-1]

	a.Events = append(a.Events, Event{Name: "wsx"})
	require.NoError(t, a.IsValid())

	a.Events = append(a.Events, Event{})
	require.Error(t, a.IsValid())

	a.Events = append(a.Events[:len(a.Events)-1], Event{Name: "edc"})
	require.NoError(t, a.IsValid())

	a.Events = append(a.Events, Event{Name: "wsx"})
	require.Error(t, a.IsValid())
}
