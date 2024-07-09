package flags

import (
	"flag"
	"io"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
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

func TestFixed8Flag_Names(t *testing.T) {
	flag := Fixed8Flag{
		Name: "myFlag",
	}

	require.Equal(t, []string{"myFlag"}, flag.Names())
}

func TestFixed8(t *testing.T) {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.SetOutput(io.Discard) // don't pollute test output
	gas := Fixed8Flag{Name: "gas", Aliases: []string{"g"}, Usage: "Gas amount", Value: Fixed8{Value: 0}, Required: true, Hidden: false, Action: nil}
	err := gas.Apply(f)
	require.NoError(t, err)
	require.NoError(t, f.Parse([]string{"--gas", "0.123"}))
	require.Equal(t, "0.123", f.Lookup("g").Value.String())
	require.NoError(t, f.Parse([]string{"-g", "0.456"}))
	require.Equal(t, "0.456", f.Lookup("g").Value.String())
	require.Error(t, f.Parse([]string{"--gas", "kek"}))
}

func TestFixed8Flag_Get(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(app, set, nil)
	flag := Fixed8Flag{
		Name: "testFlag",
	}
	fixedFlag := Fixed8{Value: fixedn.Fixed8(123)}
	set.Var(&fixedFlag, "testFlag", "test usage")
	require.NoError(t, set.Set("testFlag", "0.00000321"))
	expected := flag.Get(ctx)
	require.Equal(t, fixedn.Fixed8(321), expected.Value)
}

func TestFixed8Flag_GetValue(t *testing.T) {
	f := Fixed8Flag{Value: Fixed8{Value: fixedn.Fixed8(123)}}
	require.Equal(t, "0.00000123", f.GetValue())
	require.True(t, f.TakesValue())
}

func TestFixed8Flag_RunAction(t *testing.T) {
	called := false
	action := func(ctx *cli.Context, s string) error {
		called = true
		require.Equal(t, "0.00000123", s)
		return nil
	}
	app := cli.NewApp()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(app, set, nil)
	f := Fixed8Flag{
		Action: action,
		Value:  Fixed8{Value: fixedn.Fixed8(123)},
	}
	err := f.RunAction(ctx)
	require.NoError(t, err)
	require.True(t, called)
}

func TestFixed8Flag_GetUsage(t *testing.T) {
	f := Fixed8Flag{Usage: "Use this flag to specify gas amount"}
	require.Equal(t, "Use this flag to specify gas amount", f.GetUsage())
}

func TestFixed8Flag_IsVisible(t *testing.T) {
	f := Fixed8Flag{Hidden: false}
	require.True(t, f.IsVisible())

	f.Hidden = true
	require.False(t, f.IsVisible())
}

func TestFixed8Flag_IsRequired(t *testing.T) {
	f := Fixed8Flag{Required: false}
	require.False(t, f.IsRequired())

	f.Required = true
	require.True(t, f.IsRequired())
}
