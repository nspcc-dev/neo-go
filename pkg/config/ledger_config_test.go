package config

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestHashIndex_MarshalUnmarshalYAML(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		testserdes.MarshalUnmarshalYAML(t, &HashIndex{
			Hash:  util.Uint256{1, 2, 3},
			Index: 1,
		}, new(HashIndex))
	})
	t.Run("empty", func(t *testing.T) {
		testserdes.MarshalUnmarshalYAML(t, &HashIndex{}, new(HashIndex))
	})
	t.Run("multiple heights", func(t *testing.T) {
		require.ErrorContains(t, yaml.Unmarshal([]byte(`
1: `+util.Uint256{1, 2, 3}.String()+` 
2: `+util.Uint256{1, 2, 3}.String()), new(HashIndex)), "only one trusted height is supported")
	})
}
