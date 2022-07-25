package result

import (
	"encoding/json"
	"testing"

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
        "network": 860833102,
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
            "validatorscount": 7
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
            "validatorscount": 7
        },
        "tcpport": 10333,
        "useragent": "/Neo:3.1.0/",
        "wsport": 10334
    }`
	v := &Version{
		Magic:     860833102,
		TCPPort:   10333,
		WSPort:    10334,
		Nonce:     1677922561,
		UserAgent: "/NEO-GO:0.98.6/",
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
		},
		StateRootInHeader: false,
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
				require.NoError(t, json.Unmarshal([]byte(responseFromGoOld), actual))
				expected := new(Version)
				*expected = *v
				expected.UserAgent = "/NEO-GO:0.98.2/"
				require.Equal(t, expected, actual)
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
			expected.Magic = 0 // No magic in C#.
			require.Equal(t, expected, actual)
		})
	})
}

func TestVersionFromUserAgent(t *testing.T) {
	type testCase struct {
		success         bool
		cmpWithBreaking int
	}
	var testcases = map[string]testCase{
		"/Neo:3.1.0/":               {success: false},
		"/NEO-GO:0.98.7":            {success: true, cmpWithBreaking: 1},
		"/NEO-GO:0.98.7-pre-12344/": {success: true, cmpWithBreaking: 1},
		"/NEO-GO:0.98.6/":           {success: true, cmpWithBreaking: 1},
		"/NEO-GO:0.98.6-pre-123/":   {success: true, cmpWithBreaking: 1},
		"/NEO-GO:0.98.5/":           {success: true, cmpWithBreaking: 0},
		"/NEO-GO:0.98.5-pre-12345/": {success: true, cmpWithBreaking: -1},
		"/NEO-GO:123456":            {success: false},
	}
	for str, tc := range testcases {
		ver, err := userAgentToVersion(str)
		if tc.success {
			require.NoError(t, err)
			require.Equal(t, ver.Compare(latestNonBreakingVersion), tc.cmpWithBreaking, str)
		} else {
			require.Error(t, err)
		}
	}
}
