package rpcclient

import (
	"context"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/holiman/uint256"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv/params"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rpcClientTestCase struct {
	name           string
	invoke         func(c *Client) (interface{}, error)
	fails          bool
	serverResponse string
	result         func(c *Client) interface{}
	check          func(t *testing.T, c *Client, result interface{})
}

const base64B1 = "AAAAAMSdeyVO3QCekJTbQ4D7YEEGFv2nXPd6CfO5Kn3htI8P9tVA4+VXLyyMG12BUj6qB3CZ+JIFwWCcDF+KBHH0VSjJICSkegEAAAAAAAAAAAAAAQAAAABouegejA5skHm0udEF6HinbT8iPwHGDEBg0hpK90iZlB4ZSCG7BOr7BsvPXGDax360lvqKeNFuzaGI1RYNH50/dhQLxocy90JdsIOyodd1sOJGEjZIt7ztDEAHc2avJzz6tK+FOQMIZO/FEEikJdLJX0+iZXFcsmDRpB7lo2wWMSQbcoTXNg7leuR0VeDsKJ+YdvCuTG5WbiqWDECa6Yjj+bK4te5KR5jdLF5kLt03csyozZcd/X7NPt89IsX01zpX8ec3e+B2qySJIOhEf3cK0i+5U5wyXiFcRI8xkxMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRRBntDcOgIAAgAAAMDYpwAAAAAADHlDAAAAAACwBAAAAWi56B6MDmyQebS50QXoeKdtPyI/AQBbCwIY3fUFDBTunqIsJ+NL0BSPxBCOCPdOj1BIsgwUaLnoHowObJB5tLnRBeh4p20/Ij8UwB8MCHRyYW5zZmVyDBT1Y+pAvCg9TQ4FxI6jBbPyoHNA70FifVtSOQHGDEC8InWg8rQHWjklRojobu7kn4r0xZY2xWYs15ggVX4PQyEHpNTU6vZHT2TXRdPXAOKHhgWAttO0oTvo+9VZAjIVDEBF0qvBMlvmYJIYLqSoCjhBykcSN78UXrBjO5BKL8BpHtejWCld1VT6Z7nYrEBLgySD6HeMcp/fa6vqHzU220e/DECXtm5AA1jy9GFA7t8U6a+1uPrQFk4Ufp0UyXsun0PvN0NdhrHc37xm8k9Z0dB85V/7WLtkMaLLyjVNVIKImC76kxMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRRBntDcOgADAAAAwNinAAAAAACsiEMAAAAAALAEAAABaLnoHowObJB5tLnRBeh4p20/Ij8BAF8LAwDodkgXAAAADBTunqIsJ+NL0BSPxBCOCPdOj1BIsgwUaLnoHowObJB5tLnRBeh4p20/Ij8UwB8MCHRyYW5zZmVyDBTPduKL0AYsSkeO41VhARMZ88+k0kFifVtSOQHGDEDgj/SQT84EbWRZ4ZKhyjJTuLwVPDgVlQO3CGmgacItvni9nziJvTxziZXBG/0Hqkv68ddS1EH94RtWlqLQWRCjDEAWZUeSQ8KskILSvoWPN3836xpg/TYzOGiFVoePv91CFnap4fRFxdbporBgnZ/sUsjFZ74U8f+r0riqtvkdMMyGDEDx5iho79oDVYOCwIDH3K1UeDjAT6Hq9YsD9SCfJSE1rRsAdJPh2StYxdh9Jah1lwGbW0U+Wu6zpbVFf5CS6fFckxMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRRBntDcOg=="

const base64TxMoveNeo = "AAIAAADA2KcAAAAAAAx5QwAAAAAAsAQAAAFouegejA5skHm0udEF6HinbT8iPwEAWwsCGN31BQwU7p6iLCfjS9AUj8QQjgj3To9QSLIMFGi56B6MDmyQebS50QXoeKdtPyI/FMAfDAh0cmFuc2ZlcgwU9WPqQLwoPU0OBcSOowWz8qBzQO9BYn1bUjkBxgxAvCJ1oPK0B1o5JUaI6G7u5J+K9MWWNsVmLNeYIFV+D0MhB6TU1Or2R09k10XT1wDih4YFgLbTtKE76PvVWQIyFQxARdKrwTJb5mCSGC6kqAo4QcpHEje/FF6wYzuQSi/AaR7Xo1gpXdVU+me52KxAS4Mkg+h3jHKf32ur6h81NttHvwxAl7ZuQANY8vRhQO7fFOmvtbj60BZOFH6dFMl7Lp9D7zdDXYax3N+8ZvJPWdHQfOVf+1i7ZDGiy8o1TVSCiJgu+pMTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUQZ7Q3Do="

const b1Verbose = `{"size":1438,"nextblockhash":"0x34c20650683940a7af1881c2798e83acf9bf98aa226025af6c1d32b5530cc900","confirmations":15,"hash":"0x88c1cbf68695f73fb7b7d185c0037ffebdf032327488ebe65e0533d269e7de9b","version":0,"previousblockhash":"0x0f8fb4e17d2ab9f3097af75ca7fd16064160fb8043db94909e00dd4e257b9dc4","merkleroot":"0x2855f471048a5f0c9c60c10592f8997007aa3e52815d1b8c2c2f57e5e340d5f6","time":1626251469001,"nonce":"0","index":1,"nextconsensus":"NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq","primary":0,"witnesses":[{"invocation":"DEBg0hpK90iZlB4ZSCG7BOr7BsvPXGDax360lvqKeNFuzaGI1RYNH50/dhQLxocy90JdsIOyodd1sOJGEjZIt7ztDEAHc2avJzz6tK+FOQMIZO/FEEikJdLJX0+iZXFcsmDRpB7lo2wWMSQbcoTXNg7leuR0VeDsKJ+YdvCuTG5WbiqWDECa6Yjj+bK4te5KR5jdLF5kLt03csyozZcd/X7NPt89IsX01zpX8ec3e+B2qySJIOhEf3cK0i+5U5wyXiFcRI8x","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFEGe0Nw6"}],"tx":[{"hash":"0xf01080c50f3198f5a539c4a06d024f1b8bdc2a360a215fa7e2488f79a56d501a","size":488,"version":0,"nonce":2,"sender":"NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq","sysfee":"11000000","netfee":"4421900","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x3f223f6da778e805d1b9b479906c0e8c1ee8b968","scopes":"CalledByEntry"}],"script":"CwIY3fUFDBTunqIsJ+NL0BSPxBCOCPdOj1BIsgwUaLnoHowObJB5tLnRBeh4p20/Ij8UwB8MCHRyYW5zZmVyDBT1Y+pAvCg9TQ4FxI6jBbPyoHNA70FifVtSOQ==","witnesses":[{"invocation":"DEC8InWg8rQHWjklRojobu7kn4r0xZY2xWYs15ggVX4PQyEHpNTU6vZHT2TXRdPXAOKHhgWAttO0oTvo+9VZAjIVDEBF0qvBMlvmYJIYLqSoCjhBykcSN78UXrBjO5BKL8BpHtejWCld1VT6Z7nYrEBLgySD6HeMcp/fa6vqHzU220e/DECXtm5AA1jy9GFA7t8U6a+1uPrQFk4Ufp0UyXsun0PvN0NdhrHc37xm8k9Z0dB85V/7WLtkMaLLyjVNVIKImC76","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFEGe0Nw6"}]},{"hash":"0xced32af656e144f6be5d7172ed37747831456cb3eeaac4ee964d0b479b45d3a8","size":492,"version":0,"nonce":3,"sender":"NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq","sysfee":"11000000","netfee":"4425900","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x3f223f6da778e805d1b9b479906c0e8c1ee8b968","scopes":"CalledByEntry"}],"script":"CwMA6HZIFwAAAAwU7p6iLCfjS9AUj8QQjgj3To9QSLIMFGi56B6MDmyQebS50QXoeKdtPyI/FMAfDAh0cmFuc2ZlcgwUz3bii9AGLEpHjuNVYQETGfPPpNJBYn1bUjk=","witnesses":[{"invocation":"DEDgj/SQT84EbWRZ4ZKhyjJTuLwVPDgVlQO3CGmgacItvni9nziJvTxziZXBG/0Hqkv68ddS1EH94RtWlqLQWRCjDEAWZUeSQ8KskILSvoWPN3836xpg/TYzOGiFVoePv91CFnap4fRFxdbporBgnZ/sUsjFZ74U8f+r0riqtvkdMMyGDEDx5iho79oDVYOCwIDH3K1UeDjAT6Hq9YsD9SCfJSE1rRsAdJPh2StYxdh9Jah1lwGbW0U+Wu6zpbVFf5CS6fFc","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFEGe0Nw6"}]}]}`

const base64Header1 = "AAAAAMSdeyVO3QCekJTbQ4D7YEEGFv2nXPd6CfO5Kn3htI8P9tVA4+VXLyyMG12BUj6qB3CZ+JIFwWCcDF+KBHH0VSjJICSkegEAAAAAAAAAAAAAAQAAAABouegejA5skHm0udEF6HinbT8iPwHGDEBg0hpK90iZlB4ZSCG7BOr7BsvPXGDax360lvqKeNFuzaGI1RYNH50/dhQLxocy90JdsIOyodd1sOJGEjZIt7ztDEAHc2avJzz6tK+FOQMIZO/FEEikJdLJX0+iZXFcsmDRpB7lo2wWMSQbcoTXNg7leuR0VeDsKJ+YdvCuTG5WbiqWDECa6Yjj+bK4te5KR5jdLF5kLt03csyozZcd/X7NPt89IsX01zpX8ec3e+B2qySJIOhEf3cK0i+5U5wyXiFcRI8xkxMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRRBntDcOg=="

const header1Verbose = `{"hash":"0x88c1cbf68695f73fb7b7d185c0037ffebdf032327488ebe65e0533d269e7de9b","size":457,"version":0,"previousblockhash":"0x0f8fb4e17d2ab9f3097af75ca7fd16064160fb8043db94909e00dd4e257b9dc4","merkleroot":"0x2855f471048a5f0c9c60c10592f8997007aa3e52815d1b8c2c2f57e5e340d5f6","time":1626251469001,"index":1,"nextconsensus":"NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq","witnesses":[{"invocation":"DEBg0hpK90iZlB4ZSCG7BOr7BsvPXGDax360lvqKeNFuzaGI1RYNH50/dhQLxocy90JdsIOyodd1sOJGEjZIt7ztDEAHc2avJzz6tK+FOQMIZO/FEEikJdLJX0+iZXFcsmDRpB7lo2wWMSQbcoTXNg7leuR0VeDsKJ+YdvCuTG5WbiqWDECa6Yjj+bK4te5KR5jdLF5kLt03csyozZcd/X7NPt89IsX01zpX8ec3e+B2qySJIOhEf3cK0i+5U5wyXiFcRI8x","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFEGe0Nw6"}],"confirmations":15,"nextblockhash":"0x34c20650683940a7af1881c2798e83acf9bf98aa226025af6c1d32b5530cc900"}`

const txMoveNeoVerbose = `{"blockhash":"0x88c1cbf68695f73fb7b7d185c0037ffebdf032327488ebe65e0533d269e7de9b","confirmations":15,"blocktime":1626251469001,"vmstate":"HALT","hash":"0xf01080c50f3198f5a539c4a06d024f1b8bdc2a360a215fa7e2488f79a56d501a","size":488,"version":0,"nonce":2,"sender":"NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq","sysfee":"11000000","netfee":"4421900","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x3f223f6da778e805d1b9b479906c0e8c1ee8b968","scopes":"CalledByEntry"}],"script":"CwIY3fUFDBTunqIsJ+NL0BSPxBCOCPdOj1BIsgwUaLnoHowObJB5tLnRBeh4p20/Ij8UwB8MCHRyYW5zZmVyDBT1Y+pAvCg9TQ4FxI6jBbPyoHNA70FifVtSOQ==","witnesses":[{"invocation":"DEC8InWg8rQHWjklRojobu7kn4r0xZY2xWYs15ggVX4PQyEHpNTU6vZHT2TXRdPXAOKHhgWAttO0oTvo+9VZAjIVDEBF0qvBMlvmYJIYLqSoCjhBykcSN78UXrBjO5BKL8BpHtejWCld1VT6Z7nYrEBLgySD6HeMcp/fa6vqHzU220e/DECXtm5AA1jy9GFA7t8U6a+1uPrQFk4Ufp0UyXsun0PvN0NdhrHc37xm8k9Z0dB85V/7WLtkMaLLyjVNVIKImC76","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFEGe0Nw6"}]}`

// getResultBlock1 returns data for block number 1 which is used by several tests.
func getResultBlock1() *result.Block {
	binB, err := base64.StdEncoding.DecodeString(base64B1)
	if err != nil {
		panic(err)
	}
	b := block.New(false)
	err = testserdes.DecodeBinary(binB, b)
	if err != nil {
		panic(err)
	}
	b2Hash, err := util.Uint256DecodeStringLE("34c20650683940a7af1881c2798e83acf9bf98aa226025af6c1d32b5530cc900")
	if err != nil {
		panic(err)
	}
	return &result.Block{
		Block: *b,
		BlockMetadata: result.BlockMetadata{
			Size:          1438,
			NextBlockHash: &b2Hash,
			Confirmations: 15,
		},
	}
}

func getTxMoveNeo() *result.TransactionOutputRaw {
	b1 := getResultBlock1()
	txBin, err := base64.StdEncoding.DecodeString(base64TxMoveNeo)
	if err != nil {
		panic(err)
	}
	tx, err := transaction.NewTransactionFromBytes(txBin)
	if err != nil {
		panic(err)
	}
	return &result.TransactionOutputRaw{
		Transaction: *tx,
		TransactionMetadata: result.TransactionMetadata{
			Timestamp:     b1.Timestamp,
			Blockhash:     b1.Block.Hash(),
			Confirmations: int(b1.Confirmations),
			VMState:       "HALT",
		},
	}
}

// rpcClientTestCases contains `serverResponse` json data fetched from examples
// published in the official C# JSON-RPC API v2.10.3 reference
// (see https://docs.neo.org/docs/en-us/reference/rpc/latest-version/api.html)
var rpcClientTestCases = map[string][]rpcClientTestCase{
	"getapplicationlog": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{}, nil)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"txid":"0x17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521","executions":[{"trigger":"Application","vmstate":"HALT","gasconsumed":"1","stack":[{"type":"Integer","value":"1"}],"notifications":[]}]}}`,
			result: func(c *Client) interface{} {
				txHash, err := util.Uint256DecodeStringLE("17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521")
				if err != nil {
					panic(err)
				}
				return &result.ApplicationLog{
					Container: txHash,
					Executions: []state.Execution{
						{
							Trigger:     trigger.Application,
							VMState:     vmstate.Halt,
							GasConsumed: 1,
							Stack:       []stackitem.Item{stackitem.NewBigInteger(uint256.NewInt(1))},
							Events:      []state.NotificationEvent{},
						},
					},
				}
			},
		},
	},
	"getbestblockhash": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"0x773dd2dae4a9c9275290f89b56e67d7363ea4826dfd4fc13cc01cf73a44b0d0e"}`,
			result: func(c *Client) interface{} {
				result, err := util.Uint256DecodeStringLE("773dd2dae4a9c9275290f89b56e67d7363ea4826dfd4fc13cc01cf73a44b0d0e")
				if err != nil {
					panic(err)
				}
				return result
			},
		},
	},
	"getblock": {
		{
			name: "byIndex_positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + base64B1 + `"}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return &b.Block
			},
		},
		{
			name: "byIndex_verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				return c.GetBlockByIndexVerbose(1)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":` + b1Verbose + `}`,
			result: func(c *Client) interface{} {
				res := getResultBlock1()
				return res
			},
		},
		{
			name: "byHash_positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("d151651e86680a7ecbc87babf3346a42e7bc9974414ce192c9c22ac4f2e9d043")
				if err != nil {
					panic(err)
				}
				return c.GetBlockByHash(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + base64B1 + `"}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return &b.Block
			},
		},
		{
			name: "byHash_verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("86fe1061140b2ea791b0739fb9732abc6e5e47de4927228a1ac41de3d93eb7cb")
				if err != nil {
					panic(err)
				}
				return c.GetBlockByHashVerbose(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":` + b1Verbose + `}`,
			result: func(c *Client) interface{} {
				res := getResultBlock1()
				return res
			},
		},
	},
	"getblockcount": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockCount()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":991991}`,
			result: func(c *Client) interface{} {
				return uint32(991991)
			},
		},
	},
	"getblockhash": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(1)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"0x4c1e879872344349067c3b1a30781eeb4f9040d3795db7922f513f6f9660b9b2"}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint256DecodeStringLE("4c1e879872344349067c3b1a30781eeb4f9040d3795db7922f513f6f9660b9b2")
				if err != nil {
					panic(err)
				}
				return hash
			},
		},
	},
	"getblockheader": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("68e4bd688b852e807eef13a0ff7da7b02223e359a35153667e88f9cb4a3b0801")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + base64Header1 + `"}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return &b.Header
			},
		},
		{
			name: "verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("cbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeaderVerbose(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":` + header1Verbose + `}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return &result.Header{
					Header: b.Header,
					BlockMetadata: result.BlockMetadata{
						Size:          457,
						NextBlockHash: b.NextBlockHash,
						Confirmations: b.Confirmations,
					},
				}
			},
		},
	},
	"getblockheadercount": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeaderCount()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":2021}`,
			result: func(c *Client) interface{} {
				return uint32(2021)
			},
		},
	},
	"getblocksysfee": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"195500"}`,
			result: func(c *Client) interface{} {
				return fixedn.Fixed8FromInt64(195500)
			},
		},
	},
	"getcommittee": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetCommittee()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":["02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e"]}`,
			result: func(c *Client) interface{} {
				member, err := keys.NewPublicKeyFromString("02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e")
				if err != nil {
					panic(fmt.Errorf("failed to decode public key: %w", err))
				}
				return keys.PublicKeys{member}
			},
		},
	},
	"getconnectioncount": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":10}`,
			result: func(c *Client) interface{} {
				return 10
			},
		},
	},
	"getcontractstate": {
		{
			name: "positive, by hash",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("1b4357bff5a01bdf2a6581247cf9ed1e24629176")
				if err != nil {
					panic(err)
				}
				return c.GetContractStateByHash(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"id":0,"nef":{"magic":860243278,"compiler":"neo-go-3.0","script":"VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==","checksum":2512077441},"manifest":{"name":"Test","abi":{"methods":[],"events":[]},"groups":[],"features":{},"permissions":[],"trusts":[],"supportedstandards":[],"safemethods":[],"extra":null},"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176"}}`,
			result: func(c *Client) interface{} {
				script, err := base64.StdEncoding.DecodeString("VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==")
				if err != nil {
					panic(err)
				}
				m := manifest.NewManifest("Test")
				cs := &state.Contract{
					ContractBase: state.ContractBase{
						ID:       0,
						Hash:     hash.Hash160(script),
						NEF:      newTestNEF(script),
						Manifest: *m,
					},
				}
				return cs
			},
		},
		{
			name: "positive, by address",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractStateByAddressOrName("NWiu5oejTu925aeL9Hc1LX8SvaJhE23h15")
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"id":0,"nef":{"magic":860243278,"compiler":"neo-go-3.0","script":"VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==","checksum":2512077441},"manifest":{"name":"Test","abi":{"methods":[],"events":[]},"groups":[],"features":{},"permissions":[],"trusts":[],"supportedstandards":[],"safemethods":[],"extra":null},"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176"}}`,
			result: func(c *Client) interface{} {
				script, err := base64.StdEncoding.DecodeString("VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==")
				if err != nil {
					panic(err)
				}
				m := manifest.NewManifest("Test")
				cs := &state.Contract{
					ContractBase: state.ContractBase{
						ID:       0,
						Hash:     hash.Hash160(script),
						NEF:      newTestNEF(script),
						Manifest: *m,
					},
				}
				return cs
			},
		},
		{
			name: "positive, by id",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractStateByID(0)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"id":0,"nef":{"magic":860243278,"compiler":"neo-go-3.0","script":"VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==","checksum":2512077441},"manifest":{"name":"Test","abi":{"methods":[],"events":[]},"groups":[],"features":{},"permissions":[],"trusts":[],"supportedstandards":[],"safemethods":[],"extra":null},"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176"}}`,
			result: func(c *Client) interface{} {
				script, err := base64.StdEncoding.DecodeString("VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==")
				if err != nil {
					panic(err)
				}
				m := manifest.NewManifest("Test")
				cs := &state.Contract{
					ContractBase: state.ContractBase{
						ID:       0,
						Hash:     hash.Hash160(script),
						NEF:      newTestNEF(script),
						Manifest: *m,
					},
				}
				return cs
			},
		},
	},
	"getFeePerByte": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetFeePerByte()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMDWdldEZlZVBlckJ5dGUMFJphpG7sl7iTBtfOgfFbRiCR0AkyQWJ9W1I=","stack":[{"type":"Integer","value":"1000"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(1000)
			},
		},
	},
	"getExecFeeFactor": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetExecFeeFactor()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMDWdldEZlZVBlckJ5dGUMFJphpG7sl7iTBtfOgfFbRiCR0AkyQWJ9W1I=","stack":[{"type":"Integer","value":"1000"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(1000)
			},
		},
	},
	"getStoragePrice": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStoragePrice()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMDWdldEZlZVBlckJ5dGUMFJphpG7sl7iTBtfOgfFbRiCR0AkyQWJ9W1I=","stack":[{"type":"Integer","value":"100000"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(100000)
			},
		},
	},
	"getOraclePrice": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetOraclePrice()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMDWdldEZlZVBlckJ5dGUMFJphpG7sl7iTBtfOgfFbRiCR0AkyQWJ9W1I=","stack":[{"type":"Integer","value":"10000000"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(10000000)
			},
		},
	},
	"getNNSPrice": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNNSPrice(util.Uint160{1, 2, 3})
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMDWdldEZlZVBlckJ5dGUMFJphpG7sl7iTBtfOgfFbRiCR0AkyQWJ9W1I=","stack":[{"type":"Integer","value":"1000000"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(1000000)
			},
		},
	},
	"getGasPerBlock": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetGasPerBlock()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMDWdldEZlZVBlckJ5dGUMFJphpG7sl7iTBtfOgfFbRiCR0AkyQWJ9W1I=","stack":[{"type":"Integer","value":"500000000"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(500000000)
			},
		},
	},
	"getCandidateRegisterPrice": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetCandidateRegisterPrice()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMDWdldEZlZVBlckJ5dGUMFJphpG7sl7iTBtfOgfFbRiCR0AkyQWJ9W1I=","stack":[{"type":"Integer","value":"100000000000"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(100000000000)
			},
		},
	},
	"getDesignatedByRole": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetDesignatedByRole(noderoles.P2PNotary, 10)
			},
			serverResponse: `{"id" : 1,"result" : {"stack" : [{"value" : [{"type":"ByteString","value":"Aw0WkQoDc8WqpG18xPMTEgfHO6gRTVtMN0Mw6zw06fzl"},{"type":"ByteString","value":"A+bmJ9wIaj96Ygr+uQQvQ0AaUrQmj2b3AGnztAOkU3/L"}],"type" : "Array"}],"exception" : null,"script" : "ERQSwB8ME2dldERlc2lnbmF0ZWRCeVJvbGUMFOKV45FUTBeK2U8D7E3N/3hTTs9JQWJ9W1I=","gasconsumed" : "2028150","state" : "HALT"}, "jsonrpc" : "2.0"}`,
			result: func(c *Client) interface{} {
				pk1Bytes, _ := base64.StdEncoding.DecodeString("Aw0WkQoDc8WqpG18xPMTEgfHO6gRTVtMN0Mw6zw06fzl")
				pk1, err := keys.NewPublicKeyFromBytes(pk1Bytes, elliptic.P256())
				if err != nil {
					panic("invalid pub key #1 bytes")
				}
				pk2Bytes, _ := base64.StdEncoding.DecodeString("A+bmJ9wIaj96Ygr+uQQvQ0AaUrQmj2b3AGnztAOkU3/L")
				pk2, err := keys.NewPublicKeyFromBytes(pk2Bytes, elliptic.P256())
				if err != nil {
					panic("invalid pub key #2 bytes")
				}
				return keys.PublicKeys{pk1, pk2}
			},
		},
	},
	"getMaxNotValidBeforeDelta": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetMaxNotValidBeforeDelta()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMD2dldE1heEJsb2NrU2l6ZQwUmmGkbuyXuJMG186B8VtGIJHQCTJBYn1bUg==","stack":[{"type":"Integer","value":"262144"}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return int64(262144)
			},
		},
	},
	"isBlocked": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.IsBlocked(util.Uint160{1, 2, 3})
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"state":"HALT","gasconsumed":"2007390","script":"EMAMEmdldEJsb2NrZWRBY2NvdW50cwwUmmGkbuyXuJMG186B8VtGIJHQCTJBYn1bUg==","stack":[{"type":"Boolean","value":false}],"tx":null}}`,
			result: func(c *Client) interface{} {
				return false
			},
		},
	},
	"getnep11balances": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("1aada0032aba1ef6d1f07bbd8bec1d85f5380fb3")
				if err != nil {
					panic(err)
				}
				return c.GetNEP11Balances(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"balance":[{"assethash":"a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8","symbol":"SOME","decimals":"42","name":"Contract","tokens":[{"tokenid":"abcdef","amount":"1","lastupdatedblock":251604}]}],"address":"NcEkNmgWmf7HQVQvzhxpengpnt4DXjmZLe"}}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint160DecodeStringLE("a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8")
				if err != nil {
					panic(err)
				}
				return &result.NEP11Balances{
					Balances: []result.NEP11AssetBalance{{
						Asset:    hash,
						Decimals: 42,
						Name:     "Contract",
						Symbol:   "SOME",
						Tokens: []result.NEP11TokenBalance{{
							ID:          "abcdef",
							Amount:      "1",
							LastUpdated: 251604,
						}},
					}},
					Address: "NcEkNmgWmf7HQVQvzhxpengpnt4DXjmZLe",
				}
			},
		},
	},
	"getnep17balances": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("1aada0032aba1ef6d1f07bbd8bec1d85f5380fb3")
				if err != nil {
					panic(err)
				}
				return c.GetNEP17Balances(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"balance":[{"assethash":"a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8","symbol":"N17","decimals":"8","name":"Token","amount":"50000000000","lastupdatedblock":251604}],"address":"AY6eqWjsUFCzsVELG7yG72XDukKvC34p2w"}}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint160DecodeStringLE("a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8")
				if err != nil {
					panic(err)
				}
				return &result.NEP17Balances{
					Balances: []result.NEP17Balance{{
						Asset:       hash,
						Decimals:    8,
						Name:        "Token",
						Symbol:      "N17",
						Amount:      "50000000000",
						LastUpdated: 251604,
					}},
					Address: "AY6eqWjsUFCzsVELG7yG72XDukKvC34p2w",
				}
			},
		},
	},
	"getnep11properties": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("1aada0032aba1ef6d1f07bbd8bec1d85f5380fb3")
				if err != nil {
					panic(err)
				}
				return c.GetNEP11Properties(hash, []byte("abcdef"))
			}, // NcEkNmgWmf7HQVQvzhxpengpnt4DXjmZLe
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"name":"sometoken","field1":"c29tZXRoaW5n","field2":null}}`,
			result: func(c *Client) interface{} {
				return map[string]interface{}{
					"name":   "sometoken",
					"field1": []byte("something"),
					"field2": nil,
				}
			},
		},
	},
	"getnep11transfers": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := address.StringToUint160("NcEkNmgWmf7HQVQvzhxpengpnt4DXjmZLe")
				if err != nil {
					panic(err)
				}
				return c.GetNEP11Transfers(hash, nil, nil, nil, nil)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"sent":[],"received":[{"timestamp":1555651816,"assethash":"600c4f5200db36177e3e8a09e9f18e2fc7d12a0f","transferaddress":"NfgHwwTi3wHAS8aFAN243C5vGbkYDpqLHP","amount":"1","tokenid":"abcdef","blockindex":436036,"transfernotifyindex":0,"txhash":"df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58"}],"address":"NcEkNmgWmf7HQVQvzhxpengpnt4DXjmZLe"}}`,
			result: func(c *Client) interface{} {
				assetHash, err := util.Uint160DecodeStringLE("600c4f5200db36177e3e8a09e9f18e2fc7d12a0f")
				if err != nil {
					panic(err)
				}
				txHash, err := util.Uint256DecodeStringLE("df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58")
				if err != nil {
					panic(err)
				}
				return &result.NEP11Transfers{
					Sent: []result.NEP11Transfer{},
					Received: []result.NEP11Transfer{
						{
							Timestamp:   1555651816,
							Asset:       assetHash,
							Address:     "NfgHwwTi3wHAS8aFAN243C5vGbkYDpqLHP",
							Amount:      "1",
							ID:          "abcdef",
							Index:       436036,
							NotifyIndex: 0,
							TxHash:      txHash,
						},
					},
					Address: "NcEkNmgWmf7HQVQvzhxpengpnt4DXjmZLe",
				}
			},
		},
	},
	"getnep17transfers": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := address.StringToUint160("NcEkNmgWmf7HQVQvzhxpengpnt4DXjmZLe")
				if err != nil {
					panic(err)
				}
				return c.GetNEP17Transfers(hash, nil, nil, nil, nil)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"sent":[],"received":[{"timestamp":1555651816,"assethash":"600c4f5200db36177e3e8a09e9f18e2fc7d12a0f","transferaddress":"AYwgBNMepiv5ocGcyNT4mA8zPLTQ8pDBis","amount":"1000000","blockindex":436036,"transfernotifyindex":0,"txhash":"df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58"}],"address":"NcEkNmgWmf7HQVQvzhxpengpnt4DXjmZLe"}}`,
			result: func(c *Client) interface{} {
				assetHash, err := util.Uint160DecodeStringLE("600c4f5200db36177e3e8a09e9f18e2fc7d12a0f")
				if err != nil {
					panic(err)
				}
				txHash, err := util.Uint256DecodeStringLE("df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58")
				if err != nil {
					panic(err)
				}
				return &result.NEP17Transfers{
					Sent: []result.NEP17Transfer{},
					Received: []result.NEP17Transfer{
						{
							Timestamp:   1555651816,
							Asset:       assetHash,
							Address:     "AYwgBNMepiv5ocGcyNT4mA8zPLTQ8pDBis",
							Amount:      "1000000",
							Index:       436036,
							NotifyIndex: 0,
							TxHash:      txHash,
						},
					},
					Address: "NcEkNmgWmf7HQVQvzhxpengpnt4DXjmZLe",
				}
			},
		},
	},
	"getpeers": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetPeers()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"unconnected":[{"address":"172.200.0.1","port":"20333"}],"connected":[{"address":"127.0.0.1","port":"20335"}],"bad":[{"address":"172.200.0.254","port":"20332"}]}}`,
			result: func(c *Client) interface{} {
				return &result.GetPeers{
					Unconnected: result.Peers{
						{
							Address: "172.200.0.1",
							Port:    "20333",
						},
					},
					Connected: result.Peers{
						{
							Address: "127.0.0.1",
							Port:    "20335",
						},
					},
					Bad: result.Peers{
						{
							Address: "172.200.0.254",
							Port:    "20332",
						},
					},
				}
			},
		},
	},
	"getrawmempool": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawMemPool()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":["0x9786cce0dddb524c40ddbdd5e31a41ed1f6b5c8a683c122f627ca4a007a7cf4e"]}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint256DecodeStringLE("9786cce0dddb524c40ddbdd5e31a41ed1f6b5c8a683c122f627ca4a007a7cf4e")
				if err != nil {
					panic(err)
				}
				return []util.Uint256{hash}
			},
		},
	},
	"getrawtransaction": {
		{
			name: "positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("f5fbd303799f24ba247529d7544d4276cca54ea79f4b98095f2b0557313c5275")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"` + base64TxMoveNeo + `"}`,
			result: func(c *Client) interface{} {
				tx := getTxMoveNeo()
				return &tx.Transaction
			},
		},
		{
			name: "verbose_positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("f5fbd303799f24ba247529d7544d4276cca54ea79f4b98095f2b0557313c5275")
				if err != nil {
					panic(err)
				}
				out, err := c.GetRawTransactionVerbose(hash)
				if err != nil {
					return nil, err
				}
				out.Transaction.FeePerByte() // set fee per byte
				return out, nil
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":` + txMoveNeoVerbose + `}`,
			result: func(c *Client) interface{} {
				return getTxMoveNeo()
			},
		},
	},
	"getstateroot": {
		{
			name: "positive, by height",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStateRootByHeight(5)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"version":0,"index":5,"roothash":"0x65d19151694321e70c6d184b37a2bcf7af4a2c60c099af332a4f7815e3670686","witnesses":[]}}`,
			result: func(c *Client) interface{} {
				h, err := util.Uint256DecodeStringLE("65d19151694321e70c6d184b37a2bcf7af4a2c60c099af332a4f7815e3670686")
				if err != nil {
					panic(err)
				}
				return &state.MPTRoot{
					Version: 0,
					Index:   5,
					Root:    h,
					Witness: []transaction.Witness{},
				}
			},
		},
		{
			name: "positive, by hash",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("86fe1061140b2ea791b0739fb9732abc6e5e47de4927228a1ac41de3d93eb7cb")
				if err != nil {
					panic(err)
				}
				return c.GetStateRootByBlockHash(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"version":0,"index":5,"roothash":"0x65d19151694321e70c6d184b37a2bcf7af4a2c60c099af332a4f7815e3670686","witnesses":[]}}`,
			result: func(c *Client) interface{} {
				h, _ := util.Uint256DecodeStringLE("65d19151694321e70c6d184b37a2bcf7af4a2c60c099af332a4f7815e3670686")
				return &state.MPTRoot{
					Version: 0,
					Index:   5,
					Root:    h,
					Witness: []transaction.Witness{},
				}
			},
		},
	},
	"getstate": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				root, _ := util.Uint256DecodeStringLE("252e9d73d49c95c7618d40650da504e05183a1b2eed0685e42c360413c329170")
				cHash, _ := util.Uint160DecodeStringLE("5c9e40a12055c6b9e3f72271c9779958c842135d")
				return c.GetState(root, cHash, []byte("testkey"))
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"dGVzdHZhbHVl"}`,
			result: func(c *Client) interface{} {
				return []byte("testvalue")
			},
		},
	},
	"findstates": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				root, _ := util.Uint256DecodeStringLE("252e9d73d49c95c7618d40650da504e05183a1b2eed0685e42c360413c329170")
				cHash, _ := util.Uint160DecodeStringLE("5c9e40a12055c6b9e3f72271c9779958c842135d")
				count := 1
				return c.FindStates(root, cHash, []byte("aa"), []byte("aa00"), &count)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"results":[{"key":"YWExMA==","value":"djI="}],"firstProof":"CAEAAABhYTEwCXIAA5KNHjQ1+LFX4lQBLjMAhaLtTJnfuI86O7WnNdlshsYWBAQEBAQEBAQDTzD7MJp2KW6E8BNVjjjgZMTAjI/GI3ZrTmR2UUOtSeIEBAQEBAPKPqb0qnb4Ywz6gqpNKCUNQsfBmAnKc5p3dxokSQRpwgRSAAQDPplG1wee4KOfkehaF94R5uoKSgvQL1j5gkFTN4ywYaIEBAOhOyI39MZfoKc940g57XeqwRnxh7P62fKjnfEtBzQxHQQEBAQEBAQEBAQEBCkBBgAAAAAAAAM6A1UrwFYZAEMfe6go3jX25xz2sHsovQ2UO/UHqZZOXLIABAOwg7pkXyaTR85yQIvYnoGaG/OVRLRHOj+nhZnXb6dVtAQEBAPnciBUp3uspLQTajKTlAxgrNe+3tlqlbwlNRkz0eNmhQMzoMcWOFi9nCyn+eM5lA6Pq67DxzTQDlHljh8g8kRtJAPq9hxzTgreK0qDTavsethixguZYfV7wDmKfumMglnoqQQEBAQEBAM1x2dVBdf5BJ0Xvw2qqhvpKqxdHb8/HMFWiXkJj1uAAQQEJgEDAQYBA5kV2WLkgey9C5z6gZT69VLKcEuwyY8P853rNtGhT3NeUgAEBAQDiX59K9PuJ5RE7Z1uj7q/QJ8FGf8avLdWM7hwmWkVH2gEBAQEBAQEBAQEBAQD1SubX5XhFHcUOWdUzg1bXmDwWJwt+wpU3FOdFkU1PXBSAAQDHCzfEQyqwOO263EE6HER1vWDrwz8JiEHEOXfZ3kX7NYEBAQDEH++Hy8wBcniKuWVevaAwzHCh60kzncU30E5fDC3gJsEBAQEBAQEBAQEBCUBAgMAA1wt18LbxMKdYcJ+nEDMMWZbRsu550l8HGhcYhpl6DjSBAICdjI=","truncated":true}}`,
			result: func(c *Client) interface{} {
				proofB, _ := base64.StdEncoding.DecodeString("CAEAAABhYTEwCXIAA5KNHjQ1+LFX4lQBLjMAhaLtTJnfuI86O7WnNdlshsYWBAQEBAQEBAQDTzD7MJp2KW6E8BNVjjjgZMTAjI/GI3ZrTmR2UUOtSeIEBAQEBAPKPqb0qnb4Ywz6gqpNKCUNQsfBmAnKc5p3dxokSQRpwgRSAAQDPplG1wee4KOfkehaF94R5uoKSgvQL1j5gkFTN4ywYaIEBAOhOyI39MZfoKc940g57XeqwRnxh7P62fKjnfEtBzQxHQQEBAQEBAQEBAQEBCkBBgAAAAAAAAM6A1UrwFYZAEMfe6go3jX25xz2sHsovQ2UO/UHqZZOXLIABAOwg7pkXyaTR85yQIvYnoGaG/OVRLRHOj+nhZnXb6dVtAQEBAPnciBUp3uspLQTajKTlAxgrNe+3tlqlbwlNRkz0eNmhQMzoMcWOFi9nCyn+eM5lA6Pq67DxzTQDlHljh8g8kRtJAPq9hxzTgreK0qDTavsethixguZYfV7wDmKfumMglnoqQQEBAQEBAM1x2dVBdf5BJ0Xvw2qqhvpKqxdHb8/HMFWiXkJj1uAAQQEJgEDAQYBA5kV2WLkgey9C5z6gZT69VLKcEuwyY8P853rNtGhT3NeUgAEBAQDiX59K9PuJ5RE7Z1uj7q/QJ8FGf8avLdWM7hwmWkVH2gEBAQEBAQEBAQEBAQD1SubX5XhFHcUOWdUzg1bXmDwWJwt+wpU3FOdFkU1PXBSAAQDHCzfEQyqwOO263EE6HER1vWDrwz8JiEHEOXfZ3kX7NYEBAQDEH++Hy8wBcniKuWVevaAwzHCh60kzncU30E5fDC3gJsEBAQEBAQEBAQEBCUBAgMAA1wt18LbxMKdYcJ+nEDMMWZbRsu550l8HGhcYhpl6DjSBAICdjI=")
				proof := &result.ProofWithKey{}
				r := io.NewBinReaderFromBuf(proofB)
				proof.DecodeBinary(r)
				return result.FindStates{
					Results:    []result.KeyValue{{Key: []byte("aa10"), Value: []byte("v2")}},
					FirstProof: proof,
					Truncated:  true,
				}
			},
		},
	},
	"getstateheight": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStateHeight()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"localrootindex":11646,"validatedrootindex":11645}}`,
			result: func(c *Client) interface{} {
				return &result.StateHeight{
					Local:     11646,
					Validated: 11645,
				}
			},
		},
	},
	"getstorage": {
		{
			name: "by hash, positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("03febccf81ac85e3d795bc5cbd4e84e907812aa3")
				if err != nil {
					panic(err)
				}
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorageByHash(hash, key)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"TGlu"}`,
			result: func(c *Client) interface{} {
				value, err := hex.DecodeString("4c696e")
				if err != nil {
					panic(err)
				}
				return value
			},
		},
		{
			name: "by ID, positive",
			invoke: func(c *Client) (interface{}, error) {
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorageByID(-1, key)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"TGlu"}`,
			result: func(c *Client) interface{} {
				value, err := hex.DecodeString("4c696e")
				if err != nil {
					panic(err)
				}
				return value
			},
		},
	},
	"gettransactionheight": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2")
				if err != nil {
					panic(err)
				}
				return c.GetTransactionHeight(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":1}`,
			result: func(c *Client) interface{} {
				return uint32(1)
			},
		},
	},
	"getunclaimedgas": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimedGas("NMipL5VsNoLUBUJKPKLhxaEbPQVCZnyJyB")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"address":"NMipL5VsNoLUBUJKPKLhxaEbPQVCZnyJyB","unclaimed":"897299680935"}}`,
			result: func(c *Client) interface{} {
				addr, err := address.StringToUint160("NMipL5VsNoLUBUJKPKLhxaEbPQVCZnyJyB")
				if err != nil {
					panic(fmt.Errorf("failed to parse UnclaimedGas address: %w", err))
				}
				return result.UnclaimedGas{
					Address:   addr,
					Unclaimed: *big.NewInt(897299680935),
				}
			},
		},
	},
	"getcandidates": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetCandidates()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":[{"publickey":"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2","votes":"0","active":true},{"publickey":"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e","votes":"0","active":true},{"publickey":"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699","votes":"0","active":true},{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"0","active":true}]}`,
			result:         func(c *Client) interface{} { return []result.Candidate{} },
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.([]result.Candidate)
				require.True(t, ok)
				assert.Equal(t, 4, len(res))
			},
		},
	},
	"getvalidators": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNextBlockValidators()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":[{"publickey":"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2","votes":"0","active":true},{"publickey":"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e","votes":"0","active":true},{"publickey":"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699","votes":"0","active":true},{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"0","active":true}]}`,
			result:         func(c *Client) interface{} { return []result.Validator{} },
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.([]result.Validator)
				require.True(t, ok)
				assert.Equal(t, 4, len(res))
			},
		},
	},
	"getversion": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetVersion()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"tcpport":20332,"wsport":20342,"nonce":2153672787,"useragent":"/NEO-GO:0.73.1-pre-273-ge381358/"}}`,
			result: func(c *Client) interface{} {
				return &result.Version{
					TCPPort:   uint16(20332),
					WSPort:    uint16(20342),
					Nonce:     2153672787,
					UserAgent: "/NEO-GO:0.73.1-pre-273-ge381358/",
					Protocol: result.Protocol{
						Network: netmode.UnitTestNet,
					},
				}
			},
		},
	},
	"invokefunction": {
		{
			name: "positive, by scripthash",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("91b83e96f2a7c4fdf0c1688441ec61986c7cae26")
				if err != nil {
					panic(err)
				}
				contr, err := util.Uint160DecodeStringLE("af7c7328eee5a275a3bcaee2bf0cf662b5e739be")
				if err != nil {
					panic(err)
				}
				return c.InvokeFunction(contr, "balanceOf", []smartcontract.Parameter{
					{
						Type:  smartcontract.Hash160Type,
						Value: hash,
					},
				}, []transaction.Signer{{
					Account: util.Uint160{1, 2, 3},
				}})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"FCaufGyYYexBhGjB8P3Ep/KWPriRUcEJYmFsYW5jZU9mZ74557Vi9gy/4q68o3Wi5e4oc3yv","state":"HALT","gasconsumed":"31100000","stack":[{"type":"ByteString","value":"JivsCEQy"}],"tx":"AAgAAACAlpgAAAAAAAIEEwAAAAAAsAQAAAGqis+FnU/kArNOZz8hVoIXlqSI6wEAVwHoAwwUqorPhZ1P5AKzTmc/IVaCF5akiOsMFOeetm08E0pKd27oB9LluEbdpP2wE8AMCHRyYW5zZmVyDBTnnrZtPBNKSndu6AfS5bhG3aT9sEFifVtSOAFCDEDYNAh3TUvYsZrocFYdBvJ0Trdnj1jRuQzy9Q6YroP2Cwgk4v7q3vbeZBikz8Q7vB+RbDPsWUy+ZiqdkkeG4XoUKQwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CC0GVRA14"}}`,
			result: func(c *Client) interface{} {
				return &result.Invoke{}
			},
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.(*result.Invoke)
				require.True(t, ok)
				bytes, err := hex.DecodeString("262bec084432")
				if err != nil {
					panic(err)
				}
				script, err := base64.StdEncoding.DecodeString("FCaufGyYYexBhGjB8P3Ep/KWPriRUcEJYmFsYW5jZU9mZ74557Vi9gy/4q68o3Wi5e4oc3yv")
				if err != nil {
					panic(err)
				}
				assert.Equal(t, "HALT", res.State)
				assert.Equal(t, int64(31100000), res.GasConsumed)
				assert.Equal(t, script, res.Script)
				assert.Equal(t, []stackitem.Item{stackitem.NewByteArray(bytes)}, res.Stack)
				assert.NotNil(t, res.Transaction)
			},
		},
		{
			name: "positive, FAULT state",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("91b83e96f2a7c4fdf0c1688441ec61986c7cae26")
				if err != nil {
					panic(err)
				}
				contr, err := util.Uint160DecodeStringLE("af7c7328eee5a275a3bcaee2bf0cf662b5e739be")
				if err != nil {
					panic(err)
				}
				return c.InvokeFunction(contr, "balanceOf", []smartcontract.Parameter{
					{
						Type:  smartcontract.Hash160Type,
						Value: hash,
					},
				}, []transaction.Signer{{
					Account: util.Uint160{1, 2, 3},
				}})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"FCaufGyYYexBhGjB8P3Ep/KWPriRUcEJYmFsYW5jZU9mZ74557Vi9gy/4q68o3Wi5e4oc3yv","state":"FAULT","gasconsumed":"31100000","stack":[{"type":"ByteString","value":"JivsCEQy"}],"tx":"AAgAAACAlpgAAAAAAAIEEwAAAAAAsAQAAAGqis+FnU/kArNOZz8hVoIXlqSI6wEAVwHoAwwUqorPhZ1P5AKzTmc/IVaCF5akiOsMFOeetm08E0pKd27oB9LluEbdpP2wE8AMCHRyYW5zZmVyDBTnnrZtPBNKSndu6AfS5bhG3aT9sEFifVtSOAFCDEDYNAh3TUvYsZrocFYdBvJ0Trdnj1jRuQzy9Q6YroP2Cwgk4v7q3vbeZBikz8Q7vB+RbDPsWUy+ZiqdkkeG4XoUKQwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CC0GVRA14","exception":"gas limit exceeded"}}`,
			result: func(c *Client) interface{} {
				return &result.Invoke{}
			},
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.(*result.Invoke)
				require.True(t, ok)
				bytes, err := hex.DecodeString("262bec084432")
				if err != nil {
					panic(err)
				}
				script, err := base64.StdEncoding.DecodeString("FCaufGyYYexBhGjB8P3Ep/KWPriRUcEJYmFsYW5jZU9mZ74557Vi9gy/4q68o3Wi5e4oc3yv")
				if err != nil {
					panic(err)
				}
				assert.Equal(t, "FAULT", res.State)
				assert.Equal(t, int64(31100000), res.GasConsumed)
				assert.Equal(t, script, res.Script)
				assert.Equal(t, []stackitem.Item{stackitem.NewByteArray(bytes)}, res.Stack)
				assert.NotNil(t, res.Transaction)
			},
		},
	},
	"invokescript": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				script, err := base64.StdEncoding.DecodeString("AARuYW1lZyQFjl4bYAiEfNZicoVJCIqe6CGR")
				if err != nil {
					panic(err)
				}
				return c.InvokeScript(script, []transaction.Signer{{
					Account: util.Uint160{1, 2, 3},
				}})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"AARuYW1lZyQFjl4bYAiEfNZicoVJCIqe6CGR","state":"HALT","gasconsumed":"16100000","stack":[{"type":"ByteString","value":"TkVQNSBHQVM="}],"tx":null}}`,
			result: func(c *Client) interface{} {
				bytes, err := hex.DecodeString("4e45503520474153")
				if err != nil {
					panic(err)
				}
				script, err := base64.StdEncoding.DecodeString("AARuYW1lZyQFjl4bYAiEfNZicoVJCIqe6CGR")
				if err != nil {
					panic(err)
				}
				return &result.Invoke{
					State:       "HALT",
					GasConsumed: 16100000,
					Script:      script,
					Stack:       []stackitem.Item{stackitem.NewByteArray(bytes)},
				}
			},
		},
	},
	"invokecontractverify": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				contr, err := util.Uint160DecodeStringLE("af7c7328eee5a275a3bcaee2bf0cf662b5e739be")
				if err != nil {
					panic(err)
				}
				return c.InvokeContractVerify(contr, nil, []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}, transaction.Witness{InvocationScript: []byte{1, 2, 3}})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"FCaufGyYYexBhGjB8P3Ep/KWPriRUcEJYmFsYW5jZU9mZ74557Vi9gy/4q68o3Wi5e4oc3yv","state":"HALT","gasconsumed":"31100000","stack":[{"type":"ByteString","value":"JivsCEQy"}],"tx":"AAgAAACAlpgAAAAAAAIEEwAAAAAAsAQAAAGqis+FnU/kArNOZz8hVoIXlqSI6wEAVwHoAwwUqorPhZ1P5AKzTmc/IVaCF5akiOsMFOeetm08E0pKd27oB9LluEbdpP2wE8AMCHRyYW5zZmVyDBTnnrZtPBNKSndu6AfS5bhG3aT9sEFifVtSOAFCDEDYNAh3TUvYsZrocFYdBvJ0Trdnj1jRuQzy9Q6YroP2Cwgk4v7q3vbeZBikz8Q7vB+RbDPsWUy+ZiqdkkeG4XoUKQwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CC0GVRA14"}}`,
			result: func(c *Client) interface{} {
				return &result.Invoke{}
			},
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.(*result.Invoke)
				require.True(t, ok)
				bytes, err := hex.DecodeString("262bec084432")
				if err != nil {
					panic(err)
				}
				script, err := base64.StdEncoding.DecodeString("FCaufGyYYexBhGjB8P3Ep/KWPriRUcEJYmFsYW5jZU9mZ74557Vi9gy/4q68o3Wi5e4oc3yv")
				if err != nil {
					panic(err)
				}
				assert.Equal(t, "HALT", res.State)
				assert.Equal(t, int64(31100000), res.GasConsumed)
				assert.Equal(t, script, res.Script)
				assert.Equal(t, []stackitem.Item{stackitem.NewByteArray(bytes)}, res.Stack)
				assert.NotNil(t, res.Transaction)
			},
		},
		{
			name: "bad witness number",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeContractVerify(util.Uint160{}, nil, []transaction.Signer{{}}, []transaction.Witness{{}, {}}...)
			},
			fails: true,
		},
	},
	"sendrawtransaction": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.SendRawTransaction(transaction.New([]byte{byte(opcode.PUSH1)}, 0))
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"hash":"0x72159b0cf1221110daad6e1df6ef4ff03012173b63c86910bd7134deb659c875"}}`,
			result: func(c *Client) interface{} {
				h, err := util.Uint256DecodeStringLE("72159b0cf1221110daad6e1df6ef4ff03012173b63c86910bd7134deb659c875")
				if err != nil {
					panic(fmt.Errorf("can't decode `sendrawtransaction` result hash: %w", err))
				}
				return h
			},
		},
	},
	"submitblock": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.SubmitBlock(block.Block{
					Header:       block.Header{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"hash":"0x1bdea8f80eb5bd97fade38d5e7fb93b02c9d3e01394e9f4324218132293f7ea6"}}`,
			result: func(c *Client) interface{} {
				h, err := util.Uint256DecodeStringLE("1bdea8f80eb5bd97fade38d5e7fb93b02c9d3e01394e9f4324218132293f7ea6")
				if err != nil {
					panic(fmt.Errorf("can't decode `submitblock` result hash: %w", err))
				}
				return h
			},
		},
	},
	"validateaddress": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"address":"AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i","isvalid":true}}`,
			result: func(c *Client) interface{} {
				// no error expected
				return nil
			},
		},
	},
}

type rpcClientErrorCase struct {
	name   string
	invoke func(c *Client) (interface{}, error)
}

var rpcClientErrorCases = map[string][]rpcClientErrorCase{
	`{"jsonrpc":"2.0","id":1,"result":{"name":"name","bad":42}}`: {
		{
			name: "getnep11properties_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP11Properties(util.Uint160{}, []byte{})
			},
		},
	},
	`{"jsonrpc":"2.0","id":1,"result":{"name":100500,"good":"c29tZXRoaW5n"}}`: {
		{
			name: "getnep11properties_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP11Properties(util.Uint160{}, []byte{})
			},
		},
	},
	`{"jsonrpc":"2.0","id":1,"result":"not-a-hex-string"}`: {
		{
			name: "getblock_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblockheader_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
		},
		{
			name: "getrawtransaction_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
		},
		{
			name: "getstoragebyhash_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("03febccf81ac85e3d795bc5cbd4e84e907812aa3")
				if err != nil {
					panic(err)
				}
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorageByHash(hash, key)
			},
		},
		{
			name: "getstoragebyid_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorageByID(-1, key)
			},
		},
	},
	`{"jsonrpc":"2.0","id":1,"result":"01"}`: {
		{
			name: "getblock_decodebin_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getheader_decodebin_err",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
		},
		{
			name: "getrawtransaction_decodebin_err",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
		},
	},
	`{"jsonrpc":"2.0","id":1,"result":false}`: {
		{
			name: "sendrawtransaction_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return c.SendRawTransaction(transaction.New([]byte{byte(opcode.PUSH1)}, 0))
			},
		},
		{
			name: "submitblock_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return c.SubmitBlock(block.Block{
					Header:       block.Header{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
		},
		{
			name: "validateaddress_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i")
			},
		},
	},
	`{"id":1,"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"}}`: {
		{
			name: "getapplicationlog_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{}, nil)
			},
		},
		{
			name: "getbestblockhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
		},
		{
			name: "getblock_byindex_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblock_byindex_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndexVerbose(1)
			},
		},
		{
			name: "getblock_byhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHash(util.Uint256{})
			},
		},
		{
			name: "getblock_byhash_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHashVerbose(util.Uint256{})
			},
		},
		{
			name: "getblockhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(0)
			},
		},
		{
			name: "getblockheader_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeader(util.Uint256{})
			},
		},
		{
			name: "getblockheader_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeaderVerbose(util.Uint256{})
			},
		},
		{
			name: "getblocksysfee_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
		},
		{
			name: "getconnectioncount_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
		},
		{
			name: "getcontractstate_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractStateByHash(util.Uint160{})
			},
		},
		{
			name: "getnep11balances_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP11Balances(util.Uint160{})
			},
		},
		{
			name: "getnep17balances_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP17Balances(util.Uint160{})
			},
		},
		{
			name: "getnep11properties_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP11Properties(util.Uint160{}, []byte{})
			},
		},
		{
			name: "getnep11transfers_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP11Transfers(util.Uint160{}, nil, nil, nil, nil)
			},
		},
		{
			name: "getnep17transfers_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP17Transfers(util.Uint160{}, nil, nil, nil, nil)
			},
		},
		{
			name: "getnep17transfers_invalid_params_error 2",
			invoke: func(c *Client) (interface{}, error) {
				var stop uint64
				return c.GetNEP17Transfers(util.Uint160{}, nil, &stop, nil, nil)
			},
		},
		{
			name: "getnep17transfers_invalid_params_error 3",
			invoke: func(c *Client) (interface{}, error) {
				var start uint64
				var limit int
				return c.GetNEP17Transfers(util.Uint160{}, &start, nil, &limit, nil)
			},
		},
		{
			name: "getnep17transfers_invalid_params_error 4",
			invoke: func(c *Client) (interface{}, error) {
				var start, stop uint64
				var page int
				return c.GetNEP17Transfers(util.Uint160{}, &start, &stop, nil, &page)
			},
		},
		{
			name: "getrawtransaction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransaction(util.Uint256{})
			},
		},
		{
			name: "getrawtransaction_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransactionVerbose(util.Uint256{})
			},
		},
		{
			name: "getstoragebyhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorageByHash(util.Uint160{}, []byte{})
			},
		},
		{
			name: "getstoragebyid_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorageByID(-1, []byte{})
			},
		},
		{
			name: "gettransactionheight_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetTransactionHeight(util.Uint256{})
			},
		},
		{
			name: "getunclaimedgas_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimedGas("")
			},
		},
		{
			name: "invokefunction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeFunction(util.Uint160{}, "", []smartcontract.Parameter{}, nil)
			},
		},
		{
			name: "invokescript_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript([]byte{}, nil)
			},
		},
		{
			name: "sendrawtransaction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.SendRawTransaction(&transaction.Transaction{})
			},
		},
		{
			name: "submitblock_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.SubmitBlock(block.Block{})
			},
		},
		{
			name: "validateaddress_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("")
			},
		},
	},
	`{}`: {
		{
			name: "getapplicationlog_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{}, nil)
			},
		},
		{
			name: "getbestblockhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
		},
		{
			name: "getblock_byindex_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblock_byindex_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndexVerbose(1)
			},
		},
		{
			name: "getblock_byhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHash(util.Uint256{})
			},
		},
		{
			name: "getblock_byhash_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHashVerbose(util.Uint256{})
			},
		},
		{
			name: "getblockcount_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockCount()
			},
		},
		{
			name: "getblockhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(1)
			},
		},
		{
			name: "getblockheader_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeader(util.Uint256{})
			},
		},
		{
			name: "getblockheader_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeaderVerbose(util.Uint256{})
			},
		},
		{
			name: "getblocksysfee_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
		},
		{
			name: "getcommittee_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetCommittee()
			},
		},
		{
			name: "getconnectioncount_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
		},
		{
			name: "getcontractstate_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractStateByHash(util.Uint160{})
			},
		},
		{
			name: "getnep11balances_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP11Balances(util.Uint160{})
			},
		},
		{
			name: "getnep17balances_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP17Balances(util.Uint160{})
			},
		},
		{
			name: "getnep11transfers_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP11Transfers(util.Uint160{}, nil, nil, nil, nil)
			},
		},
		{
			name: "getnep17transfers_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP17Transfers(util.Uint160{}, nil, nil, nil, nil)
			},
		},
		{
			name: "getpeers_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetPeers()
			},
		},
		{
			name: "getrawmempool_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawMemPool()
			},
		},
		{
			name: "getrawtransaction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransaction(util.Uint256{})
			},
		},
		{
			name: "getrawtransaction_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransactionVerbose(util.Uint256{})
			},
		},
		{
			name: "getstoragebyhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorageByHash(util.Uint160{}, []byte{})
			},
		},
		{
			name: "getstoragebyid_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorageByID(-1, []byte{})
			},
		},
		{
			name: "gettransactionheight_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetTransactionHeight(util.Uint256{})
			},
		},
		{
			name: "getunclaimedgas_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimedGas("")
			},
		},
		{
			name: "getcandidates_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetCandidates()
			},
		},
		{
			name: "getvalidators_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNextBlockValidators()
			},
		},
		{
			name: "invokefunction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeFunction(util.Uint160{}, "", []smartcontract.Parameter{}, nil)
			},
		},
		{
			name: "invokescript_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript([]byte{}, nil)
			},
		},
		{
			name: "sendrawtransaction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.SendRawTransaction(transaction.New([]byte{byte(opcode.PUSH1)}, 0))
			},
		},
		{
			name: "submitblock_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.SubmitBlock(block.Block{
					Header:       block.Header{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
		},
		{
			name: "validateaddress_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("")
			},
		},
	},
}

func TestRPCClients(t *testing.T) {
	t.Run("Client", func(t *testing.T) {
		testRPCClient(t, func(ctx context.Context, endpoint string, opts Options) (*Client, error) {
			c, err := New(ctx, endpoint, opts)
			require.NoError(t, err)
			c.getNextRequestID = getTestRequestID
			require.NoError(t, c.Init())
			return c, nil
		})
	})
	t.Run("WSClient", func(t *testing.T) {
		testRPCClient(t, func(ctx context.Context, endpoint string, opts Options) (*Client, error) {
			wsc, err := NewWS(ctx, httpURLtoWS(endpoint), opts)
			require.NoError(t, err)
			wsc.getNextRequestID = getTestRequestID
			require.NoError(t, wsc.Init())
			return &wsc.Client, nil
		})
	})
}

func testRPCClient(t *testing.T, newClient func(context.Context, string, Options) (*Client, error)) {
	for method, testBatch := range rpcClientTestCases {
		t.Run(method, func(t *testing.T) {
			for _, testCase := range testBatch {
				t.Run(testCase.name, func(t *testing.T) {
					srv := initTestServer(t, testCase.serverResponse)

					endpoint := srv.URL
					opts := Options{}
					c, err := newClient(context.TODO(), endpoint, opts)
					if err != nil {
						t.Fatal(err)
					}
					c.getNextRequestID = getTestRequestID

					actual, err := testCase.invoke(c)
					if testCase.fails {
						assert.Error(t, err)
					} else {
						assert.NoError(t, err)

						expected := testCase.result(c)
						if testCase.check == nil {
							assert.Equal(t, expected, actual)
						} else {
							testCase.check(t, c, actual)
						}
					}
				})
			}
		})
	}
	for serverResponse, testBatch := range rpcClientErrorCases {
		srv := initTestServer(t, serverResponse)

		endpoint := srv.URL
		opts := Options{}
		for _, testCase := range testBatch {
			t.Run(testCase.name, func(t *testing.T) {
				c, err := newClient(context.TODO(), endpoint, opts)
				if err != nil {
					t.Fatal(err)
				}
				c.getNextRequestID = getTestRequestID
				_, err = testCase.invoke(c)
				assert.Error(t, err)
			})
		}
	}
}

func httpURLtoWS(url string) string {
	return "ws" + strings.TrimPrefix(url, "http") + "/ws"
}

func initTestServer(t *testing.T, resp string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/ws" && req.Method == "GET" {
			var upgrader = websocket.Upgrader{}
			ws, err := upgrader.Upgrade(w, req, nil)
			require.NoError(t, err)
			for {
				err = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
				require.NoError(t, err)
				_, p, err := ws.ReadMessage()
				if err != nil {
					break
				}
				r := params.NewIn()
				err = json.Unmarshal(p, r)
				if err != nil {
					t.Fatalf("Cannot decode request body: %s", req.Body)
				}
				response := wrapInitResponse(r, resp)
				err = ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
				require.NoError(t, err)
				err = ws.WriteMessage(1, []byte(response))
				if err != nil {
					break
				}
			}
			ws.Close()
			return
		}
		r := params.NewRequest()
		err := r.DecodeData(req.Body)
		if err != nil {
			t.Fatalf("Cannot decode request body: %s", req.Body)
		}
		requestHandler(t, r.In, w, resp)
	}))

	t.Cleanup(srv.Close)

	return srv
}

func requestHandler(t *testing.T, r *params.In, w http.ResponseWriter, resp string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	response := wrapInitResponse(r, resp)
	_, err := w.Write([]byte(response))
	if err != nil {
		t.Fatalf("Error writing response: %s", err.Error())
	}
}

func wrapInitResponse(r *params.In, resp string) string {
	var response string
	switch r.Method {
	case "getversion":
		response = `{"id":1,"jsonrpc":"2.0","result":{"protocol":{"network":42},"tcpport":20332,"wsport":20342,"nonce":2153672787,"useragent":"/NEO-GO:0.73.1-pre-273-ge381358/"}}`
	case "getnativecontracts":
		response = `{"jsonrpc":"2.0","id":1,"result":[{"id":-1,"hash":"0xfffdc93764dbaddd97c48f252a53ea4643faa3fd","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":1110259869},"manifest":{"name":"ContractManagement","groups":[],"features":{},"supportedstandards":[],"abi":{"methods":[{"name":"deploy","parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"}],"returntype":"Array","offset":0,"safe":false},{"name":"deploy","parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"},{"name":"data","type":"Any"}],"returntype":"Array","offset":7,"safe":false},{"name":"destroy","parameters":[],"returntype":"Void","offset":14,"safe":false},{"name":"getContract","parameters":[{"name":"hash","type":"Hash160"}],"returntype":"Array","offset":21,"safe":true},{"name":"getMinimumDeploymentFee","parameters":[],"returntype":"Integer","offset":28,"safe":true},{"name":"setMinimumDeploymentFee","parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","offset":35,"safe":false},{"name":"update","parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"}],"returntype":"Void","offset":42,"safe":false},{"name":"update","parameters":[{"name":"nefFile","type":"ByteArray"},{"name":"manifest","type":"ByteArray"},{"name":"data","type":"Any"}],"returntype":"Void","offset":49,"safe":false}],"events":[{"name":"Deploy","parameters":[{"name":"Hash","type":"Hash160"}]},{"name":"Update","parameters":[{"name":"Hash","type":"Hash160"}]},{"name":"Destroy","parameters":[{"name":"Hash","type":"Hash160"}]}]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null},"updatehistory":[0]},{"id":-2,"hash":"0xacce6fd80d44e1796aa0c2c625e9e4e0ce39efc0","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":1325686241},"manifest":{"name":"StdLib","groups":[],"features":{},"supportedstandards":[],"abi":{"methods":[{"name":"atoi","parameters":[{"name":"value","type":"String"}],"returntype":"Integer","offset":0,"safe":true},{"name":"atoi","parameters":[{"name":"value","type":"String"},{"name":"base","type":"Integer"}],"returntype":"Integer","offset":7,"safe":true},{"name":"base58CheckDecode","parameters":[{"name":"s","type":"String"}],"returntype":"ByteArray","offset":14,"safe":true},{"name":"base58CheckEncode","parameters":[{"name":"data","type":"ByteArray"}],"returntype":"String","offset":21,"safe":true},{"name":"base58Decode","parameters":[{"name":"s","type":"String"}],"returntype":"ByteArray","offset":28,"safe":true},{"name":"base58Encode","parameters":[{"name":"data","type":"ByteArray"}],"returntype":"String","offset":35,"safe":true},{"name":"base64Decode","parameters":[{"name":"s","type":"String"}],"returntype":"ByteArray","offset":42,"safe":true},{"name":"base64Encode","parameters":[{"name":"data","type":"ByteArray"}],"returntype":"String","offset":49,"safe":true},{"name":"deserialize","parameters":[{"name":"data","type":"ByteArray"}],"returntype":"Any","offset":56,"safe":true},{"name":"itoa","parameters":[{"name":"value","type":"Integer"}],"returntype":"String","offset":63,"safe":true},{"name":"itoa","parameters":[{"name":"value","type":"Integer"},{"name":"base","type":"Integer"}],"returntype":"String","offset":70,"safe":true},{"name":"jsonDeserialize","parameters":[{"name":"json","type":"ByteArray"}],"returntype":"Any","offset":77,"safe":true},{"name":"jsonSerialize","parameters":[{"name":"item","type":"Any"}],"returntype":"ByteArray","offset":84,"safe":true},{"name":"memoryCompare","parameters":[{"name":"str1","type":"ByteArray"},{"name":"str2","type":"ByteArray"}],"returntype":"Integer","offset":91,"safe":true},{"name":"memorySearch","parameters":[{"name":"mem","type":"ByteArray"},{"name":"value","type":"ByteArray"}],"returntype":"Integer","offset":98,"safe":true},{"name":"memorySearch","parameters":[{"name":"mem","type":"ByteArray"},{"name":"value","type":"ByteArray"},{"name":"start","type":"Integer"}],"returntype":"Integer","offset":105,"safe":true},{"name":"memorySearch","parameters":[{"name":"mem","type":"ByteArray"},{"name":"value","type":"ByteArray"},{"name":"start","type":"Integer"},{"name":"backward","type":"Boolean"}],"returntype":"Integer","offset":112,"safe":true},{"name":"serialize","parameters":[{"name":"item","type":"Any"}],"returntype":"ByteArray","offset":119,"safe":true},{"name":"stringSplit","parameters":[{"name":"str","type":"String"},{"name":"separator","type":"String"}],"returntype":"Array","offset":126,"safe":true},{"name":"stringSplit","parameters":[{"name":"str","type":"String"},{"name":"separator","type":"String"},{"name":"removeEmptyEntries","type":"Boolean"}],"returntype":"Array","offset":133,"safe":true}],"events":[]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null},"updatehistory":[0]},{"id":-3,"hash":"0x726cb6e0cd8628a1350a611384688911ab75f51b","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dA","checksum":1592866325},"manifest":{"name":"CryptoLib","groups":[],"features":{},"supportedstandards":[],"abi":{"methods":[{"name":"ripemd160","parameters":[{"name":"data","type":"ByteArray"}],"returntype":"ByteArray","offset":0,"safe":true},{"name":"sha256","parameters":[{"name":"data","type":"ByteArray"}],"returntype":"ByteArray","offset":7,"safe":true},{"name":"verifyWithECDsa","parameters":[{"name":"message","type":"ByteArray"},{"name":"pubkey","type":"ByteArray"},{"name":"signature","type":"ByteArray"},{"name":"curve","type":"Integer"}],"returntype":"Boolean","offset":14,"safe":true}],"events":[]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null},"updatehistory":[0]},{"id":-4,"hash":"0xda65b600f7124ce6c79950c1772a36403104f2be","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dA","checksum":529571427},"manifest":{"name":"LedgerContract","groups":[],"features":{},"supportedstandards":[],"abi":{"methods":[{"name":"currentHash","parameters":[],"returntype":"Hash256","offset":0,"safe":true},{"name":"currentIndex","parameters":[],"returntype":"Integer","offset":7,"safe":true},{"name":"getBlock","parameters":[{"name":"indexOrHash","type":"ByteArray"}],"returntype":"Array","offset":14,"safe":true},{"name":"getTransaction","parameters":[{"name":"hash","type":"Hash256"}],"returntype":"Array","offset":21,"safe":true},{"name":"getTransactionFromBlock","parameters":[{"name":"blockIndexOrHash","type":"ByteArray"},{"name":"txIndex","type":"Integer"}],"returntype":"Array","offset":28,"safe":true},{"name":"getTransactionHeight","parameters":[{"name":"hash","type":"Hash256"}],"returntype":"Integer","offset":35,"safe":true}],"events":[]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null},"updatehistory":[0]},{"id":-5,"hash":"0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":588003825},"manifest":{"name":"NeoToken","groups":[],"features":{},"supportedstandards":["NEP-17"],"abi":{"methods":[{"name":"balanceOf","parameters":[{"name":"account","type":"Hash160"}],"returntype":"Integer","offset":0,"safe":true},{"name":"decimals","parameters":[],"returntype":"Integer","offset":7,"safe":true},{"name":"getAccountState","parameters":[{"name":"account","type":"Hash160"}],"returntype":"Array","offset":14,"safe":true},{"name":"getCandidates","parameters":[],"returntype":"Array","offset":21,"safe":true},{"name":"getCommittee","parameters":[],"returntype":"Array","offset":28,"safe":true},{"name":"getGasPerBlock","parameters":[],"returntype":"Integer","offset":35,"safe":true},{"name":"getNextBlockValidators","parameters":[],"returntype":"Array","offset":42,"safe":true},{"name":"getRegisterPrice","parameters":[],"returntype":"Integer","offset":49,"safe":true},{"name":"registerCandidate","parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean","offset":56,"safe":false},{"name":"setGasPerBlock","parameters":[{"name":"gasPerBlock","type":"Integer"}],"returntype":"Void","offset":63,"safe":false},{"name":"setRegisterPrice","parameters":[{"name":"registerPrice","type":"Integer"}],"returntype":"Void","offset":70,"safe":false},{"name":"symbol","parameters":[],"returntype":"String","offset":77,"safe":true},{"name":"totalSupply","parameters":[],"returntype":"Integer","offset":84,"safe":true},{"name":"transfer","parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"},{"name":"data","type":"Any"}],"returntype":"Boolean","offset":91,"safe":false},{"name":"unclaimedGas","parameters":[{"name":"account","type":"Hash160"},{"name":"end","type":"Integer"}],"returntype":"Integer","offset":98,"safe":true},{"name":"unregisterCandidate","parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean","offset":105,"safe":false},{"name":"vote","parameters":[{"name":"account","type":"Hash160"},{"name":"voteTo","type":"PublicKey"}],"returntype":"Boolean","offset":112,"safe":false}],"events":[{"name":"Transfer","parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"}]}]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null},"updatehistory":[0]},{"id":-6,"hash":"0xd2a4cff31913016155e38e474a2c06d08be276cf","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":2663858513},"manifest":{"name":"GasToken","groups":[],"features":{},"supportedstandards":["NEP-17"],"abi":{"methods":[{"name":"balanceOf","parameters":[{"name":"account","type":"Hash160"}],"returntype":"Integer","offset":0,"safe":true},{"name":"decimals","parameters":[],"returntype":"Integer","offset":7,"safe":true},{"name":"symbol","parameters":[],"returntype":"String","offset":14,"safe":true},{"name":"totalSupply","parameters":[],"returntype":"Integer","offset":21,"safe":true},{"name":"transfer","parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"},{"name":"data","type":"Any"}],"returntype":"Boolean","offset":28,"safe":false}],"events":[{"name":"Transfer","parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"}]}]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null},"updatehistory":[0]},{"id":-7,"hash":"0xcc5e4edd9f5f8dba8bb65734541df7a1c081c67b","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0AQQRr3e2dA","checksum":3443651689},"manifest":{"name":"PolicyContract","groups":[],"features":{},"supportedstandards":[],"abi":{"methods":[{"name":"blockAccount","parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean","offset":0,"safe":false},{"name":"getExecFeeFactor","parameters":[],"returntype":"Integer","offset":7,"safe":true},{"name":"getFeePerByte","parameters":[],"returntype":"Integer","offset":14,"safe":true},{"name":"getStoragePrice","parameters":[],"returntype":"Integer","offset":21,"safe":true},{"name":"isBlocked","parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean","offset":28,"safe":true},{"name":"setExecFeeFactor","parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","offset":35,"safe":false},{"name":"setFeePerByte","parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","offset":42,"safe":false},{"name":"setStoragePrice","parameters":[{"name":"value","type":"Integer"}],"returntype":"Void","offset":49,"safe":false},{"name":"unblockAccount","parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean","offset":56,"safe":false}],"events":[]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null},"updatehistory":[0]},{"id":-8,"hash":"0x49cf4e5378ffcd4dec034fd98a174c5491e395e2","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0A=","checksum":983638438},"manifest":{"name":"RoleManagement","groups":[],"features":{},"supportedstandards":[],"abi":{"methods":[{"name":"designateAsRole","parameters":[{"name":"role","type":"Integer"},{"name":"nodes","type":"Array"}],"returntype":"Void","offset":0,"safe":false},{"name":"getDesignatedByRole","parameters":[{"name":"role","type":"Integer"},{"name":"index","type":"Integer"}],"returntype":"Array","offset":7,"safe":true}],"events":[{"name":"Designation","parameters":[{"name":"Role","type":"Integer"},{"name":"BlockIndex","type":"Integer"}]}]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null},"updatehistory":[0]},{"id":-9,"hash":"0xfe924b7cfe89ddd271abaf7210a80a7e11178758","nef":{"magic":860243278,"compiler":"neo-core-v3.0","source":"","tokens":[],"script":"EEEa93tnQBBBGvd7Z0AQQRr3e2dAEEEa93tnQBBBGvd7Z0A=","checksum":2663858513},"manifest":{"name":"OracleContract","groups":[],"features":{},"supportedstandards":[],"abi":{"methods":[{"name":"finish","parameters":[],"returntype":"Void","offset":0,"safe":false},{"name":"getPrice","parameters":[],"returntype":"Integer","offset":7,"safe":true},{"name":"request","parameters":[{"name":"url","type":"String"},{"name":"filter","type":"String"},{"name":"callback","type":"String"},{"name":"userData","type":"Any"},{"name":"gasForResponse","type":"Integer"}],"returntype":"Void","offset":14,"safe":false},{"name":"setPrice","parameters":[{"name":"price","type":"Integer"}],"returntype":"Void","offset":21,"safe":false},{"name":"verify","parameters":[],"returntype":"Boolean","offset":28,"safe":true}],"events":[{"name":"OracleRequest","parameters":[{"name":"Id","type":"Integer"},{"name":"RequestContract","type":"Hash160"},{"name":"Url","type":"String"},{"name":"Filter","type":"String"}]},{"name":"OracleResponse","parameters":[{"name":"Id","type":"Integer"},{"name":"OriginalTx","type":"Hash256"}]}]},"permissions":[{"contract":"*","methods":"*"}],"trusts":[],"extra":null},"updatehistory":[0]}]}`
	case "getcontractstate":
		p := params.Params(r.RawParams)
		name, _ := p.Value(0).GetString()
		switch name {
		case "NeoToken":
			response = `{"id":1,"jsonrpc":"2.0","result":{"id":-1,"script":"DANORU9Ba2d4Cw==","manifest":{"name":"NEO","abi":{"hash":"0xde5f57d430d3dece511cf975a8d37848cb9e0525","methods":[{"name":"name","offset":0,"parameters":null,"returntype":"String"},{"name":"symbol","offset":0,"parameters":null,"returntype":"String"},{"name":"decimals","offset":0,"parameters":null,"returntype":"Integer"},{"name":"totalSupply","offset":0,"parameters":null,"returntype":"Integer"},{"name":"balanceOf","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Integer"},{"name":"transfer","offset":0,"parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"}],"returntype":"Boolean"},{"name":"onPersist","offset":0,"parameters":null,"returntype":"Void"},{"name":"postPersist","offset":0,"parameters":null,"returntype":"Void"},{"name":"unclaimedGas","offset":0,"parameters":[{"name":"account","type":"Hash160"},{"name":"end","type":"Integer"}],"returntype":"Integer"},{"name":"registerCandidate","offset":0,"parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean"},{"name":"unregisterCandidate","offset":0,"parameters":[{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean"},{"name":"vote","offset":0,"parameters":[{"name":"account","type":"Hash160"},{"name":"pubkey","type":"PublicKey"}],"returntype":"Boolean"},{"name":"getCandidates","offset":0,"parameters":null,"returntype":"Array"},{"name":"getСommittee","offset":0,"parameters":null,"returntype":"Array"},{"name":"getNextBlockValidators","offset":0,"parameters":null,"returntype":"Array"},{"name":"getGasPerBlock","offset":0,"parameters":null,"returntype":"Integer"},{"name":"setGasPerBlock","offset":0,"parameters":[{"name":"gasPerBlock","type":"Integer"}],"returntype":"Boolean"}],"events":[{"name":"Transfer","parameters":null}]},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":["NEP-5"],"trusts":[],"safemethods":["name","symbol","decimals","totalSupply","balanceOf","unclaimedGas","getCandidates","getСommittee","getNextBlockValidators"],"extra":null},"hash":"0xde5f57d430d3dece511cf975a8d37848cb9e0525"}}`
		case "GasToken":
			response = `{"id":1,"jsonrpc":"2.0","result":{"id":-2,"script":"DANHQVNBa2d4Cw==","manifest":{"name":"GAS","abi":{"hash":"0x668e0c1f9d7b70a99dd9e06eadd4c784d641afbc","methods":[{"name":"name","offset":0,"parameters":null,"returntype":"String"},{"name":"symbol","offset":0,"parameters":null,"returntype":"String"},{"name":"decimals","offset":0,"parameters":null,"returntype":"Integer"},{"name":"totalSupply","offset":0,"parameters":null,"returntype":"Integer"},{"name":"balanceOf","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Integer"},{"name":"transfer","offset":0,"parameters":[{"name":"from","type":"Hash160"},{"name":"to","type":"Hash160"},{"name":"amount","type":"Integer"}],"returntype":"Boolean"},{"name":"onPersist","offset":0,"parameters":null,"returntype":"Void"},{"name":"postPersist","offset":0,"parameters":null,"returntype":"Void"}],"events":[{"name":"Transfer","parameters":null}]},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":["NEP-5"],"trusts":[],"safemethods":["name","symbol","decimals","totalSupply","balanceOf"],"extra":null},"hash":"0x668e0c1f9d7b70a99dd9e06eadd4c784d641afbc"}}`
		case "PolicyContract":
			response = `{"id":1,"jsonrpc":"2.0","result":{"id":-3,"updatecounter":0,"hash":"0xac593e6183643940a9193f87c64ccf55ef19c529","script":"DAZQb2xpY3lBGvd7Zw==","manifest":{"name":"Policy","abi":{"methods":[{"name":"getMaxTransactionsPerBlock","offset":0,"parameters":null,"returntype":"Integer"},{"name":"getMaxBlockSize","offset":0,"parameters":null,"returntype":"Integer"},{"name":"getFeePerByte","offset":0,"parameters":null,"returntype":"Integer"},{"name":"isBlocked","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean"},{"name":"getMaxBlockSystemFee","offset":0,"parameters":null,"returntype":"Integer"},{"name":"setMaxBlockSize","offset":0,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Boolean"},{"name":"setMaxTransactionsPerBlock","offset":0,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Boolean"},{"name":"setFeePerByte","offset":0,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Boolean"},{"name":"setMaxBlockSystemFee","offset":0,"parameters":[{"name":"value","type":"Integer"}],"returntype":"Boolean"},{"name":"blockAccount","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean"},{"name":"unblockAccount","offset":0,"parameters":[{"name":"account","type":"Hash160"}],"returntype":"Boolean"}],"events":[]},"groups":[],"permissions":[{"contract":"*","methods":"*"}],"supportedstandards":[],"trusts":[],"safemethods":["getMaxTransactionsPerBlock","getMaxBlockSize","getFeePerByte","isBlocked","getMaxBlockSystemFee"],"extra":null}}}`
		default:
			response = resp
		}
	default:
		response = resp
	}
	return response
}

func TestCalculateValidUntilBlock(t *testing.T) {
	var (
		getBlockCountCalled int
		getValidatorsCalled int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r := params.NewRequest()
		err := r.DecodeData(req.Body)
		if err != nil {
			t.Fatalf("Cannot decode request body: %s", req.Body)
		}
		var response string
		switch r.In.Method {
		case "getblockcount":
			getBlockCountCalled++
			response = `{"jsonrpc":"2.0","id":1,"result":50}`
		case "getnextblockvalidators":
			getValidatorsCalled++
			response = `{"id":1,"jsonrpc":"2.0","result":[{"publickey":"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2","votes":"0","active":true},{"publickey":"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e","votes":"0","active":true},{"publickey":"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699","votes":"0","active":true},{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"0","active":true}]}`
		}
		requestHandler(t, r.In, w, response)
	}))
	t.Cleanup(srv.Close)

	endpoint := srv.URL
	opts := Options{}
	c, err := New(context.TODO(), endpoint, opts)
	if err != nil {
		t.Fatal(err)
	}
	c.getNextRequestID = getTestRequestID
	require.NoError(t, c.Init())

	validUntilBlock, err := c.CalculateValidUntilBlock()
	assert.NoError(t, err)
	assert.Equal(t, uint32(55), validUntilBlock)
	assert.Equal(t, 1, getBlockCountCalled)
	assert.Equal(t, 1, getValidatorsCalled)

	// check, whether caching is working
	validUntilBlock, err = c.CalculateValidUntilBlock()
	assert.NoError(t, err)
	assert.Equal(t, uint32(55), validUntilBlock)
	assert.Equal(t, 2, getBlockCountCalled)
	assert.Equal(t, 1, getValidatorsCalled)
}

func TestGetNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r := params.NewRequest()
		err := r.DecodeData(req.Body)
		if err != nil {
			t.Fatalf("Cannot decode request body: %s", req.Body)
		}
		// request handler already have `getversion` response wrapper
		requestHandler(t, r.In, w, "")
	}))
	t.Cleanup(srv.Close)
	endpoint := srv.URL
	opts := Options{}

	t.Run("bad", func(t *testing.T) {
		c, err := New(context.TODO(), endpoint, opts)
		if err != nil {
			t.Fatal(err)
		}
		c.getNextRequestID = getTestRequestID
		// network was not initialised
		_, err = c.GetNetwork()
		require.True(t, errors.Is(err, errNetworkNotInitialized))
		require.Equal(t, false, c.cache.initDone)
	})

	t.Run("good", func(t *testing.T) {
		c, err := New(context.TODO(), endpoint, opts)
		if err != nil {
			t.Fatal(err)
		}
		c.getNextRequestID = getTestRequestID
		require.NoError(t, c.Init())
		m, err := c.GetNetwork()
		require.NoError(t, err)
		require.Equal(t, netmode.UnitTestNet, m)
	})
}

func TestUninitedClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r := params.NewRequest()
		err := r.DecodeData(req.Body)
		require.NoErrorf(t, err, "Cannot decode request body: %s", req.Body)
		// request handler already have `getversion` response wrapper
		requestHandler(t, r.In, w, "")
	}))
	t.Cleanup(srv.Close)
	endpoint := srv.URL
	opts := Options{}

	c, err := New(context.TODO(), endpoint, opts)
	require.NoError(t, err)
	c.getNextRequestID = getTestRequestID

	_, err = c.GetBlockByIndex(0)
	require.Error(t, err)
	_, err = c.GetBlockByIndexVerbose(0)
	require.Error(t, err)
	_, err = c.GetBlockHeader(util.Uint256{})
	require.Error(t, err)
	_, err = c.GetRawTransaction(util.Uint256{})
	require.Error(t, err)
	_, err = c.GetRawTransactionVerbose(util.Uint256{})
	require.Error(t, err)
	_, err = c.IsBlocked(util.Uint160{})
	require.Error(t, err)
	_, err = c.GetFeePerByte()
	require.Error(t, err)
}

func newTestNEF(script []byte) nef.File {
	var ne nef.File
	ne.Header.Magic = nef.Magic
	ne.Header.Compiler = "neo-go-3.0"
	ne.Script = script
	ne.Checksum = ne.CalculateChecksum()
	return ne
}

func getTestRequestID() uint64 {
	return 1
}
