package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplicationConfigurationEquals(t *testing.T) {
	a := &ApplicationConfiguration{}
	o := &ApplicationConfiguration{}
	require.True(t, a.EqualsButServices(o))
	require.True(t, o.EqualsButServices(a))
	require.True(t, a.EqualsButServices(a))

	cfg1, err := LoadFile(filepath.Join("..", "..", "config", "protocol.mainnet.yml"))
	require.NoError(t, err)
	cfg2, err := LoadFile(filepath.Join("..", "..", "config", "protocol.testnet.yml"))
	require.NoError(t, err)
	require.False(t, cfg1.ApplicationConfiguration.EqualsButServices(&cfg2.ApplicationConfiguration))
}

func TestGetAddresses(t *testing.T) {
	type testcase struct {
		cfg        *ApplicationConfiguration
		expected   []AnnounceableAddress
		shouldFail bool
	}
	addr1 := "1.2.3.4"
	addr2 := "5.6.7.8"
	addr3 := "4.3.2.1"
	v6Plain0 := "3731:54:65fe:2::a7"
	v6Plain1 := "3731:54:65fe:2::1"
	v6Plain2 := "3731:54:65fe::a:1"
	v6Plain3 := "3731:54:65fe::1:1"
	v6Plain4 := "3731:54:65fe:2::"
	port1 := uint16(1)
	port2 := uint16(2)
	port3 := uint16(3)
	cases := []testcase{
		// Compatibility with the old behaviour.
		{
			cfg:      &ApplicationConfiguration{},
			expected: []AnnounceableAddress{{Address: ":0"}},
		},
		{
			cfg:      &ApplicationConfiguration{Address: &addr1},
			expected: []AnnounceableAddress{{Address: addr1 + ":0"}},
		},
		{
			cfg:      &ApplicationConfiguration{NodePort: &port1},
			expected: []AnnounceableAddress{{Address: ":1"}},
		},
		{
			cfg:      &ApplicationConfiguration{AnnouncedNodePort: &port1},
			expected: []AnnounceableAddress{{Address: ":0", AnnouncedPort: port1}},
		},
		{
			cfg:      &ApplicationConfiguration{Address: &addr1, NodePort: &port1},
			expected: []AnnounceableAddress{{Address: addr1 + ":1"}},
		},
		{
			cfg:      &ApplicationConfiguration{Address: &addr1, AnnouncedNodePort: &port1},
			expected: []AnnounceableAddress{{Address: addr1 + ":0", AnnouncedPort: port1}},
		},
		{
			cfg:      &ApplicationConfiguration{NodePort: &port1, AnnouncedNodePort: &port2},
			expected: []AnnounceableAddress{{Address: ":1", AnnouncedPort: port2}},
		},
		{
			cfg:      &ApplicationConfiguration{NodePort: &port1, AnnouncedNodePort: &port2},
			expected: []AnnounceableAddress{{Address: ":1", AnnouncedPort: port2}},
		},
		{
			cfg:      &ApplicationConfiguration{Address: &addr1, NodePort: &port1, AnnouncedNodePort: &port2},
			expected: []AnnounceableAddress{{Address: addr1 + ":1", AnnouncedPort: port2}},
		},
		// Compatibility with new multi-addresses.
		{
			cfg: &ApplicationConfiguration{
				Address: &addr1, NodePort: &port1, AnnouncedNodePort: &port2,
				Addresses: []string{addr1, addr2 + ":3", addr3 + ":1:3"},
			},
			expected: []AnnounceableAddress{
				{Address: addr1 + ":1", AnnouncedPort: port2},
				{Address: addr1},
				{Address: addr2 + ":3"},
				{Address: addr3 + ":1", AnnouncedPort: port3},
			},
		},
		// Multi-addresses checks.
		{
			cfg: &ApplicationConfiguration{
				Addresses: []string{addr1},
			},
			expected: []AnnounceableAddress{
				{Address: addr1},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				Addresses: []string{":1"},
			},
			expected: []AnnounceableAddress{
				{Address: ":1"},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				Addresses: []string{"::1"},
			},
			expected: []AnnounceableAddress{
				{Address: ":", AnnouncedPort: port1},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				Addresses: []string{addr1 + ":1"},
			},
			expected: []AnnounceableAddress{
				{Address: addr1 + ":1"},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				Addresses: []string{addr1 + "::1"},
			},
			expected: []AnnounceableAddress{
				{Address: addr1 + ":", AnnouncedPort: port1},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				Addresses: []string{addr1 + ":1:2"},
			},
			expected: []AnnounceableAddress{
				{Address: addr1 + ":1", AnnouncedPort: port2},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				Addresses: []string{addr1 + ":1", addr2 + "::2"},
			},
			expected: []AnnounceableAddress{
				{Address: addr1 + ":1"},
				{Address: addr2 + ":", AnnouncedPort: port2},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				Addresses: []string{v6Plain0, v6Plain1, v6Plain2, v6Plain3, v6Plain4},
			},
			expected: []AnnounceableAddress{
				{Address: v6Plain0},
				{Address: v6Plain1},
				{Address: v6Plain2},
				{Address: v6Plain3},
				{Address: v6Plain4},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				Addresses: []string{"[3731:54:65fe:2::]:123", "[3731:54:65fe:2::]:123:124"},
			},
			expected: []AnnounceableAddress{
				{Address: "[3731:54:65fe:2::]:123"},
				{Address: "[3731:54:65fe:2::]:123", AnnouncedPort: 124},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				Addresses: []string{"127.0.0.1:QWER:123"},
			},
			shouldFail: true,
		},
	}
	for i, c := range cases {
		actual, err := c.cfg.GetAddresses()
		if c.shouldFail {
			require.Error(t, err, i)
		} else {
			require.NoError(t, err)
			require.Equal(t, c.expected, actual, i)
		}
	}
}
