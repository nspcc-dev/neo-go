package oracle

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
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

func TestDefaultClient_RestrictedRedirectErr(t *testing.T) {
	cfg := config.OracleConfiguration{
		AllowPrivateHost: false,
		RequestTimeout:   time.Second,
	}
	cl := getDefaultClient(cfg)

	testCases := []string{
		"http://localhost:8080",
		"http://localhost",
		"https://localhost:443",
		"https://" + net.IPv4zero.String(),
		"https://" + net.IPv4(10, 0, 0, 1).String(),
		"https://" + net.IPv4(192, 168, 0, 1).String(),
		"https://[" + net.IPv6interfacelocalallnodes.String() + "]",
		"https://[" + net.IPv6loopback.String() + "]",
	}
	for _, c := range testCases {
		t.Run(c, func(t *testing.T) {
			_, err := cl.Get(c)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrRestrictedRedirect), err)
			require.True(t, strings.Contains(err.Error(), "IP is not global unicast"), err)
		})
	}
}
