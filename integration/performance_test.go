package integration

import (
	"math/rand"
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/stretchr/testify/require"
)

// Benchmark test to measure number of processed TX.
// Same benchmark made on reference C# node https://github.com/neo-project/neo/issues/1321.
func BenchmarkTXPerformanceTest(t *testing.B) {
	net := config.ModeUnitTestNet
	configPath := "../config"
	cfg, err := config.Load(configPath, net)
	require.NoError(t, err, "could not load config")

	memoryStore := storage.NewMemoryStore()
	chain, err := core.NewBlockchain(memoryStore, cfg.ProtocolConfiguration)
	require.NoError(t, err, "could not create chain")

	go chain.Run()

	serverConfig := network.NewServerConfig(cfg)
	server := network.NewServer(serverConfig, chain)
	for n := 0; n < t.N; n++ {
		tx := getTX()
		require.Equal(t, server.RelayTxn(tx), network.RelaySucceed)
		require.Equal(t, server.RelayTxn(tx), network.RelayAlreadyExists)
	}
	chain.Close()
}

// getTX returns Invocation transaction with some random attributes in order to have different hashes.
func getTX() *transaction.Transaction {
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
