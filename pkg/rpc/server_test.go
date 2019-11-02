package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRPC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	chain, handler := initServerWithInMemoryChain(ctx, t)

	t.Run("getbestblockhash", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getbestblockhash", "params": []}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res StringResultResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, "0x"+chain.CurrentBlockHash().ReverseString(), res.Result)
	})

	t.Run("getblock", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getblock", "params": [1]}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res GetBlockResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		block, err := chain.GetBlock(chain.GetHeaderHash(1))
		assert.NoErrorf(t, err, "could not get block")
		expectedHash := "0x" + block.Hash().ReverseString()
		assert.Equal(t, expectedHash, res.Result.Hash)
	})

	t.Run("getblockcount", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getblockcount", "params": []}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res IntResultResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, chain.BlockHeight()+1, uint32(res.Result))
	})

	t.Run("getblockhash", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getblockhash", "params": [1]}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res StringResultResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		block, err := chain.GetBlock(chain.GetHeaderHash(1))
		assert.NoErrorf(t, err, "could not get block")
		expectedHash := "0x" + block.Hash().ReverseString()
		assert.Equal(t, expectedHash, res.Result)
	})

	t.Run("getconnectioncount", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getconnectioncount", "params": []}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res IntResultResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, 0, res.Result)
	})

	t.Run("getversion", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getversion", "params": []}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res GetVersionResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, "/NEO-GO:/", res.Result.UserAgent)
	})

	t.Run("getpeers", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getpeers", "params": []}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res GetPeersResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, []int{}, res.Result.Bad)
		assert.Equal(t, []int{}, res.Result.Unconnected)
		assert.Equal(t, []int{}, res.Result.Connected)
	})

	t.Run("validateaddress_positive", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "validateaddress", "params": ["AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i"]}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res ValidateAddrResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, true, res.Result.IsValid)
	})

	t.Run("validateaddress_negative", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "validateaddress", "params": [1]}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res ValidateAddrResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, false, res.Result.IsValid)
	})

	t.Run("getassetstate_positive", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getassetstate", "params": ["602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"]}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res GetAssetResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, "00", res.Result.Owner)
		assert.Equal(t, "AWKECj9RD8rS8RPcpCgYVjk1DeYyHwxZm3", res.Result.Admin)
	})

	t.Run("getassetstate_negative", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getassetstate", "params": ["602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de2"]}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res StringResultResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, "Invalid assetid", res.Result)
	})

	t.Run("getaccountstate_positive", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getaccountstate", "params": ["AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU"]}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res GetAccountStateResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, 1, len(res.Result.Balances))
		assert.Equal(t, false, res.Result.Frozen)
	})

	t.Run("getaccountstate_negative", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getaccountstate", "params": ["AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"]}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res StringResultResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, "Invalid public account address", res.Result)
	})

	t.Run("getrawtransaction", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		TXHash := block.Transactions[1].Hash()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s"]}"`, TXHash.ReverseString())
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res StringResultResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, "400000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000", res.Result)
	})

	t.Run("sendrawtransaction_positive", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "sendrawtransaction", "params": ["d1001b00046e616d6567d3d8602814a429a91afdbaa3914884a1c90c733101201cc9c05cefffe6cdd7b182816a9152ec218d2ec000000141403387ef7940a5764259621e655b3c621a6aafd869a611ad64adcc364d8dd1edf84e00a7f8b11b630a377eaef02791d1c289d711c08b7ad04ff0d6c9caca22cfe6232103cbb45da6072c14761c9da545749d9cfd863f860c351066d16df480602a2024c6ac"]}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res SendTXResponse
		err := json.Unmarshal(bytes.TrimSpace(body), &res)
		assert.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, true, res.Result)
	})

	t.Run("sendrawtransaction_negative", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "sendrawtransaction", "params": ["0274d792072617720636f6e7472616374207472616e73616374696f6e206465736372697074696f6e01949354ea0a8b57dfee1e257a1aedd1e0eea2e5837de145e8da9c0f101bfccc8e0100029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500a3e11100000000ea610aa6db39bd8c8556c9569d94b5e5a5d0ad199b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc5004f2418010000001cc9c05cefffe6cdd7b182816a9152ec218d2ec0014140dbd3cddac5cb2bd9bf6d93701f1a6f1c9dbe2d1b480c54628bbb2a4d536158c747a6af82698edf9f8af1cac3850bcb772bd9c8e4ac38f80704751cc4e0bd0e67232103cbb45da6072c14761c9da545749d9cfd863f860c351066d16df480602a2024c6ac"]}`
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, true)
	})
}

func checkErrResponse(t *testing.T, body []byte, expectingFail bool) {
	var errresp ErrorResponse
	err := json.Unmarshal(bytes.TrimSpace(body), &errresp)
	assert.Nil(t, err)
	if expectingFail {
		assert.NotEqual(t, 0, errresp.Error.Code)
		assert.NotEqual(t, "", errresp.Error.Message)
	} else {
		assert.Equal(t, 0, errresp.Error.Code)
		assert.Equal(t, "", errresp.Error.Message)
	}
}

func doRPCCall(rpcCall string, handler http.HandlerFunc, t *testing.T) []byte {
	req := httptest.NewRequest("POST", "http://0.0.0.0:20333/", strings.NewReader(rpcCall))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	resp := w.Result()
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoErrorf(t, err, "could not read response from the request: %s", rpcCall)
	return body
}
