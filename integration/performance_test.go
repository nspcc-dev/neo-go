package integration

import (
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
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
	data := getTransactions(t, t.N)
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
