package rpc

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/stretchr/testify/assert"
)

func TestHandler(t *testing.T) {

	// setup rpcServer server
	net := config.ModeUnitTestNet

	configPath := "../../config"

	cfg, err := config.Load(configPath, net)
	if err != nil {
		t.Fatal("could not create levelDB chain", err)
	}

	chain, err := core.NewBlockchainLevelDB(cfg)
	if err != nil {
		t.Fatal("could not create levelDB chain", err)
	}

	serverConfig := network.NewServerConfig(cfg)
	server := network.NewServer(serverConfig, chain)

	rpcServer := NewServer(chain, cfg.ApplicationConfiguration.RPCPort, server)

	// setup handler
	handler := http.HandlerFunc(rpcServer.requestHandler)

	testCases := []struct {
		rpcCall        string
		method         string
		expectedResult string
	}{
		{`{"jsonrpc": "2.0", "id": 1, "method": "getassetstate", "params": ["602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"] }`,
			"getassetstate_1",
			`{"jsonrpc":"2.0","result":{"assetId":"0x602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7","assetType":1,"name":"NEOGas","amount":"100000000","available":"0","precision":8,"fee":0,"address":"0x0000000000000000000000000000000000000000","owner":"00","admin":"AWKECj9RD8rS8RPcpCgYVjk1DeYyHwxZm3","issuer":"AFmseVrdL9f9oyCzZefL9tG6UbvhPbdYzM","expiration":0,"is_frozen":false},"id":1}`},

		{`{ "jsonrpc": "2.0", "id": 1, "method": "getassetstate", "params": ["c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b"] }`,
			"getassetstate_2",
			`{"jsonrpc":"2.0","result":{"assetId":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","assetType":0,"name":"NEO","amount":"100000000","available":"0","precision":0,"fee":0,"address":"0x0000000000000000000000000000000000000000","owner":"00","admin":"Abf2qMs1pzQb8kYk9RuxtUb9jtRKJVuBJt","issuer":"AFmseVrdL9f9oyCzZefL9tG6UbvhPbdYzM","expiration":0,"is_frozen":false},"id":1}`},

		{`{"jsonrpc": "2.0", "id": 1, "method": "getassetstate", "params": ["62c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"] }`,
			"getassetstate_3",
			`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"unable to decode 62c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7 to Uint256: expected string size of 64 got 63"},"id":1}`},

		{`{"jsonrpc": "2.0", "id": 1, "method": "getassetstate", "params": [123] }`,
			"getassetstate_4",
			`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"expected param at index 0 to be a valid string assetID parameter"},"id":1}`},

		{`{"jsonrpc": "2.0", "id": 1, "method": "getblockhash", "params": [10] }`,
			"getblockhash_1",
			`{"jsonrpc":"2.0","result":"0xd69e7a1f62225a35fed91ca578f33447d93fa0fd2b2f662b957e19c38c1dab1e","id":1}`},

		{`{"jsonrpc": "2.0", "id": 1, "method": "getblockhash", "params": [-2] }`,
			"getblockhash_2",
			`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"Param at index 0 should be greater than or equal to 0 and less then or equal to current block height, got: -2"},"id":1}`},

		{`{"jsonrpc": "2.0", "id": 1, "method": "getblock", "params": [10] }`,
			"getblock",
			`{"jsonrpc":"2.0","result":{"version":0,"previousblockhash":"0x7c5b4c8a70336bf68e8679be7c9a2a15f85c0f6d0e14389019dcc3edfab2bb4b","merkleroot":"0xc027979ad29226b7d34523b1439a64a6cf57fe3f4e823e9d3e90d43934783d26","time":1529926220,"height":10,"nonce":8313828522725096825,"next_consensus":"0xbe48d3a3f5d10013ab9ffee489706078714f1ea2","script":{"invocation":"40ac828e1c2a214e4d356fd2eccc7c7be9ef426f8e4ea67a50464e90ca4367e611c4c5247082b85a7d5ed985cfb90b9af2f1195531038f49c63fb6894b517071ea40b22b83d9457ca5c4c5bb2d8d7e95333820611d447bb171ce7b8af3b999d0a5a61c2301cdd645a33a47defd09c0f237a0afc86e9a84c2fe675d701e4015c0302240a6899296660c612736edc22f8d630927649d4ef1301868079032d80aae6cc1e21622f256497a84a71d7afeeef4c124135f611db24a0f7ab3d2a6886f15db7865","verification":"532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae"},"tx":[{"type":0,"version":0,"attributes":null,"vin":null,"vout":null,"scripts":null}],"confirmations":12338,"nextblockhash":"0x2b1c78633dae7ab81f64362e0828153079a17b018d779d0406491f84c27b086f","hash":"0xd69e7a1f62225a35fed91ca578f33447d93fa0fd2b2f662b957e19c38c1dab1e"},"id":1}`},

		{`{"jsonrpc": "2.0", "id": 1, "method": "getblockcount", "params": [] }`,
			"getblockcount",
			`{"jsonrpc":"2.0","result":12349,"id":1}`},

		{`{"jsonrpc": "2.0", "id": 1, "method": "getconnectioncount", "params": [] }`,
			"getconnectioncount",
			`{"jsonrpc":"2.0","result":0,"id":1}`},

		{`{"jsonrpc": "2.0", "id": 1, "method": "getversion", "params": [] }`,
			"getversion",
			fmt.Sprintf(`{"jsonrpc":"2.0","result":{"port":20333,"nonce":%s,"useragent":"/NEO-GO:/"},"id":1}`, strconv.FormatUint(uint64(server.ID()), 10))},

		{`{"jsonrpc": "2.0", "id": 1, "method": "getbestblockhash", "params": [] }`,
			"getbestblockhash",
			`{"jsonrpc":"2.0","result":"877f5f2084181b85ce4726ab0a86bea6cc82cdbcb6f2eb59e6b04d27fd10929c","id":1}`},

		{`{"jsonrpc": "2.0", "id": 1, "method": "getpeers", "params": [] }`,
			"getpeers",
			`{"jsonrpc":"2.0","result":{"unconnected":[],"connected":[],"bad":[]},"id":1}`},

		{`{ "jsonrpc": "2.0", "id": 1, "method": "getaccountstate", "params": ["AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"] }`,
			"getaccountstate_1",
			`{"jsonrpc":"2.0","result":{"version":0,"address":"AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y","script_hash":"0xe9eed8dc39332032dc22e5d6e86332c50327ba23","frozen":false,"votes":[],"balances":{"602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7":"72099.99960000","c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b":"99989900"}},"id":1}`,
		},

		{`{ "jsonrpc": "2.0", "id": 1, "method": "getaccountstate", "params": ["AK2nJJpJr6o664CWJKi1QRXjqeic2zR"] }`,
			"getaccountstate_2",
			`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"unable to decode AK2nJJpJr6o664CWJKi1QRXjqeic2zR to Uint160: invalid base-58 check string: invalid checksum."},"id":1}`,
		},

		{`{ "jsonrpc": "2.0", "id": 1, "method": "getaccountstate", "params": [123] }`,
			"getaccountstate_3",
			`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"expected param at index 0 to be a valid string account address parameter"},"id":1}`,
		},
	}

	for _, tc := range testCases {

		t.Run(fmt.Sprintf("method: %s, rpc call: %s", tc.method, tc.rpcCall), func(t *testing.T) {

			jsonStr := []byte(tc.rpcCall)

			req := httptest.NewRequest("POST", "http://0.0.0.0:20333/", bytes.NewBuffer(jsonStr))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler(w, req)

			resp := w.Result()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("could not read response from the request: %s", tc.rpcCall)
			}

			fmt.Println(string(bytes.TrimSpace(body)))
			assert.Equal(t, tc.expectedResult, string(bytes.TrimSpace(body)))
		})

	}
}
