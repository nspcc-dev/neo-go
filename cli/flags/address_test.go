package flags

import (
	"flag"
	"io/ioutil"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
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

func TestAddressFlag_GetName(t *testing.T) {
	flag := AddressFlag{
		Name: "my flag",
	}

	require.Equal(t, "my flag", flag.GetName())
}

func TestAddress(t *testing.T) {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.SetOutput(ioutil.Discard) // don't pollute test output
	addr := AddressFlag{Name: "addr, a"}
	addr.Apply(f)
	require.NoError(t, f.Parse([]string{"--addr", "NRHkiY2hLy5ypD32CKZtL6pNwhbFMqDEhR"}))
	require.Equal(t, "NRHkiY2hLy5ypD32CKZtL6pNwhbFMqDEhR", f.Lookup("a").Value.String())
	require.NoError(t, f.Parse([]string{"-a", "NRHkiY2hLy5ypD32CKZtL6pNwhbFMqDEhR"}))
	require.Equal(t, "NRHkiY2hLy5ypD32CKZtL6pNwhbFMqDEhR", f.Lookup("a").Value.String())
	require.Error(t, f.Parse([]string{"--addr", "kek"}))
}
