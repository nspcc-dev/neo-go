package client

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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

const base64B1 = "AAAAAMU1lpLU9L9XS3U0WvRgGV9aU5WoX8f6MWgNjfz89nyeomtq7Iw0SoX9caDTvpAT4ulAxcy/vWK7q9GH0raCqQfxbcftdwEAAAEAAAAAXhK+qHrrlViE9vnOeEWMzLl8MJUBxgxAVLK3uK5qryZv+jBuH0dBn7VU+sYztObj1sj65/az1v2XCrlLlL2z2LeHccRnn7jAXUE0m80q7QAxEWyhzJPA/QxAOCcAytavTTPv0uQ+rhoBRXvyxaaEdSCZq0VDJCNtI4O9iFXq+Q++GJjzA04z4QZo7KCB8KD8aruBc69i6PoqwwxAR7dzN1DAk9G1RCuSZx7X7U/qqJfT7Wa4Us9kq/40AVpJgwr0RNUGWf1Xh8K53f+tzw1UtHZMoI5YZyJtMEiQY5QTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUC0ETje+vAgACAAAAwNinAAAAAAASfUMAAAAAALAEAAABXhK+qHrrlViE9vnOeEWMzLl8MJUBAFsLAhjd9QUMFKqKz4WdT+QCs05nPyFWgheWpIjrDBReEr6oeuuVWIT2+c54RYzMuXwwlRTAHwwIdHJhbnNmZXIMFIOrBnmtVcBQoTrUP1k26nP16x72QWJ9W1I5AcYMQIoQAeuRy5Lgj4MYiuF9tLhAeYhKf6PrczcbKGeWmz+KNWULI+mQbeDPFWG3mGwPkSBELVqVMrUNqigZLflJhNwMQBuqOk8xrVlAx5A5Va9FlMhu3io+wIrubPoWNr0sklBKh48H9w3WHPfTFBSAW8M9ePou/TVXM40X+U07fy+s+8MMQIlw1AKX/fk1rn1GqjZOqNmhVjQPm6Tk7Cb1fzeBa4baIEy5DmaaM8ayh0tm8N3Vc8JNSwnK91vIXPG/A9RKTKuUEwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrwADAAAAwNinAAAAAACyjEMAAAAAALAEAAABXhK+qHrrlViE9vnOeEWMzLl8MJUBAF8LAwDodkgXAAAADBSqis+FnU/kArNOZz8hVoIXlqSI6wwUXhK+qHrrlViE9vnOeEWMzLl8MJUUwB8MCHRyYW5zZmVyDBQos62rcmn5whgds8t0Hr9VGTDicEFifVtSOQHGDEA7aJyGTIq0pV20LzVWOCreh6XIxLUCWHVgUFsCTxPOPdqtZBHKnejng3d2BRm/lecTyPLeq7KpRCD9awRvadFWDEBjVZRvSGtGcOEjtUxl4AH5XelYlIUG5k+x3QyYKZtWQc96lUX1hohrNkCmWeWNwC2l8eJGpUxicM+WZGODCVp8DEDbQxvmqRTQ+flc6JetmaqHyw8rfoeQNtmEFpw2cNhyAo5L5Ilp2wbVtJNOJPfw72J7E6FhTK8slIKRqXzpdnyKlBMMIQIQOn990BZVhZf3lg0nxRakOU/ZaLnmUVXrSwE+QEBAbgwhAqe8Vf6GhOARl2jRBLoweVvcyGYZ6GSt0mFWcj7Rhc1iDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcIMIQPZDAffY+aQzneRLhCrUazJRLZoYCN7YIxPj4MJ5x7mmRQLQRON768="

const base64TxMoveNeo = "AAIAAADA2KcAAAAAABJ9QwAAAAAAsAQAAAFeEr6oeuuVWIT2+c54RYzMuXwwlQEAWwsCGN31BQwUqorPhZ1P5AKzTmc/IVaCF5akiOsMFF4Svqh665VYhPb5znhFjMy5fDCVFMAfDAh0cmFuc2ZlcgwUg6sGea1VwFChOtQ/WTbqc/XrHvZBYn1bUjkBxgxAihAB65HLkuCPgxiK4X20uEB5iEp/o+tzNxsoZ5abP4o1ZQsj6ZBt4M8VYbeYbA+RIEQtWpUytQ2qKBkt+UmE3AxAG6o6TzGtWUDHkDlVr0WUyG7eKj7Aiu5s+hY2vSySUEqHjwf3DdYc99MUFIBbwz14+i79NVczjRf5TTt/L6z7wwxAiXDUApf9+TWufUaqNk6o2aFWNA+bpOTsJvV/N4FrhtogTLkOZpozxrKHS2bw3dVzwk1LCcr3W8hc8b8D1EpMq5QTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUC0ETje+v"

const b1Verbose = `{"id":5,"jsonrpc":"2.0","result":{"size":1433,"nextblockhash":"0x85ab779bc19247aa504c36879ce75cb7f662b4e8067fbc83e5d24ef0afd9a84f","confirmations":6,"hash":"0xea6385e943832b65ee225aaeb31933a97f3362505ab84cfe5dbd91cd1672b9b7","version":0,"previousblockhash":"0x9e7cf6fcfc8d0d6831fac75fa895535a5f1960f45a34754b57bff4d4929635c5","merkleroot":"0x07a982b6d287d1abbb62bdbfccc540e9e21390bed3a071fd854a348cec6a6ba2","time":1614602006001,"index":1,"nextconsensus":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","primary":0,"witnesses":[{"invocation":"DEBUsre4rmqvJm/6MG4fR0GftVT6xjO05uPWyPrn9rPW/ZcKuUuUvbPYt4dxxGefuMBdQTSbzSrtADERbKHMk8D9DEA4JwDK1q9NM+/S5D6uGgFFe/LFpoR1IJmrRUMkI20jg72IVer5D74YmPMDTjPhBmjsoIHwoPxqu4Fzr2Lo+irDDEBHt3M3UMCT0bVEK5JnHtftT+qol9PtZrhSz2Sr/jQBWkmDCvRE1QZZ/VeHwrnd/63PDVS0dkygjlhnIm0wSJBj","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}],"tx":[{"hash":"0x7c10b90077bddfe9095b2db96bb4ac33994ed1ca99c805410f55c771eee0b77b","size":489,"version":0,"nonce":2,"sender":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","sysfee":"11000000","netfee":"4422930","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x95307cb9cc8c4578cef9f6845895eb7aa8be125e","scopes":"CalledByEntry"}],"script":"CwIY3fUFDBSqis+FnU/kArNOZz8hVoIXlqSI6wwUXhK+qHrrlViE9vnOeEWMzLl8MJUUwB8MCHRyYW5zZmVyDBSDqwZ5rVXAUKE61D9ZNupz9ese9kFifVtSOQ==","witnesses":[{"invocation":"DECKEAHrkcuS4I+DGIrhfbS4QHmISn+j63M3Gyhnlps/ijVlCyPpkG3gzxVht5hsD5EgRC1alTK1DaooGS35SYTcDEAbqjpPMa1ZQMeQOVWvRZTIbt4qPsCK7mz6Fja9LJJQSoePB/cN1hz30xQUgFvDPXj6Lv01VzONF/lNO38vrPvDDECJcNQCl/35Na59Rqo2TqjZoVY0D5uk5Owm9X83gWuG2iBMuQ5mmjPGsodLZvDd1XPCTUsJyvdbyFzxvwPUSkyr","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}]},{"hash":"0x41846075f4c5aec54d70b476befb97b35696700454b1168e1ae8888d8fb204a3","size":493,"version":0,"nonce":3,"sender":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","sysfee":"11000000","netfee":"4426930","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x95307cb9cc8c4578cef9f6845895eb7aa8be125e","scopes":"CalledByEntry"}],"script":"CwMA6HZIFwAAAAwUqorPhZ1P5AKzTmc/IVaCF5akiOsMFF4Svqh665VYhPb5znhFjMy5fDCVFMAfDAh0cmFuc2ZlcgwUKLOtq3Jp+cIYHbPLdB6/VRkw4nBBYn1bUjk=","witnesses":[{"invocation":"DEA7aJyGTIq0pV20LzVWOCreh6XIxLUCWHVgUFsCTxPOPdqtZBHKnejng3d2BRm/lecTyPLeq7KpRCD9awRvadFWDEBjVZRvSGtGcOEjtUxl4AH5XelYlIUG5k+x3QyYKZtWQc96lUX1hohrNkCmWeWNwC2l8eJGpUxicM+WZGODCVp8DEDbQxvmqRTQ+flc6JetmaqHyw8rfoeQNtmEFpw2cNhyAo5L5Ilp2wbVtJNOJPfw72J7E6FhTK8slIKRqXzpdnyK","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}]}]}}`

const base64Header1 = "AAAAAMU1lpLU9L9XS3U0WvRgGV9aU5WoX8f6MWgNjfz89nyeomtq7Iw0SoX9caDTvpAT4ulAxcy/vWK7q9GH0raCqQfxbcftdwEAAAEAAAAAXhK+qHrrlViE9vnOeEWMzLl8MJUBxgxAVLK3uK5qryZv+jBuH0dBn7VU+sYztObj1sj65/az1v2XCrlLlL2z2LeHccRnn7jAXUE0m80q7QAxEWyhzJPA/QxAOCcAytavTTPv0uQ+rhoBRXvyxaaEdSCZq0VDJCNtI4O9iFXq+Q++GJjzA04z4QZo7KCB8KD8aruBc69i6PoqwwxAR7dzN1DAk9G1RCuSZx7X7U/qqJfT7Wa4Us9kq/40AVpJgwr0RNUGWf1Xh8K53f+tzw1UtHZMoI5YZyJtMEiQY5QTDCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG4MIQKnvFX+hoTgEZdo0QS6MHlb3MhmGehkrdJhVnI+0YXNYgwhArNiK/QBe9/jF8WK7V9MdT8ga324lgRvp9d0u8S/f43CDCED2QwH32PmkM53kS4Qq1GsyUS2aGAje2CMT4+DCece5pkUC0ETje+vAA=="

const header1Verbose = `{"id":5,"jsonrpc":"2.0","result":{"hash":"0xea6385e943832b65ee225aaeb31933a97f3362505ab84cfe5dbd91cd1672b9b7","size":451,"version":0,"previousblockhash":"0x9e7cf6fcfc8d0d6831fac75fa895535a5f1960f45a34754b57bff4d4929635c5","merkleroot":"0x07a982b6d287d1abbb62bdbfccc540e9e21390bed3a071fd854a348cec6a6ba2","time":1614602006001,"index":1,"nextconsensus":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","witnesses":[{"invocation":"DEBUsre4rmqvJm/6MG4fR0GftVT6xjO05uPWyPrn9rPW/ZcKuUuUvbPYt4dxxGefuMBdQTSbzSrtADERbKHMk8D9DEA4JwDK1q9NM+/S5D6uGgFFe/LFpoR1IJmrRUMkI20jg72IVer5D74YmPMDTjPhBmjsoIHwoPxqu4Fzr2Lo+irDDEBHt3M3UMCT0bVEK5JnHtftT+qol9PtZrhSz2Sr/jQBWkmDCvRE1QZZ/VeHwrnd/63PDVS0dkygjlhnIm0wSJBj","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}],"confirmations":6,"nextblockhash":"0x85ab779bc19247aa504c36879ce75cb7f662b4e8067fbc83e5d24ef0afd9a84f"}}`

const txMoveNeoVerbose = `{"id":5,"jsonrpc":"2.0","result":{"blockhash":"0xea6385e943832b65ee225aaeb31933a97f3362505ab84cfe5dbd91cd1672b9b7","confirmations":6,"blocktime":1614602006001,"vmstate":"HALT","hash":"0x7c10b90077bddfe9095b2db96bb4ac33994ed1ca99c805410f55c771eee0b77b","size":489,"version":0,"nonce":2,"sender":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","sysfee":"11000000","netfee":"4422930","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x95307cb9cc8c4578cef9f6845895eb7aa8be125e","scopes":"CalledByEntry"}],"script":"CwIY3fUFDBSqis+FnU/kArNOZz8hVoIXlqSI6wwUXhK+qHrrlViE9vnOeEWMzLl8MJUUwB8MCHRyYW5zZmVyDBSDqwZ5rVXAUKE61D9ZNupz9ese9kFifVtSOQ==","witnesses":[{"invocation":"DECKEAHrkcuS4I+DGIrhfbS4QHmISn+j63M3Gyhnlps/ijVlCyPpkG3gzxVht5hsD5EgRC1alTK1DaooGS35SYTcDEAbqjpPMa1ZQMeQOVWvRZTIbt4qPsCK7mz6Fja9LJJQSoePB/cN1hz30xQUgFvDPXj6Lv01VzONF/lNO38vrPvDDECJcNQCl/35Na59Rqo2TqjZoVY0D5uk5Owm9X83gWuG2iBMuQ5mmjPGsodLZvDd1XPCTUsJyvdbyFzxvwPUSkyr","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}]}}`

// getResultBlock1 returns data for block number 1 which is used by several tests.
func getResultBlock1() *result.Block {
	binB, err := base64.StdEncoding.DecodeString(base64B1)
	if err != nil {
		panic(err)
	}
	b := block.New(netmode.UnitTestNet, false)
	err = testserdes.DecodeBinary(binB, b)
	if err != nil {
		panic(err)
	}
	b2Hash, err := util.Uint256DecodeStringLE("85ab779bc19247aa504c36879ce75cb7f662b4e8067fbc83e5d24ef0afd9a84f")
	if err != nil {
		panic(err)
	}
	return &result.Block{
		Block: *b,
		BlockMetadata: result.BlockMetadata{
			Size:          1433,
			NextBlockHash: &b2Hash,
			Confirmations: 6,
		},
	}
}

func getTxMoveNeo() *result.TransactionOutputRaw {
	b1 := getResultBlock1()
	txBin, err := base64.StdEncoding.DecodeString(base64TxMoveNeo)
	if err != nil {
		panic(err)
	}
	tx, err := transaction.NewTransactionFromBytes(netmode.UnitTestNet, txBin)
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
// published in official C# JSON-RPC API v2.10.3 reference
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
							VMState:     vm.HaltState,
							GasConsumed: 1,
							Stack:       []stackitem.Item{stackitem.NewBigInteger(big.NewInt(1))},
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
			serverResponse: b1Verbose,
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
			serverResponse: b1Verbose,
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
			serverResponse: header1Verbose,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return &result.Header{
					Hash:          b.Hash(),
					Size:          451,
					Version:       b.Version,
					NextBlockHash: b.NextBlockHash,
					PrevBlockHash: b.PrevHash,
					MerkleRoot:    b.MerkleRoot,
					Timestamp:     b.Timestamp,
					Index:         b.Index,
					NextConsensus: address.Uint160ToString(b.NextConsensus),
					Witnesses:     []transaction.Witness{b.Script},
					Confirmations: 6,
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
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"id":0,"nef":{"magic":860243278,"compiler":"neo-go-3.0","script":"VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==","checksum":2512077441},"manifest":{"name":"Test","abi":{"methods":[],"events":[]},"groups":[],"permissions":[],"trusts":[],"supportedstandards":[],"safemethods":[],"extra":null},"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176"}}`,
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
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"id":0,"nef":{"magic":860243278,"compiler":"neo-go-3.0","script":"VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==","checksum":2512077441},"manifest":{"name":"Test","abi":{"methods":[],"events":[]},"groups":[],"permissions":[],"trusts":[],"supportedstandards":[],"safemethods":[],"extra":null},"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176"}}`,
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
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"id":0,"nef":{"magic":860243278,"compiler":"neo-go-3.0","script":"VgJXHwIMDWNvbnRyYWN0IGNhbGx4eVMTwEEFB5IWIXhKDANQdXSXJyQAAAAQVUGEGNYNIXJqeRDOeRHOU0FSoUH1IUURQCOPAgAASgwLdG90YWxTdXBwbHmXJxEAAABFAkBCDwBAI28CAABKDAhkZWNpbWFsc5cnDQAAAEUSQCNWAgAASgwEbmFtZZcnEgAAAEUMBFJ1YmxAIzwCAABKDAZzeW1ib2yXJxEAAABFDANSVUJAIyECAABKDAliYWxhbmNlT2aXJ2IAAAAQVUGEGNYNIXN5EM50bMoAFLQnIwAAAAwPaW52YWxpZCBhZGRyZXNzEVVBNtNSBiFFENsgQGtsUEEfLnsHIXUMCWJhbGFuY2VPZmxtUxPAQQUHkhYhRW1AI7IBAABKDAh0cmFuc2ZlcpcnKwEAABBVQYQY1g0hdnkQzncHbwfKABS0JyoAAAAMFmludmFsaWQgJ2Zyb20nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRHOdwhvCMoAFLQnKAAAAAwUaW52YWxpZCAndG8nIGFkZHJlc3MRVUE201IGIUUQ2yBAeRLOdwlvCRC1JyIAAAAMDmludmFsaWQgYW1vdW50EVVBNtNSBiFFENsgQG5vB1BBHy57ByF3Cm8Kbwm1JyYAAAAMEmluc3VmZmljaWVudCBmdW5kcxFVQTbTUgYhRRDbIEBvCm8Jn3cKbm8HbwpTQVKhQfUhbm8IUEEfLnsHIXcLbwtvCZ53C25vCG8LU0FSoUH1IQwIdHJhbnNmZXJvB28IbwlUFMBBBQeSFiFFEUAjewAAAEoMBGluaXSXJ1AAAAAQVUGEGNYNIXcMEFVBh8PSZCF3DQJAQg8Adw5vDG8Nbw5TQVKhQfUhDAh0cmFuc2ZlcgwA2zBvDW8OVBTAQQUHkhYhRRFAIyMAAAAMEWludmFsaWQgb3BlcmF0aW9uQTbTUgY6IwUAAABFQA==","checksum":2512077441},"manifest":{"name":"Test","abi":{"methods":[],"events":[]},"groups":[],"permissions":[],"trusts":[],"supportedstandards":[],"safemethods":[],"extra":null},"hash":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176"}}`,
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
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"balance":[{"assethash":"a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8","amount":"50000000000","lastupdatedblock":251604}],"address":"AY6eqWjsUFCzsVELG7yG72XDukKvC34p2w"}}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint160DecodeStringLE("a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8")
				if err != nil {
					panic(err)
				}
				return &result.NEP17Balances{
					Balances: []result.NEP17Balance{{
						Asset:       hash,
						Amount:      "50000000000",
						LastUpdated: 251604,
					}},
					Address: "AY6eqWjsUFCzsVELG7yG72XDukKvC34p2w",
				}
			},
		},
	},
	"getnep17transfers": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP17Transfers("AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF", nil, nil, nil, nil)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"sent":[],"received":[{"timestamp":1555651816,"assethash":"600c4f5200db36177e3e8a09e9f18e2fc7d12a0f","transferaddress":"AYwgBNMepiv5ocGcyNT4mA8zPLTQ8pDBis","amount":"1000000","blockindex":436036,"transfernotifyindex":0,"txhash":"df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58"}],"address":"AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF"}}`,
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
					Address: "AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF",
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
				hash, err := util.Uint256DecodeStringLE("ca23bd5df3249836849309ca2afe972bfd288b0a7ae61302c8fd545daa8bffd6")
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
				hash, err := util.Uint256DecodeStringLE("7c10b90077bddfe9095b2db96bb4ac33994ed1ca99c805410f55c771eee0b77b")
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
			serverResponse: txMoveNeoVerbose,
			result: func(c *Client) interface{} {
				return getTxMoveNeo()
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
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"magic":42,"tcpport":20332,"wsport":20342,"nonce":2153672787,"useragent":"/NEO-GO:0.73.1-pre-273-ge381358/"}}`,
			result: func(c *Client) interface{} {
				return &result.Version{
					Magic:     netmode.UnitTestNet,
					TCPPort:   uint16(20332),
					WSPort:    uint16(20342),
					Nonce:     2153672787,
					UserAgent: "/NEO-GO:0.73.1-pre-273-ge381358/",
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
				return c.SendRawTransaction(transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0))
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
				return c.SendRawTransaction(transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0))
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
			name: "getnep17balances_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP17Balances(util.Uint160{})
			},
		},
		{
			name: "getnep17transfers_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP17Transfers("", nil, nil, nil, nil)
			},
		},
		{
			name: "getnep17transfers_invalid_params_error 2",
			invoke: func(c *Client) (interface{}, error) {
				var stop uint32
				return c.GetNEP17Transfers("NTh9TnZTstvAePEYWDGLLxidBikJE24uTo", nil, &stop, nil, nil)
			},
		},
		{
			name: "getnep17transfers_invalid_params_error 3",
			invoke: func(c *Client) (interface{}, error) {
				var start uint32
				var limit int
				return c.GetNEP17Transfers("NTh9TnZTstvAePEYWDGLLxidBikJE24uTo", &start, nil, &limit, nil)
			},
		},
		{
			name: "getnep17transfers_invalid_params_error 4",
			invoke: func(c *Client) (interface{}, error) {
				var start, stop uint32
				var page int
				return c.GetNEP17Transfers("NTh9TnZTstvAePEYWDGLLxidBikJE24uTo", &start, &stop, nil, &page)
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
			name: "getnep17balances_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP17Balances(util.Uint160{})
			},
		},
		{
			name: "getnep17transfers_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP17Transfers("", nil, nil, nil, nil)
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
				return c.SendRawTransaction(transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0))
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
			require.NoError(t, c.Init())
			return c, nil
		})
	})
	t.Run("WSClient", func(t *testing.T) {
		testRPCClient(t, func(ctx context.Context, endpoint string, opts Options) (*Client, error) {
			wsc, err := NewWS(ctx, httpURLtoWS(endpoint), opts)
			require.NoError(t, err)
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
		c, err := newClient(context.TODO(), endpoint, opts)
		if err != nil {
			t.Fatal(err)
		}

		for _, testCase := range testBatch {
			t.Run(testCase.name, func(t *testing.T) {
				_, err := testCase.invoke(c)
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
				ws.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, p, err := ws.ReadMessage()
				if err != nil {
					break
				}
				r := request.NewIn()
				err = json.Unmarshal(p, r)
				if err != nil {
					t.Fatalf("Cannot decode request body: %s", req.Body)
				}
				response := wrapInitResponse(r, resp)
				ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
				err = ws.WriteMessage(1, []byte(response))
				if err != nil {
					break
				}
			}
			ws.Close()
			return
		}
		r := request.NewRequest()
		err := r.DecodeData(req.Body)
		if err != nil {
			t.Fatalf("Cannot decode request body: %s", req.Body)
		}
		requestHandler(t, r.In, w, resp)
	}))

	t.Cleanup(srv.Close)

	return srv
}

func requestHandler(t *testing.T, r *request.In, w http.ResponseWriter, resp string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	response := wrapInitResponse(r, resp)
	_, err := w.Write([]byte(response))
	if err != nil {
		t.Fatalf("Error writing response: %s", err.Error())
	}
}

func wrapInitResponse(r *request.In, resp string) string {
	var response string
	switch r.Method {
	case "getversion":
		response = `{"id":1,"jsonrpc":"2.0","result":{"magic":42,"tcpport":20332,"wsport":20342,"nonce":2153672787,"useragent":"/NEO-GO:0.73.1-pre-273-ge381358/"}}`
	case "getcontractstate":
		p, _ := r.Params()
		name, _ := p.ValueWithType(0, request.StringT).GetString()
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
		r := request.NewRequest()
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
		r := request.NewRequest()
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
		// network was not initialised
		require.Equal(t, netmode.Magic(0), c.GetNetwork())
		require.Equal(t, false, c.initDone)
	})

	t.Run("good", func(t *testing.T) {
		c, err := New(context.TODO(), endpoint, opts)
		if err != nil {
			t.Fatal(err)
		}
		require.NoError(t, c.Init())
		require.Equal(t, netmode.UnitTestNet, c.GetNetwork())
	})
}

func TestUninitedClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r := request.NewRequest()
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
