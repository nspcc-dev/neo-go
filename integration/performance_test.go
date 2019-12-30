package integration

import (
	"math/rand"
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/encoding/address"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/rpc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Benchmark test to measure number of processed TX.
// Same benchmark made on reference C# node https://github.com/neo-project/neo/issues/1321.
func BenchmarkTXPerformanceTest(t *testing.B) {
	net := config.ModeUnitTestNet
	configPath := "../config"
	cfg, err := config.Load(configPath, net)
	require.NoError(t, err, "could not load config")

	logger := zaptest.NewLogger(t)
	memoryStore := storage.NewMemoryStore()
	chain, err := core.NewBlockchain(memoryStore, cfg.ProtocolConfiguration, logger)
	require.NoError(t, err, "could not create chain")

	go chain.Run()

	serverConfig := network.NewServerConfig(cfg)
	server := network.NewServer(serverConfig, chain, logger)
	data := prepareData(t)
	t.ResetTimer()

	for n := 0; n < t.N; n++ {
		if server.RelayTxn(data[n]) != network.RelaySucceed {
			t.Fail()
		}
		if server.RelayTxn(data[n]) != network.RelayAlreadyExists {
			t.Fail()
		}
	}
	chain.Close()
}

func prepareData(t *testing.B) []*transaction.Transaction {
	var data []*transaction.Transaction

	wif := getWif(t)

	for n := 0; n < t.N; n++ {
		tx := getTX(t, wif)
		require.NoError(t, rpc.SignTx(tx, wif))
		data = append(data, tx)
	}
	return data
}

// getWif returns Wif.
func getWif(t *testing.B) *keys.WIF {
	var (
		wifEncoded = "KxhEDBQyyEFymvfJD96q8stMbJMbZUb6D1PmXqBWZDU2WvbvVs9o"
		version    = byte(0x00)
	)
	wif, err := keys.WIFDecode(wifEncoded, version)
	require.NoError(t, err)
	return wif
}

// getTX returns Invocation transaction with some random attributes in order to have different hashes.
func getTX(t *testing.B, wif *keys.WIF) *transaction.Transaction {
	fromAddress := wif.PrivateKey.Address()
	fromAddressHash, err := address.StringToUint160(fromAddress)
	require.NoError(t, err)

	tx := &transaction.Transaction{
		Type:    transaction.InvocationType,
		Version: 0,
		Data: &transaction.InvocationTX{
			Script:  []byte{0x51},
			Gas:     1,
			Version: 1,
		},
		Attributes: []transaction.Attribute{},
		Inputs:     []transaction.Input{},
		Outputs:    []transaction.Output{},
		Scripts:    []transaction.Witness{},
		Trimmed:    false,
	}
	tx.Attributes = append(tx.Attributes,
		transaction.Attribute{
			Usage: transaction.Description,
			Data:  []byte(randString(10)),
		})
	tx.Attributes = append(tx.Attributes,
		transaction.Attribute{
			Usage: transaction.Script,
			Data:  fromAddressHash.BytesBE(),
		})
	return tx
}

// String returns a random string with the n as its length.
func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(Int(65, 90))
	}

	return string(b)
}

// Int returns a random integer in [min,max).
func Int(min, max int) int {
	return min + rand.Intn(max-min)
}
