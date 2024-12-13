package config

import (
	"path/filepath"
	"testing"
	"time"

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
	v6Plain0 := "3731:54:65fe:2::a7"
	v6Plain1 := "3731:54:65fe:2::1"
	v6Plain2 := "3731:54:65fe::a:1"
	v6Plain3 := "3731:54:65fe::1:1"
	v6Plain4 := "3731:54:65fe:2::"
	port1 := uint16(1)
	port2 := uint16(2)
	cases := []testcase{
		// Multi-addresses checks.
		{
			cfg: &ApplicationConfiguration{
				P2P: P2P{Addresses: []string{addr1}},
			},
			expected: []AnnounceableAddress{
				{Address: addr1},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				P2P: P2P{Addresses: []string{":1"}},
			},
			expected: []AnnounceableAddress{
				{Address: ":1"},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				P2P: P2P{Addresses: []string{"::1"}},
			},
			expected: []AnnounceableAddress{
				{Address: ":", AnnouncedPort: port1},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				P2P: P2P{Addresses: []string{addr1 + ":1"}},
			},
			expected: []AnnounceableAddress{
				{Address: addr1 + ":1"},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				P2P: P2P{Addresses: []string{addr1 + "::1"}},
			},
			expected: []AnnounceableAddress{
				{Address: addr1 + ":", AnnouncedPort: port1},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				P2P: P2P{Addresses: []string{addr1 + ":1:2"}},
			},
			expected: []AnnounceableAddress{
				{Address: addr1 + ":1", AnnouncedPort: port2},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				P2P: P2P{Addresses: []string{addr1 + ":1", addr2 + "::2"}},
			},
			expected: []AnnounceableAddress{
				{Address: addr1 + ":1"},
				{Address: addr2 + ":", AnnouncedPort: port2},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				P2P: P2P{Addresses: []string{v6Plain0, v6Plain1, v6Plain2, v6Plain3, v6Plain4}},
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
				P2P: P2P{Addresses: []string{"[3731:54:65fe:2::]:123", "[3731:54:65fe:2::]:123:124"}},
			},
			expected: []AnnounceableAddress{
				{Address: "[3731:54:65fe:2::]:123"},
				{Address: "[3731:54:65fe:2::]:123", AnnouncedPort: 124},
			},
		},
		{
			cfg: &ApplicationConfiguration{
				P2P: P2P{Addresses: []string{"127.0.0.1:QWER:123"}},
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

func TestNeoFSBlockFetcherValidation(t *testing.T) {
	type testcase struct {
		cfg        NeoFSBlockFetcher
		shouldFail bool
		errMsg     string
	}
	validContainerID := "9iVfUg8aDHKjPC4LhQXEkVUM4HDkR7UCXYLs8NQwYfSG"
	invalidContainerID := "invalid-container-id"

	cases := []testcase{
		{
			cfg: NeoFSBlockFetcher{
				InternalService:        InternalService{Enabled: true},
				Timeout:                time.Second,
				ContainerID:            validContainerID,
				Addresses:              []string{"127.0.0.1"},
				OIDBatchSize:           10,
				BQueueSize:             20,
				SkipIndexFilesSearch:   true,
				DownloaderWorkersCount: 4,
			},
			shouldFail: false,
		},
		{
			cfg: NeoFSBlockFetcher{
				InternalService: InternalService{Enabled: true},
				Timeout:         time.Second,
				ContainerID:     "",
				Addresses:       []string{"127.0.0.1"},
				OIDBatchSize:    10,
				BQueueSize:      20,
			},
			shouldFail: true,
			errMsg:     "container ID is not set",
		},
		{
			cfg: NeoFSBlockFetcher{
				InternalService: InternalService{Enabled: true},
				Timeout:         time.Second,
				ContainerID:     invalidContainerID,
				Addresses:       []string{"127.0.0.1"},
				OIDBatchSize:    10,
				BQueueSize:      20,
			},
			shouldFail: true,
			errMsg:     "invalid container ID",
		},
		{
			cfg: NeoFSBlockFetcher{
				InternalService: InternalService{Enabled: true},
				Timeout:         time.Second,
				ContainerID:     validContainerID,
				Addresses:       []string{},
				OIDBatchSize:    10,
				BQueueSize:      20,
			},
			shouldFail: true,
			errMsg:     "addresses are not set",
		},
		{
			cfg: NeoFSBlockFetcher{
				InternalService: InternalService{Enabled: true},
				Timeout:         time.Second,
				ContainerID:     validContainerID,
				Addresses:       []string{"127.0.0.1"},
				OIDBatchSize:    10,
				BQueueSize:      5,
			},
			shouldFail: true,
			errMsg:     "BQueueSize (5) is lower than OIDBatchSize (10)",
		},
	}

	for _, c := range cases {
		err := c.cfg.Validate()
		if c.shouldFail {
			require.Error(t, err)
			require.Contains(t, err.Error(), c.errMsg)
		} else {
			require.NoError(t, err)
		}
	}
}
