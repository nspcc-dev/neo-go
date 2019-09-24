package rpc

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/rpc/result"
	"github.com/CityOfZion/neo-go/pkg/rpc/wrappers"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

// ErrorResponse struct represents JSON-RPC error.
type ErrorResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Error   struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	ID int `json:"id"`
}

// SendTXResponse struct for testing.
type SendTXResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  bool   `json:"result"`
	ID      int    `json:"id"`
}

// ValidateAddrResponse struct for testing.
type ValidateAddrResponse struct {
	Jsonrpc string                           `json:"jsonrpc"`
	Result  wrappers.ValidateAddressResponse `json:"result"`
	ID      int                              `json:"id"`
}

// GetPeersResponse struct for testing.
type GetPeersResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Unconnected []int `json:"unconnected"`
		Connected   []int `json:"connected"`
		Bad         []int `json:"bad"`
	} `json:"result"`
	ID int `json:"id"`
}

// GetVersionResponse struct for testing.
type GetVersionResponse struct {
	Jsonrpc string         `json:"jsonrpc"`
	Result  result.Version `json:"result"`
	ID      int            `json:"id"`
}

// IntResultResponse struct for testing.
type IntResultResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  int    `json:"result"`
	ID      int    `json:"id"`
}

// StringResultResponse struct for testing.
type StringResultResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  string `json:"result"`
	ID      int    `json:"id"`
}

// GetBlockResponse struct for testing.
type GetBlockResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Version           int    `json:"version"`
		Previousblockhash string `json:"previousblockhash"`
		Merkleroot        string `json:"merkleroot"`
		Time              int    `json:"time"`
		Height            int    `json:"height"`
		Nonce             int    `json:"nonce"`
		NextConsensus     string `json:"next_consensus"`
		Script            struct {
			Invocation   string `json:"invocation"`
			Verification string `json:"verification"`
		} `json:"script"`
		Tx []struct {
			Type       string      `json:"type"`
			Version    int         `json:"version"`
			Attributes interface{} `json:"attributes"`
			Vin        interface{} `json:"vin"`
			Vout       interface{} `json:"vout"`
			Scripts    interface{} `json:"scripts"`
		} `json:"tx"`
		Confirmations int    `json:"confirmations"`
		Nextblockhash string `json:"nextblockhash"`
		Hash          string `json:"hash"`
	} `json:"result"`
	ID int `json:"id"`
}

// GetAssetResponse struct for testing.
type GetAssetResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		AssetID    string `json:"assetID"`
		AssetType  int    `json:"assetType"`
		Name       string `json:"name"`
		Amount     string `json:"amount"`
		Available  string `json:"available"`
		Precision  int    `json:"precision"`
		Fee        int    `json:"fee"`
		Address    string `json:"address"`
		Owner      string `json:"owner"`
		Admin      string `json:"admin"`
		Issuer     string `json:"issuer"`
		Expiration int    `json:"expiration"`
		IsFrozen   bool   `json:"is_frozen"`
	} `json:"result"`
	ID int `json:"id"`
}

// GetAccountStateResponse struct for testing.
type GetAccountStateResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Version    int           `json:"version"`
		ScriptHash string        `json:"script_hash"`
		Frozen     bool          `json:"frozen"`
		Votes      []interface{} `json:"votes"`
		Balances   []struct {
			Asset string `json:"asset"`
			Value string `json:"value"`
		} `json:"balances"`
	} `json:"result"`
	ID int `json:"id"`
}

func initServerWithInMemoryChain(ctx context.Context, t *testing.T) (*core.Blockchain, http.HandlerFunc) {
	net := config.ModeUnitTestNet
	configPath := "../../config"
	cfg, err := config.Load(configPath, net)
	require.NoError(t, err, "could not load config")

	memoryStore := storage.NewMemoryStore()
	chain, err := core.NewBlockchain(memoryStore, cfg.ProtocolConfiguration)
	require.NoError(t, err, "could not create chain")

	go chain.Run(ctx)
	initBlocks(t, chain)

	serverConfig := network.NewServerConfig(cfg)
	server := network.NewServer(serverConfig, chain)
	rpcServer := NewServer(chain, cfg.ApplicationConfiguration.RPCPort, server)
	handler := http.HandlerFunc(rpcServer.requestHandler)

	return chain, handler
}

func initBlocks(t *testing.T, chain *core.Blockchain) {
	blocks := makeBlocks(10)
	for i := 0; i < len(blocks); i++ {
		require.NoError(t, chain.AddBlock(blocks[i]))
	}
	require.NoError(t, chain.Persist(context.Background()))
}

func makeBlocks(n int) []*core.Block {
	blocks := make([]*core.Block, n)
	for i := 0; i < n; i++ {
		blocks[i] = newBlock(uint32(i+1), newTX(transaction.MinerType))
	}
	return blocks
}

func newTX(t transaction.TXType) *transaction.Transaction {
	return &transaction.Transaction{
		Type: t,
	}
}

func newBlock(index uint32, txs ...*transaction.Transaction) *core.Block {
	b := &core.Block{
		BlockBase: core.BlockBase{
			Version:       0,
			PrevHash:      hash.Sha256([]byte("a")),
			MerkleRoot:    hash.Sha256([]byte("b")),
			Timestamp:     uint32(time.Now().UTC().Unix()),
			Index:         index,
			ConsensusData: 1111,
			NextConsensus: util.Uint160{},
			Script: &transaction.Witness{
				VerificationScript: []byte{0x0},
				InvocationScript:   []byte{0x1},
			},
		},
		Transactions: txs,
	}
	return b
}
