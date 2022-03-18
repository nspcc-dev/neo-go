package flags

import (
	"flag"
	"io"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/stretchr/testify/require"
)

func TestFixed8_String(t *testing.T) {
	value := fixedn.Fixed8(123)
	f := Fixed8{
		Value: value,
	}

	require.Equal(t, "0.00000123", f.String())
}

func TestFixed8_Set(t *testing.T) {
	value := fixedn.Fixed8(123)
	f := Fixed8{}

	require.Error(t, f.Set("not-a-fixed8"))

	require.NoError(t, f.Set("0.00000123"))
	require.Equal(t, value, f.Value)
}

func TestFixed8_Fixed8(t *testing.T) {
	f := Fixed8{
		Value: fixedn.Fixed8(123),
	}

	require.Equal(t, fixedn.Fixed8(123), f.Fixed8())
}

func TestFixed8Flag_String(t *testing.T) {
	flag := Fixed8Flag{
		Name:  "myFlag",
		Usage: "Gas amount",
	}

	require.Equal(t, "--myFlag value\tGas amount", flag.String())
}

func TestFixed8Flag_GetName(t *testing.T) {
	flag := Fixed8Flag{
		Name: "myFlag",
	}

	require.Equal(t, "myFlag", flag.GetName())
}

func TestFixed8(t *testing.T) {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.SetOutput(io.Discard) // don't pollute test output
	gas := Fixed8Flag{Name: "gas, g"}
	gas.Apply(f)
	require.NoError(t, f.Parse([]string{"--gas", "0.123"}))
	require.Equal(t, "0.123", f.Lookup("g").Value.String())
	require.NoError(t, f.Parse([]string{"-g", "0.456"}))
	require.Equal(t, "0.456", f.Lookup("g").Value.String())
	require.Error(t, f.Parse([]string{"--gas", "kek"}))
}
