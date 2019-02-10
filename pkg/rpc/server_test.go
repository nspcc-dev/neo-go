package rpc

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network"

	//"github.com/CityOfZion/neo-go/pkg/util"
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
			`{"jsonrpc":"2.0","result":"8000012023ba2703c53263e8d6e522dc32203339dcd8eee901ff6a846c115ef1fb88664b00aa67f2c95e9405286db1b56c9120c27c698490530000029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50010a5d4e8000000affb37f5fdb9c6fec48d9f0eee85af82950f9b4a9b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500f01b9b0986230023ba2703c53263e8d6e522dc32203339dcd8eee9014140a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c62321031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac","id":1}`},

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
			`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"unable to decode AK2nJJpJr6o664CWJKi1QRXjqeic2zR to Uint260"},"id":1}`,
		},
		{`{ "jsonrpc": "2.0", "id": 1, "method": "getaccountstate", "params": [123] }`,
			"getaccountstate_3",
			`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"Please provide a valid string account address parameter"},"id":1}`,
		},
		{`{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", 1] }`,
			"getrawtransaction_1",
			`{"jsonrpc":"2.0","result":{"type":"ContractTransaction","version":0,"attributes":[{"usage":"Script","data":"23ba2703c53263e8d6e522dc32203339dcd8eee9"}],"vin":[{"txid":"0x539084697cc220916cb5b16d2805945ec9f267aa004b6688fbf15e116c846aff","vout":0}],"vout":[{"asset":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","value":"10000","address":"AXpNr3SDfLXbPHNdqxYeHK5cYpKMHZxMZ9","n":0},{"asset":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","value":"99990000","address":"AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y","n":1}],"scripts":[{"invocation":"40a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c6","verification":"21031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac"}],"txid":"0xf999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a","size":283,"sys_fee":"0","net_fee":"0","blockhash":"0x6088bf9d3b55c67184f60b00d2e380228f713b4028b24c1719796dcd2006e417","confirmations":2902,"blocktime":1533756500},"id":1}`,
		},
		{`{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", 3] }`,
			"getrawtransaction_2",
			`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"expected param at index 1 to be either 1 or 0"},"id":1}`,
		},
		{`{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", "dads"] }`,
			"getrawtransaction_3",
			`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"expected param at index 1 to be either 1 or 0"},"id":1}`,
		},
		{`{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["45a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", 1] }`,
			"getrawtransaction_4",
			`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params","data":"param at index 0, (45a41306c846ea80290416143e8e856559818065be3f4e143c60e43a), could not be decode to Uint256: expected string size of 64 got 56"},"id":1}`,
		},
		{`{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a"] }`,
			"getrawtransaction_5",
			`{"jsonrpc":"2.0","result":"8000012023ba2703c53263e8d6e522dc32203339dcd8eee901ff6a846c115ef1fb88664b00aa67f2c95e9405286db1b56c9120c27c698490530000029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50010a5d4e8000000affb37f5fdb9c6fec48d9f0eee85af82950f9b4a9b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500f01b9b0986230023ba2703c53263e8d6e522dc32203339dcd8eee9014140a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c62321031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac","id":1}`,
		},
		{`{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", 0] }`,
			"getrawtransaction_6",
			`{"jsonrpc":"2.0","result":"8000012023ba2703c53263e8d6e522dc32203339dcd8eee901ff6a846c115ef1fb88664b00aa67f2c95e9405286db1b56c9120c27c698490530000029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50010a5d4e8000000affb37f5fdb9c6fec48d9f0eee85af82950f9b4a9b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500f01b9b0986230023ba2703c53263e8d6e522dc32203339dcd8eee9014140a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c62321031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac","id":1}`,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("method: %s, rpc call: %s", tc.method, tc.rpcCall), func(t *testing.T) {

			req := httptest.NewRequest("POST", "http://0.0.0.0:20333/", strings.NewReader(tc.rpcCall))
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
