package interopnames

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromID(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		id := ToID([]byte(names[0]))
		name, err := FromID(id)
		require.NoError(t, err)
		require.Equal(t, names[0], name)
	})
	t.Run("Invalid", func(t *testing.T) {
		_, err := FromID(0x42424242)
		require.ErrorIs(t, err, errNotFound)
	})
}
