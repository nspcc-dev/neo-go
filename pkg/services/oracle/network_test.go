package oracle

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsReserved(t *testing.T) {
	require.True(t, isReserved(net.IPv4zero))
	require.True(t, isReserved(net.IPv4(10, 0, 0, 1)))
	require.True(t, isReserved(net.IPv4(192, 168, 0, 1)))
	require.True(t, isReserved(net.IPv6interfacelocalallnodes))
	require.True(t, isReserved(net.IPv6loopback))

	require.False(t, isReserved(net.IPv4(8, 8, 8, 8)))
}
