package manifest

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestContainer_Restrict(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		c := new(WildStrings)
		require.True(t, c.IsWildcard())
		require.True(t, c.Contains("abc"))
		c.Restrict()
		require.False(t, c.IsWildcard())
		require.False(t, c.Contains("abc"))
		require.Equal(t, 0, len(c.Value))
	})

	t.Run("uint160", func(t *testing.T) {
		c := new(WildUint160s)
		u := random.Uint160()
		require.True(t, c.IsWildcard())
		require.True(t, c.Contains(u))
		c.Restrict()
		require.False(t, c.IsWildcard())
		require.False(t, c.Contains(u))
		require.Equal(t, 0, len(c.Value))
	})
}

func TestContainer_Add(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		c := new(WildStrings)
		require.Equal(t, []string(nil), c.Value)

		c.Add("abc")
		require.True(t, c.Contains("abc"))
		require.False(t, c.Contains("aaa"))
	})

	t.Run("uint160", func(t *testing.T) {
		c := new(WildUint160s)
		require.Equal(t, []util.Uint160(nil), c.Value)

		exp := []util.Uint160{random.Uint160(), random.Uint160()}
		for i := range exp {
			c.Add(exp[i])
		}
		for i := range exp {
			require.True(t, c.Contains(exp[i]))
		}
		require.False(t, c.Contains(random.Uint160()))
	})
}

func TestContainer_MarshalJSON(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		t.Run("wildcard", func(t *testing.T) {
			expected := new(WildStrings)
			testserdes.MarshalUnmarshalJSON(t, expected, new(WildStrings))
		})

		t.Run("empty", func(t *testing.T) {
			expected := new(WildStrings)
			expected.Restrict()
			testserdes.MarshalUnmarshalJSON(t, expected, new(WildStrings))
		})

		t.Run("non-empty", func(t *testing.T) {
			expected := new(WildStrings)
			expected.Add("string1")
			expected.Add("string2")
			testserdes.MarshalUnmarshalJSON(t, expected, new(WildStrings))
		})

		t.Run("invalid", func(t *testing.T) {
			js := []byte(`[123]`)
			c := new(WildStrings)
			require.Error(t, json.Unmarshal(js, c))
		})
	})

	t.Run("uint160", func(t *testing.T) {
		t.Run("wildcard", func(t *testing.T) {
			expected := new(WildUint160s)
			testserdes.MarshalUnmarshalJSON(t, expected, new(WildUint160s))
		})

		t.Run("empty", func(t *testing.T) {
			expected := new(WildUint160s)
			expected.Restrict()
			testserdes.MarshalUnmarshalJSON(t, expected, new(WildUint160s))
		})

		t.Run("non-empty", func(t *testing.T) {
			expected := new(WildUint160s)
			expected.Add(random.Uint160())
			testserdes.MarshalUnmarshalJSON(t, expected, new(WildUint160s))
		})

		t.Run("invalid", func(t *testing.T) {
			js := []byte(`["notahex"]`)
			c := new(WildUint160s)
			require.Error(t, json.Unmarshal(js, c))
		})
	})
}
