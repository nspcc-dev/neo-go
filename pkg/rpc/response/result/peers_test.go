package result

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPeers(t *testing.T) {
	gp := NewGetPeers()
	require.Equal(t, 0, len(gp.Unconnected))
	require.Equal(t, 0, len(gp.Connected))
	require.Equal(t, 0, len(gp.Bad))

	gp.AddUnconnected([]string{"1.1.1.1:53", "8.8.8.8:53", "9.9.9.9:53"})
	gp.AddConnected([]string{"192.168.0.1:10333"})
	gp.AddBad([]string{"127.0.0.1:20333"})

	require.Equal(t, 3, len(gp.Unconnected))
	require.Equal(t, 1, len(gp.Connected))
	require.Equal(t, 1, len(gp.Bad))
	require.Equal(t, "192.168.0.1", gp.Connected[0].Address)
	require.Equal(t, "10333", gp.Connected[0].Port)
	require.Equal(t, "127.0.0.1", gp.Bad[0].Address)
	require.Equal(t, "20333", gp.Bad[0].Port)
}
