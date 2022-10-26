package standard

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/stretchr/testify/require"
)

func fooMethodBarEvent() *manifest.Manifest {
	return &manifest.Manifest{
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name: "foo",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.ByteArrayType},
						{Type: smartcontract.PublicKeyType},
					},
					ReturnType: smartcontract.IntegerType,
					Safe:       true,
				},
			},
			Events: []manifest.Event{
				{
					Name: "bar",
					Parameters: []manifest.Parameter{
						{Type: smartcontract.StringType},
					},
				},
			},
		},
	}
}

func TestComplyMissingMethod(t *testing.T) {
	m := fooMethodBarEvent()
	m.ABI.GetMethod("foo", -1).Name = "notafoo"
	err := Comply(m, &Standard{Manifest: *fooMethodBarEvent()})
	require.True(t, errors.Is(err, ErrMethodMissing))
}

func TestComplyInvalidReturnType(t *testing.T) {
	m := fooMethodBarEvent()
	m.ABI.GetMethod("foo", -1).ReturnType = smartcontract.VoidType
	err := Comply(m, &Standard{Manifest: *fooMethodBarEvent()})
	require.True(t, errors.Is(err, ErrInvalidReturnType))
}

func TestComplyMethodParameterCount(t *testing.T) {
	t.Run("Method", func(t *testing.T) {
		m := fooMethodBarEvent()
		f := m.ABI.GetMethod("foo", -1)
		f.Parameters = append(f.Parameters, manifest.Parameter{Type: smartcontract.BoolType})
		err := Comply(m, &Standard{Manifest: *fooMethodBarEvent()})
		require.True(t, errors.Is(err, ErrMethodMissing))
	})
	t.Run("Event", func(t *testing.T) {
		m := fooMethodBarEvent()
		ev := m.ABI.GetEvent("bar")
		ev.Parameters = ev.Parameters[:0]
		err := Comply(m, &Standard{Manifest: *fooMethodBarEvent()})
		require.True(t, errors.Is(err, ErrInvalidParameterCount))
	})
}

func TestComplyParameterType(t *testing.T) {
	t.Run("Method", func(t *testing.T) {
		m := fooMethodBarEvent()
		m.ABI.GetMethod("foo", -1).Parameters[0].Type = smartcontract.InteropInterfaceType
		err := Comply(m, &Standard{Manifest: *fooMethodBarEvent()})
		require.True(t, errors.Is(err, ErrInvalidParameterType))
	})
	t.Run("Event", func(t *testing.T) {
		m := fooMethodBarEvent()
		m.ABI.GetEvent("bar").Parameters[0].Type = smartcontract.InteropInterfaceType
		err := Comply(m, &Standard{Manifest: *fooMethodBarEvent()})
		require.True(t, errors.Is(err, ErrInvalidParameterType))
	})
}

func TestComplyParameterName(t *testing.T) {
	t.Run("Method", func(t *testing.T) {
		m := fooMethodBarEvent()
		m.ABI.GetMethod("foo", -1).Parameters[0].Name = "hehe"
		s := &Standard{Manifest: *fooMethodBarEvent()}
		err := Comply(m, s)
		require.True(t, errors.Is(err, ErrInvalidParameterName))
		require.NoError(t, ComplyABI(m, s))
	})
	t.Run("Event", func(t *testing.T) {
		m := fooMethodBarEvent()
		m.ABI.GetEvent("bar").Parameters[0].Name = "hehe"
		s := &Standard{Manifest: *fooMethodBarEvent()}
		err := Comply(m, s)
		require.True(t, errors.Is(err, ErrInvalidParameterName))
		require.NoError(t, ComplyABI(m, s))
	})
}

func TestMissingEvent(t *testing.T) {
	m := fooMethodBarEvent()
	m.ABI.GetEvent("bar").Name = "notabar"
	err := Comply(m, &Standard{Manifest: *fooMethodBarEvent()})
	require.True(t, errors.Is(err, ErrEventMissing))
}

func TestSafeFlag(t *testing.T) {
	m := fooMethodBarEvent()
	m.ABI.GetMethod("foo", -1).Safe = false
	err := Comply(m, &Standard{Manifest: *fooMethodBarEvent()})
	require.True(t, errors.Is(err, ErrSafeMethodMismatch))
}

func TestComplyValid(t *testing.T) {
	m := fooMethodBarEvent()
	m.ABI.Methods = append(m.ABI.Methods, manifest.Method{
		Name:       "newmethod",
		Offset:     123,
		ReturnType: smartcontract.ByteArrayType,
	})
	m.ABI.Events = append(m.ABI.Events, manifest.Event{
		Name: "otherevent",
		Parameters: []manifest.Parameter{{
			Name: "names do not matter",
			Type: smartcontract.IntegerType,
		}},
	})
	require.NoError(t, Comply(m, &Standard{Manifest: *fooMethodBarEvent()}))
}

func TestCheck(t *testing.T) {
	m := manifest.NewManifest("Test")
	require.Error(t, Check(m, manifest.NEP17StandardName))

	m.ABI.Methods = append(m.ABI.Methods, DecimalTokenBase.ABI.Methods...)
	m.ABI.Methods = append(m.ABI.Methods, Nep17.ABI.Methods...)
	m.ABI.Events = append(m.ABI.Events, Nep17.ABI.Events...)
	require.NoError(t, Check(m, manifest.NEP17StandardName))
	require.NoError(t, CheckABI(m, manifest.NEP17StandardName))
}

func TestOptional(t *testing.T) {
	var m Standard
	m.Optional = []manifest.Method{{
		Name:       "optMet",
		Parameters: []manifest.Parameter{{Type: smartcontract.ByteArrayType}},
		ReturnType: smartcontract.IntegerType,
	}}

	t.Run("wrong parameter count, do not check", func(t *testing.T) {
		var actual manifest.Manifest
		actual.ABI.Methods = []manifest.Method{{
			Name:       "optMet",
			ReturnType: smartcontract.IntegerType,
		}}
		require.NoError(t, Comply(&actual, &m))
	})
	t.Run("good parameter count, bad return", func(t *testing.T) {
		var actual manifest.Manifest
		actual.ABI.Methods = []manifest.Method{{
			Name:       "optMet",
			Parameters: []manifest.Parameter{{Type: smartcontract.ArrayType}},
			ReturnType: smartcontract.IntegerType,
		}}
		require.Error(t, Comply(&actual, &m))
	})
	t.Run("good parameter count, good return", func(t *testing.T) {
		var actual manifest.Manifest
		actual.ABI.Methods = []manifest.Method{{
			Name:       "optMet",
			Parameters: []manifest.Parameter{{Type: smartcontract.ByteArrayType}},
			ReturnType: smartcontract.IntegerType,
		}}
		require.NoError(t, Comply(&actual, &m))
	})
}
