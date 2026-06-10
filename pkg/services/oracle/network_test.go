package oracle

import (
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
	require.True(t, isReserved(net.IPv6zero))
	require.True(t, isReserved(net.ParseIP("::ffff:127.0.0.1"))) // IPv4 mapped into IPv6

	require.False(t, isReserved(net.IPv4(8, 8, 8, 8)))

	// Ref. https://github.com/neo-project/neo-node/pull/1066.
	t.Run("compat", func(t *testing.T) {
		for _, tc := range []struct {
			addr     string
			expected bool
		}{
			// IPv4 Loopback & Special
			{"127.0.0.1", true},
			{"127.255.255.255", true},
			{"0.0.0.0", true},         // IPAddress.Any
			{"255.255.255.255", true}, // IPAddress.Broadcast

			// IPv4 Private Networks
			{"10.0.0.1", true},
			{"10.255.255.255", true},
			{"172.16.0.1", true},
			{"172.31.255.255", true},
			{"192.168.1.1", true},
			{"192.168.255.255", true},

			// IPv4 Link-Local (APIPA) & CGNAT
			{"169.254.0.1", true},
			{"100.64.0.1", true},
			{"100.127.255.255", true},

			// IPv6 Loopback & Special
			{"::1", true},
			{"::", true},      // IPAddress.IPv6Any
			{"fe80::1", true}, // Link-Local
			{"fc00::1", true}, // Unique-Local
			{"fec0::1", true}, // Site-Local

			// IPv4-Mapped IPv6
			{"::ffff:192.168.1.1", true},
			{"::ffff:10.0.0.1", true},
			{"::ffff:8.8.8.8", false}, // Public IP mapped

			// 6to4 Encapsulated (the fixed bug, ref. https://github.com/neo-project/neo-node/pull/1066)
			{"2002:C0A8:0101::", true},  // 6to4 embedding 192.168.1.1
			{"2002:0A00:0001::", true},  // 6to4 embedding 10.0.0.1
			{"2002:AC10:0001::", true},  // 6to4 embedding 172.16.0.1
			{"2002:0808:0808::", false}, // 6to4 embedding 8.8.8.8 (Public)

			// --- Public Addresses (Should be false) ---
			{"8.8.8.8", false},
			{"1.1.1.1", false},
			{"172.15.255.255", false}, // Just outside private Class B
			{"172.32.0.0", false},     // Just outside private Class B
			{"100.63.255.255", false}, // Just outside CGNAT
			{"100.128.0.0", false},    // Just outside CGNAT
			{"2001:db8::", false},     // Documentation address
		} {
			t.Run(tc.addr, func(t *testing.T) {
				require.Equal(t, tc.expected, isReserved(net.ParseIP(tc.addr)))
			})
		}
	})
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
			_, err := cl.Get(c) //nolint:bodyclose // It errors out and it's a test.
			require.ErrorIs(t, err, ErrRestrictedRedirect)
			require.True(t, strings.Contains(err.Error(), "IP is not global unicast"), err)
		})
	}
}
