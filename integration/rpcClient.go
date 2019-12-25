package integration

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
)

// RPCClient used in integration test.
type RPCClient struct {
	addr string
	port nat.Port
}

// NewRPCClient creates new client for RPC communications.
func NewRPCClient(port nat.Port) *RPCClient {
	addr := "http://127.0.0.1" + ":" + port.Port()
	return &RPCClient{port: port, addr: addr}
}

// SendTX sends transaction.
func (c *RPCClient) SendTX(tx *transaction.Transaction, t *testing.T) []byte {
	writer := io.NewBufBinWriter()
	tx.EncodeBinary(writer.BinWriter)
	rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "sendrawtransaction", "params": ["%s"]}`, hex.EncodeToString(writer.Bytes()))
	return c.doRPCCall(rpc, t)
}

// GetBlock sends getblock RPC request.
func (c *RPCClient) GetBlock(t *testing.T, index int) []byte {
	rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getblock", "params": [%v]}`, index)
	return c.doRPCCall(rpc, t)
}

// GetBlockCount send getblockcount RPC request.
func (c *RPCClient) GetBlockCount(t *testing.T) []byte {
	rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getblockcount", "params": []}`)
	return c.doRPCCall(rpc, t)
}

func (c *RPCClient) doRPCCall(rpcCall string, t *testing.T) []byte {
	req, err := http.NewRequest("POST", c.addr, strings.NewReader(rpcCall))
	if err != nil {
		log.Fatalf("can't create request %s", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("error after calling rpc server %s", err)
		return nil
	}
	if resp != nil {
		body, err := ioutil.ReadAll(resp.Body)
		assert.NoErrorf(t, err, "could not read response from the request: %s", rpcCall)
		return bytes.TrimSpace(body)
	}
	return nil
}
