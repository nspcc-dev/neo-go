package integration

import (
	"encoding/hex"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
)

// Main steps for testing are:
// - prepare docker image with neo-go node or c# node
// - start privatenet
// - start docker nodeContainerReq
// - create RPC client
// - generate input for RPC client
// - start sending txes to the node
// - measure how much TX could be sent

const (
	// RPCPort for RPC calls to test nodes.
	RPCPort = nat.Port("20331")
	// NumberOfTx number of transactions to be generated.
	NumberOfTx = 3000
	// TimeoutInSeconds time in seconds to wait for test to finish.
	TimeoutInSeconds = 500
)

// SendTXResponse struct for testing.
type SendTXResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  bool   `json:"result"`
	ID      int    `json:"id"`
}

// GetBlockCountResponse struct for testing.
type GetBlockCountResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  int    `json:"result"`
	ID      int    `json:"id"`
}

// GetBlockResponse struct for testing.
type GetBlockResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  string `json:"result"`
	ID      int    `json:"id"`
}

func TestIntegration(t *testing.T) {
	// Comment t.Skip() to run this test. It's disabled so in CI it;s not running every time.
	t.Skip()
	callNode(t, RPCPort)
}

// makes a new RPC client and calls node by RPC.
func callNode(t *testing.T, port nat.Port) {
	client := NewRPCClient(port)
	data := getTransactions(t, NumberOfTx)
	startTime := uint32(time.Now().UTC().Unix())
	// -1 since getBlockCount returns block count including first zero block
	startBlockIndex := getBlockCount(t, client) - 1
	log.Printf("Started test from block = %v at unix time = %v", startBlockIndex, startTime)
	for _, tx := range data {
		body := client.SendTX(tx, t)
		var resp SendTXResponse
		err := json.Unmarshal(body, &resp)
		require.NoError(t, err)
	}
	log.Println("All transactions were sent")

	lastBlockTimestamp := startTime
	searchAllTX(t, client, startBlockIndex, data, &lastBlockTimestamp)

	log.Printf("Sent %v transactions in %v seconds", NumberOfTx, lastBlockTimestamp-startTime)
}

// getBlockCount returns current block index.
func getBlockCount(t *testing.T, client *RPCClient) int {
	bodyBlockCount := client.GetBlockCount(t)
	var respBlockCount GetBlockCountResponse
	err := json.Unmarshal(bodyBlockCount, &respBlockCount)
	require.NoError(t, err)
	return respBlockCount.Result
}

// getBlock returns block by index.
func getBlock(t *testing.T, client *RPCClient, index int) *core.Block {
	bodyBlock := client.GetBlock(t, index)
	var respBlock GetBlockResponse
	err := json.Unmarshal(bodyBlock, &respBlock)
	require.NoError(t, err)
	decodedResp, _ := hex.DecodeString(respBlock.Result)
	block := &core.Block{}
	newReader := io.NewBinReaderFromBuf(decodedResp)
	block.DecodeBinary(newReader)
	return block
}

// searchAllTX performs search for all TX which were generated.
// For searching used Hash of the transactions.
func searchAllTX(t *testing.T, client *RPCClient, startBlockIndex int,
	txToSearch []*transaction.Transaction, lastBlockTimestamp *uint32) {

	allTX := txToSearch
	timeout := time.After(TimeoutInSeconds * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	counter := 0
	passedBlocks := make(map[int]bool)
	for {
		select {
		case <-timeout:
			log.Fatal("timed out")
			return
		case tick := <-ticker.C:
			log.Println("Tick at", tick)
			log.Printf("Left to check: %v", len(allTX) - counter)
			currentBlock := getBlockCount(t, client) - 1
			if currentBlock > startBlockIndex && !passedBlocks[currentBlock] {
				passedBlocks[currentBlock] = false
				log.Printf("Current block height: %v", currentBlock)
				block := getBlock(t, client, currentBlock)
				*lastBlockTimestamp = block.Timestamp
				for _, tx := range block.Transactions {
					for _, txToSearch := range allTX {
						if len(tx.Scripts) > 0 {
							if tx.Hash() == txToSearch.Hash() {
								counter ++
							}
						}
					}
				}
				passedBlocks[currentBlock] = true
				if len(allTX) == counter {
					ticker.Stop()
					return
				}
			}
		}
	}
}
