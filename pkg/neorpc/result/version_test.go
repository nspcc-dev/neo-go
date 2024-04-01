package result

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/stretchr/testify/require"
)

func TestVersion_MarshalUnmarshalJSON(t *testing.T) {
	responseFromGoOld := `{
        "network": 860833102,
        "nonce": 1677922561,
        "protocol": {
            "addressversion": 53,
            "initialgasdistribution": "52000000",
            "maxtraceableblocks": 2102400,
            "maxtransactionsperblock": 512,
            "maxvaliduntilblockincrement": 5760,
            "memorypoolmaxtransactions": 50000,
            "msperblock": 15000,
            "network": 860833102,
            "validatorscount": 7
        },
        "tcpport": 10333,
        "useragent": "/NEO-GO:0.98.2/",
        "wsport": 10334
    }`
	responseFromGoNew := `{
        "nonce": 1677922561,
        "protocol": {
            "addressversion": 53,
            "initialgasdistribution": 5200000000000000,
            "maxtraceableblocks": 2102400,
            "maxtransactionsperblock": 512,
            "maxvaliduntilblockincrement": 5760,
            "memorypoolmaxtransactions": 50000,
            "msperblock": 15000,
            "network": 860833102,
            "validatorscount": 7,
            "hardforks": [{"name": "Aspidochelone", "blockheight": 123}, {"name": "Basilisk", "blockheight": 1234}]
        },
        "rpc": {
            "maxiteratorresultitems": 100,
            "sessionenabled": true
        },
        "tcpport": 10333,
        "useragent": "/NEO-GO:0.98.6/",
        "wsport": 10334
    }`
	responseFromSharp := `{
        "nonce": 1677922561,
        "protocol": {
            "addressversion": 53,
            "initialgasdistribution": 5200000000000000,
            "maxtraceableblocks": 2102400,
            "maxtransactionsperblock": 512,
            "maxvaliduntilblockincrement": 5760,
            "memorypoolmaxtransactions": 50000,
            "msperblock": 15000,
            "network": 860833102,
            "validatorscount": 7,
            "hardforks": [{"name": "HF_Aspidochelone", "blockheight": 123}, {"name": "HF_Basilisk", "blockheight": 1234}]
        },
        "rpc": {
            "maxiteratorresultitems": 100,
            "sessionenabled": true
        },
        "tcpport": 10333,
        "useragent": "/Neo:3.1.0/",
        "wsport": 10334
    }`
	v := &Version{
		TCPPort:   10333,
		WSPort:    10334,
		Nonce:     1677922561,
		UserAgent: "/NEO-GO:0.98.6/",
		RPC: RPC{
			MaxIteratorResultItems: 100,
			SessionEnabled:         true,
		},
		Protocol: Protocol{
			AddressVersion:              53,
			Network:                     860833102,
			MillisecondsPerBlock:        15000,
			MaxTraceableBlocks:          2102400,
			MaxValidUntilBlockIncrement: 5760,
			MaxTransactionsPerBlock:     512,
			MemoryPoolMaxTransactions:   50000,
			ValidatorsCount:             7,
			// Unmarshalled InitialGasDistribution should always be a valid Fixed8 for both old and new clients.
			InitialGasDistribution: fixedn.Fixed8FromInt64(52000000),
			StateRootInHeader:      false,
			Hardforks:              map[config.Hardfork]uint32{config.HFAspidochelone: 123, config.HFBasilisk: 1234},
		},
	}
	t.Run("MarshalJSON", func(t *testing.T) {
		actual, err := json.Marshal(v)
		require.NoError(t, err)
		require.JSONEq(t, responseFromGoNew, string(actual))
	})
	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Run("Go node response", func(t *testing.T) {
			t.Run("old RPC server", func(t *testing.T) {
				actual := &Version{}
				require.Error(t, json.Unmarshal([]byte(responseFromGoOld), actual))
			})
			t.Run("new RPC server", func(t *testing.T) {
				actual := &Version{}
				require.NoError(t, json.Unmarshal([]byte(responseFromGoNew), actual))
				require.Equal(t, v, actual)
			})
		})
		t.Run("Sharp node response", func(t *testing.T) {
			actual := &Version{}
			require.NoError(t, json.Unmarshal([]byte(responseFromSharp), actual))
			expected := new(Version)
			*expected = *v
			expected.UserAgent = "/Neo:3.1.0/"
			require.Equal(t, expected, actual)
		})
	})
}
