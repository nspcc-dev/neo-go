package rpc

import (
	"net/http"
	"os"
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/block"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/rpc/response/result"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
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

// InvokeFunctionResponse struct for testing.
type InvokeFunctionResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Script      string      `json:"script"`
		State       string      `json:"state"`
		GasConsumed string      `json:"gas_consumed"`
		Stack       []FuncParam `json:"stack"`
		TX          string      `json:"tx,omitempty"`
	} `json:"result"`
	ID int `json:"id"`
}

// ValidateAddrResponse struct for testing.
type ValidateAddrResponse struct {
	Jsonrpc string                 `json:"jsonrpc"`
	Result  result.ValidateAddress `json:"result"`
	ID      int                    `json:"id"`
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
	Jsonrpc string       `json:"jsonrpc"`
	Result  result.Block `json:"result"`
	ID      int          `json:"id"`
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

// GetUnspents struct for testing.
type GetUnspents struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Balance []struct {
			Unspents []struct {
				TxID  string `json:"txid"`
				Index int    `json:"n"`
				Value string `json:"value"`
			} `json:"unspent"`
			AssetHash   string `json:"asset_hash"`
			Asset       string `json:"asset"`
			AssetSymbol string `json:"asset_symbol"`
			Amount      string `json:"amount"`
		} `json:"balance"`
		Address string `json:"address"`
	} `json:"result"`
	ID int `json:"id"`
}

// GetContractStateResponse struct for testing.
type GetContractStateResponce struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Version     byte         `json:"version"`
		ScriptHash  util.Uint160 `json:"hash"`
		Script      []byte       `json:"script"`
		ParamList   interface{}  `json:"parameters"`
		ReturnType  interface{}  `json:"returntype"`
		Name        string       `json:"name"`
		CodeVersion string       `json:"code_version"`
		Author      string       `json:"author"`
		Email       string       `json:"email"`
		Description string       `json:"description"`
		Properties  struct {
			HasStorage       bool `json:"storage"`
			HasDynamicInvoke bool `json:"dynamic_invoke"`
			IsPayable        bool `json:"is_payable"`
		} `json:"properties"`
	} `json:"result"`
	ID int `json:"id"`
}

func initServerWithInMemoryChain(t *testing.T) (*core.Blockchain, http.HandlerFunc) {
	var nBlocks uint32

	net := config.ModeUnitTestNet
	configPath := "../../config"
	cfg, err := config.Load(configPath, net)
	require.NoError(t, err, "could not load config")

	memoryStore := storage.NewMemoryStore()
	logger := zaptest.NewLogger(t)
	chain, err := core.NewBlockchain(memoryStore, cfg.ProtocolConfiguration, logger)
	require.NoError(t, err, "could not create chain")

	go chain.Run()

	// File "./testdata/testblocks.acc" was generated by function core._
	// ("neo-go/pkg/core/helper_test.go").
	// To generate new "./testdata/testblocks.acc", follow the steps:
	// 		1. Rename the function
	// 		2. Add specific test-case into "neo-go/pkg/core/blockchain_test.go"
	// 		3. Run tests with `$ make test`
	f, err := os.Open("testdata/testblocks.acc")
	require.Nil(t, err)
	br := io.NewBinReaderFromIO(f)
	nBlocks = br.ReadU32LE()
	require.Nil(t, br.Err)
	for i := 0; i < int(nBlocks); i++ {
		b := &block.Block{}
		b.DecodeBinary(br)
		require.Nil(t, br.Err)
		require.NoError(t, chain.AddBlock(b))
	}

	serverConfig := network.NewServerConfig(cfg)
	server, err := network.NewServer(serverConfig, chain, logger)
	require.NoError(t, err)
	rpcServer := NewServer(chain, cfg.ApplicationConfiguration.RPC, server, logger)
	handler := http.HandlerFunc(rpcServer.requestHandler)

	return chain, handler
}
