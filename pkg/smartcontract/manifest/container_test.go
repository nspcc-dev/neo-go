package manifest

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
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

	t.Run("PermissionDesc", func(t *testing.T) {
		check := func(t *testing.T, u PermissionDesc) {
			c := new(WildPermissionDescs)
			require.False(t, c.IsWildcard())
			require.False(t, c.Contains(u))
			c.Wildcard = true
			require.True(t, c.IsWildcard())
			require.True(t, c.Contains(u))
			c.Restrict()
			require.False(t, c.IsWildcard())
			require.False(t, c.Contains(u))
			require.Equal(t, 0, len(c.Value))
		}
		t.Run("Hash", func(t *testing.T) {
			check(t, PermissionDesc{
				Type:  PermissionHash,
				Value: random.Uint160(),
			})
		})
		t.Run("Group", func(t *testing.T) {
			pk, err := keys.NewPrivateKey()
			require.NoError(t, err)
			check(t, PermissionDesc{
				Type:  PermissionGroup,
				Value: pk.PublicKey(),
			})
		})
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
		c := new(WildPermissionDescs)
		require.Equal(t, []PermissionDesc(nil), c.Value)
		pk, err := keys.NewPrivateKey()
		require.NoError(t, err)
		exp := []PermissionDesc{
			{Type: PermissionHash, Value: random.Uint160()},
			{Type: PermissionGroup, Value: pk.PublicKey()},
		}
		for i := range exp {
			c.Add(exp[i])
		}
		for i := range exp {
			require.True(t, c.Contains(exp[i]))
		}
		pkRand, err := keys.NewPrivateKey()
		require.NoError(t, err)
		require.False(t, c.Contains(PermissionDesc{Type: PermissionHash, Value: random.Uint160()}))
		require.False(t, c.Contains(PermissionDesc{Type: PermissionGroup, Value: pkRand.PublicKey()}))
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

	t.Run("PermissionDesc", func(t *testing.T) {
		t.Run("wildcard", func(t *testing.T) {
			expected := new(WildPermissionDescs)
			testserdes.MarshalUnmarshalJSON(t, expected, new(WildPermissionDescs))
		})

		t.Run("empty", func(t *testing.T) {
			expected := new(WildPermissionDescs)
			expected.Restrict()
			testserdes.MarshalUnmarshalJSON(t, expected, new(WildPermissionDescs))
		})

		t.Run("non-empty", func(t *testing.T) {
			expected := new(WildPermissionDescs)
			expected.Add(PermissionDesc{
				Type:  PermissionHash,
				Value: random.Uint160(),
			})
			testserdes.MarshalUnmarshalJSON(t, expected, new(WildPermissionDescs))
		})

		t.Run("invalid", func(t *testing.T) {
			js := []byte(`["notahex"]`)
			c := new(WildPermissionDescs)
			require.Error(t, json.Unmarshal(js, c))
		})
	})
}
