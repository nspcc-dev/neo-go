package flags

import (
	"flag"
	"io"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestParseAddress(t *testing.T) {
	expected := random.Uint160()
	t.Run("simple LE", func(t *testing.T) {
		u, err := ParseAddress(expected.StringLE())
		require.NoError(t, err)
		require.Equal(t, expected, u)
	})
	t.Run("with prefix", func(t *testing.T) {
		u, err := ParseAddress("0x" + expected.StringLE())
		require.NoError(t, err)
		require.Equal(t, expected, u)

		t.Run("bad", func(t *testing.T) {
			_, err := ParseAddress("0s" + expected.StringLE())
			require.Error(t, err)
		})
	})
	t.Run("address", func(t *testing.T) {
		addr := address.Uint160ToString(expected)
		u, err := ParseAddress(addr)
		require.NoError(t, err)
		require.Equal(t, expected, u)

		t.Run("bad", func(t *testing.T) {
			_, err := ParseAddress(addr[1:])
			require.Error(t, err)
		})
	})
}

func TestAddress_String(t *testing.T) {
	value := util.Uint160{1, 2, 3}
	addr := Address{
		IsSet: true,
		Value: value,
	}

	require.Equal(t, address.Uint160ToString(value), addr.String())
}

func TestAddress_Set(t *testing.T) {
	value := util.Uint160{1, 2, 3}
	addr := Address{}

	t.Run("bad address", func(t *testing.T) {
		require.Error(t, addr.Set("not an address"))
	})

	t.Run("positive", func(t *testing.T) {
		require.NoError(t, addr.Set(address.Uint160ToString(value)))
		require.Equal(t, true, addr.IsSet)
		require.Equal(t, value, addr.Value)
	})
}

func TestAddress_Uint160(t *testing.T) {
	value := util.Uint160{4, 5, 6}
	addr := Address{}

	t.Run("not set", func(t *testing.T) {
		require.Panics(t, func() { addr.Uint160() })
	})

	t.Run("success", func(t *testing.T) {
		addr.IsSet = true
		addr.Value = value
		require.Equal(t, value, addr.Uint160())
	})
}

func TestAddressFlag_IsSet(t *testing.T) {
	flag := AddressFlag{}

	t.Run("not set", func(t *testing.T) {
		require.False(t, flag.IsSet())
	})

	t.Run("set", func(t *testing.T) {
		flag.Value.IsSet = true
		require.True(t, flag.IsSet())
	})
}

func TestAddressFlag_String(t *testing.T) {
	flag := AddressFlag{
		Name:  "myFlag",
		Usage: "Address to pass",
		Value: Address{},
	}

	require.Equal(t, "--myFlag value\tAddress to pass", flag.String())
}

func TestAddress_getNameHelp(t *testing.T) {
	require.Equal(t, "-f value", getNameHelp("f"))
	require.Equal(t, "--flag value", getNameHelp("flag"))
}

func TestAddressFlag_Names(t *testing.T) {
	flag := AddressFlag{
		Name:    "flag",
		Aliases: []string{"my"},
	}

	require.Equal(t, []string{"flag", "my"}, flag.Names())
}

func TestAddress(t *testing.T) {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.SetOutput(io.Discard) // don't pollute test output
	addr := AddressFlag{Name: "addr", Aliases: []string{"a"}}
	err := addr.Apply(f)
	require.NoError(t, err)
	require.NoError(t, f.Parse([]string{"--addr", "NRHkiY2hLy5ypD32CKZtL6pNwhbFMqDEhR"}))
	require.Equal(t, "NRHkiY2hLy5ypD32CKZtL6pNwhbFMqDEhR", f.Lookup("a").Value.String())
	require.NoError(t, f.Parse([]string{"-a", "NRHkiY2hLy5ypD32CKZtL6pNwhbFMqDEhR"}))
	require.Equal(t, "NRHkiY2hLy5ypD32CKZtL6pNwhbFMqDEhR", f.Lookup("a").Value.String())
	require.Error(t, f.Parse([]string{"--addr", "kek"}))
}

func TestAddressFlag_IsRequired(t *testing.T) {
	flag := AddressFlag{Required: true}
	require.True(t, flag.IsRequired())

	flag.Required = false
	require.False(t, flag.IsRequired())
}

func TestAddressFlag_IsVisible(t *testing.T) {
	flag := AddressFlag{Hidden: false}
	require.True(t, flag.IsVisible())

	flag.Hidden = true
	require.False(t, flag.IsVisible())
}

func TestAddressFlag_TakesValue(t *testing.T) {
	flag := AddressFlag{}
	require.True(t, flag.TakesValue())
}

func TestAddressFlag_GetUsage(t *testing.T) {
	flag := AddressFlag{Usage: "Specify the address"}
	require.Equal(t, "Specify the address", flag.GetUsage())
}

func TestAddressFlag_GetValue(t *testing.T) {
	addrValue := util.Uint160{1, 2, 3}
	flag := AddressFlag{Value: Address{IsSet: true, Value: addrValue}}
	expectedStr := address.Uint160ToString(addrValue)
	require.Equal(t, expectedStr, flag.GetValue())
}

func TestAddressFlag_Get(t *testing.T) {
	app := cli.NewApp()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(app, set, nil)

	flag := AddressFlag{
		Name:  "testAddress",
		Value: Address{Value: util.Uint160{1, 2, 3}, IsSet: false},
	}

	set.Var(&flag.Value, "testAddress", "test usage")
	require.NoError(t, set.Set("testAddress", address.Uint160ToString(util.Uint160{3, 2, 1})))

	expected := flag.Get(ctx)
	require.True(t, expected.IsSet)
	require.Equal(t, util.Uint160{3, 2, 1}, expected.Value)
}

func TestAddressFlag_RunAction(t *testing.T) {
	called := false
	action := func(ctx *cli.Context, s string) error {
		called = true
		require.Equal(t, address.Uint160ToString(util.Uint160{1, 2, 3}), s)
		return nil
	}

	app := cli.NewApp()
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	ctx := cli.NewContext(app, set, nil)

	flag := AddressFlag{
		Action: action,
		Value:  Address{IsSet: true, Value: util.Uint160{4, 5, 6}},
	}

	expected := address.Uint160ToString(util.Uint160{1, 2, 3})
	set.Var(&flag.Value, "testAddress", "test usage")
	require.NoError(t, set.Set("testAddress", expected))
	require.Equal(t, expected, flag.GetValue())

	err := flag.RunAction(ctx)
	require.NoError(t, err)
	require.True(t, called)
}
