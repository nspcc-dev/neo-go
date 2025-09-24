package rpcsrv

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

const testOverflow = false

func wsReader(t *testing.T, ws *websocket.Conn, msgCh chan<- []byte, readerStopCh chan struct{}, readerToExitCh chan struct{}) {
readLoop:
	for {
		select {
		case <-readerStopCh:
			break readLoop
		default:
			err := ws.SetReadDeadline(time.Now().Add(5 * time.Second))
			select {
			case <-readerStopCh:
				break readLoop
			default:
				require.NoError(t, err)
			}

			_, body, err := ws.ReadMessage()
			select {
			case <-readerStopCh:
				break readLoop
			default:
				require.NoError(t, err)
			}

			select {
			case msgCh <- body:
			case <-time.After(10 * time.Second):
				t.Log("exiting wsReader loop: unable to send response to receiver")
				break readLoop
			}
		}
	}
	close(readerToExitCh)
}

func callWSGetRaw(t *testing.T, ws *websocket.Conn, msg string, respCh <-chan []byte) *neorpc.Response {
	var resp = new(neorpc.Response)

	require.NoError(t, ws.SetWriteDeadline(time.Now().Add(5*time.Second)))
	require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte(msg)))

	body := <-respCh
	require.NoError(t, json.Unmarshal(body, resp))
	return resp
}

func getNotification(t *testing.T, respCh <-chan []byte) *neorpc.Notification {
	var resp = new(neorpc.Notification)
	body := <-respCh
	require.NoError(t, json.Unmarshal(body, resp))
	return resp
}

func initCleanServerAndWSClient(t *testing.T, startNetworkServer ...bool) (*core.Blockchain, *Server, *websocket.Conn, chan []byte) {
	chain, rpcSrv, httpSrv := initClearServerWithInMemoryChain(t)
	ws, respMsgs := initWSClient(t, httpSrv, rpcSrv, startNetworkServer...)
	return chain, rpcSrv, ws, respMsgs
}

func initWSClient(t *testing.T, httpSrv *httptest.Server, rpcSrv *Server, startNetworkServer ...bool) (*websocket.Conn, chan []byte) {
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	url := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"
	ws, r, err := dialer.Dial(url, nil)
	require.NoError(t, err)
	defer r.Body.Close()

	// Use buffered channel to read server's messages and then read expected
	// responses from it.
	respMsgs := make(chan []byte, 16)
	readerStopCh := make(chan struct{})
	readerToExitCh := make(chan struct{})
	go wsReader(t, ws, respMsgs, readerStopCh, readerToExitCh)
	if len(startNetworkServer) != 0 && startNetworkServer[0] {
		rpcSrv.coreServer.Start()
	}
	t.Cleanup(func() {
	drainLoop:
		for {
			select {
			case <-respMsgs:
			default:
				break drainLoop
			}
		}
		close(readerStopCh)
		ws.Close()
		<-readerToExitCh
		if len(startNetworkServer) != 0 && startNetworkServer[0] {
			rpcSrv.coreServer.Shutdown()
		}
	})
	return ws, respMsgs
}

func callSubscribe(t *testing.T, ws *websocket.Conn, msgs <-chan []byte, params string) string {
	var s string
	resp := callWSGetRaw(t, ws, fmt.Sprintf(`{"jsonrpc": "2.0","method": "subscribe","params": %s,"id": 1}`, params), msgs)
	require.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)
	require.NoError(t, json.Unmarshal(resp.Result, &s))
	return s
}

func callUnsubscribe(t *testing.T, ws *websocket.Conn, msgs <-chan []byte, id string) {
	var b bool
	resp := callWSGetRaw(t, ws, fmt.Sprintf(`{"jsonrpc": "2.0","method": "unsubscribe","params": ["%s"],"id": 1}`, id), msgs)
	require.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)
	require.NoError(t, json.Unmarshal(resp.Result, &b))
	require.Equal(t, true, b)
}

func TestSubscriptions(t *testing.T) {
	var subIDs = make([]string, 0)
	var subFeeds = []string{"block_added", "transaction_added", "notification_from_execution", "transaction_executed", "notary_request_event", "header_of_added_block", "mempool_event"}

	chain, rpcSrv, c, respMsgs := initCleanServerAndWSClient(t, true)

	for _, feed := range subFeeds {
		s := callSubscribe(t, c, respMsgs, fmt.Sprintf(`["%s"]`, feed))
		subIDs = append(subIDs, s)
	}

	for _, b := range getTestBlocks(t) {
		require.NoError(t, chain.AddBlock(b))
		resp := getNotification(t, respMsgs)
		require.Equal(t, neorpc.ExecutionEventID, resp.Event)
		for {
			resp = getNotification(t, respMsgs)
			if resp.Event != neorpc.NotificationEventID {
				break
			}
		}
		for i := range b.Transactions {
			if i > 0 {
				resp = getNotification(t, respMsgs)
			}
			require.Equal(t, neorpc.ExecutionEventID, resp.Event)
			for {
				resp := getNotification(t, respMsgs)
				if resp.Event == neorpc.NotificationEventID {
					continue
				}
				require.Equal(t, neorpc.TransactionEventID, resp.Event)
				break
			}
		}
		resp = getNotification(t, respMsgs)
		require.Equal(t, neorpc.ExecutionEventID, resp.Event)
		for {
			resp = getNotification(t, respMsgs)
			if resp.Event != neorpc.NotificationEventID {
				break
			}
		}
		require.Equal(t, neorpc.HeaderOfAddedBlockEventID, resp.Event)
		resp = getNotification(t, respMsgs)
		require.Equal(t, neorpc.BlockEventID, resp.Event)
	}

	// We should manually add NotaryRequest to test notification.
	sender := testchain.PrivateKeyByID(0)
	err := rpcSrv.coreServer.RelayP2PNotaryRequest(createValidNotaryRequest(chain, sender, 1, 2_0000_0000, nil))
	require.NoError(t, err)
	for {
		resp := getNotification(t, respMsgs)
		if resp.Event == neorpc.NotaryRequestEventID {
			break
		}
	}
	// Test that subscribing to the mempool delivers both “added” and “removed” events.
	// We add a transaction with a specific signer and then remove it, expecting
	// two notifications: one when it’s added to the pool, and one when it’s removed.
	signer := testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()
	tx := &transaction.Transaction{
		Signers: []transaction.Signer{{Account: signer}},
	}

	require.NoError(t, chain.GetMemPool().Add(tx, &FeerStub{}))
	e := getNotification(t, respMsgs)
	require.Equal(t, neorpc.MempoolEventID, e.Event)

	chain.GetMemPool().Remove(tx.Hash())
	e2 := getNotification(t, respMsgs)
	require.Equal(t, neorpc.MempoolEventID, e2.Event)

	for _, id := range subIDs {
		callUnsubscribe(t, c, respMsgs, id)
	}
}

func TestFilteredSubscriptions(t *testing.T) {
	priv0 := testchain.PrivateKeyByID(0)
	var goodSender = priv0.GetScriptHash()

	var cases = map[string]struct {
		params      string
		check       func(*testing.T, *neorpc.Notification)
		shouldCheck bool
	}{
		"tx matching sender": {
			params:      `["transaction_added", {"sender":"` + goodSender.StringLE() + `"}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.TransactionEventID, resp.Event)
				sender := rmap["sender"].(string)
				require.Equal(t, address.Uint160ToString(goodSender), sender)
			},
		},
		"tx matching signer": {
			params:      `["transaction_added", {"signer":"` + goodSender.StringLE() + `"}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.TransactionEventID, resp.Event)
				signers := rmap["signers"].([]any)
				signer0 := signers[0].(map[string]any)
				signer0acc := signer0["account"].(string)
				require.Equal(t, "0x"+goodSender.StringLE(), signer0acc)
			},
		},
		"tx matching sender and signer": {
			params:      `["transaction_added", {"sender":"` + goodSender.StringLE() + `", "signer":"` + goodSender.StringLE() + `"}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.TransactionEventID, resp.Event)
				sender := rmap["sender"].(string)
				require.Equal(t, address.Uint160ToString(goodSender), sender)
				signers := rmap["signers"].([]any)
				signer0 := signers[0].(map[string]any)
				signer0acc := signer0["account"].(string)
				require.Equal(t, "0x"+goodSender.StringLE(), signer0acc)
			},
		},
		"notification matching contract hash": {
			params:      `["notification_from_execution", {"contract":"` + testContractHashLE + `"}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.NotificationEventID, resp.Event)
				c := rmap["contract"].(string)
				require.Equal(t, "0x"+testContractHashLE, c)
			},
		},
		"notification matching name": {
			params:      `["notification_from_execution", {"name":"Transfer"}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.NotificationEventID, resp.Event)
				n := rmap["eventname"].(string)
				require.Equal(t, "Transfer", n)
			},
		},
		"notification matching contract hash and name": {
			params:      `["notification_from_execution", {"contract":"` + testContractHashLE + `", "name":"Transfer"}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.NotificationEventID, resp.Event)
				c := rmap["contract"].(string)
				require.Equal(t, "0x"+testContractHashLE, c)
				n := rmap["eventname"].(string)
				require.Equal(t, "Transfer", n)
			},
		},
		"notification matching contract hash and parameter": {
			params:      `["notification_from_execution", {"contract":"` + testContractHashLE + `", "parameters":[{"type":"Any","value":null},{"type":"Hash160","value":"` + testContractHashLE + `"}]}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.NotificationEventID, resp.Event)
				c := rmap["contract"].(string)
				require.Equal(t, "0x"+testContractHashLE, c)
				// It should be exact unique "Init" call sending all the tokens to the contract itself.
				parameters := rmap["state"].(map[string]any)["value"].([]any)
				require.Len(t, parameters, 3)
				// Sender.
				toType := parameters[1].(map[string]any)["type"].(string)
				require.Equal(t, smartcontract.Hash160Type.ConvertToStackitemType().String(), toType)
				to := parameters[1].(map[string]any)["value"].(string)
				require.Equal(t, base64.StdEncoding.EncodeToString(testContractHash.BytesBE()), to)
				// This amount happens only for initial token distribution.
				amountType := parameters[2].(map[string]any)["type"].(string)
				require.Equal(t, smartcontract.IntegerType.ConvertToStackitemType().String(), amountType)
				amount := parameters[2].(map[string]any)["value"].(string)
				require.Equal(t, "1000000", amount)
			},
		},
		"notification matching contract hash but unknown parameter": {
			params:      `["notification_from_execution", {"contract":"` + testContractHashLE + `", "parameters":[{"type":"Any","value":null},{"type":"Hash160","value":"ffffffffffffffffffffffffffffffffffffffff"}]}]`,
			shouldCheck: false,
			check: func(t *testing.T, resp *neorpc.Notification) {
				t.Fatal("this filter should not return any notification from test contract")
			},
		},
		"execution matching state": {
			params:      `["transaction_executed", {"state":"HALT"}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.ExecutionEventID, resp.Event)
				st := rmap["vmstate"].(string)
				require.Equal(t, "HALT", st)
			},
		},
		"execution matching container": {
			params:      `["transaction_executed", {"container":"` + deploymentTxHash + `"}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.ExecutionEventID, resp.Event)
				tx := rmap["container"].(string)
				require.Equal(t, "0x"+deploymentTxHash, tx)
			},
		},
		"execution matching state and container": {
			params:      `["transaction_executed", {"state":"HALT", "container":"` + deploymentTxHash + `"}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.ExecutionEventID, resp.Event)
				tx := rmap["container"].(string)
				require.Equal(t, "0x"+deploymentTxHash, tx)
				st := rmap["vmstate"].(string)
				require.Equal(t, "HALT", st)
			},
		},
		"tx non-matching": {
			params:      `["transaction_added", {"sender":"00112233445566778899aabbccddeeff00112233"}]`,
			shouldCheck: false,
			check: func(t *testing.T, _ *neorpc.Notification) {
				t.Fatal("unexpected match for EnrollmentTransaction")
			},
		},
		"notification non-matching": {
			params:      `["notification_from_execution", {"contract":"00112233445566778899aabbccddeeff00112233"}]`,
			shouldCheck: false,
			check: func(t *testing.T, _ *neorpc.Notification) {
				t.Fatal("unexpected match for contract 00112233445566778899aabbccddeeff00112233")
			},
		},
		"execution non-matching": {
			// We have single FAULTed transaction in chain, this, use the wrong hash for this test instead of FAULT state.
			params:      `["transaction_executed", {"container":"0x` + util.Uint256{}.StringLE() + `"}]`,
			shouldCheck: false,
			check: func(t *testing.T, n *neorpc.Notification) {
				t.Fatal("unexpected match for faulted execution")
			},
		},
		"header of added block": {
			params:      `["header_of_added_block", {"primary": 0, "since": 5}]`,
			shouldCheck: true,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.HeaderOfAddedBlockEventID, resp.Event)
				primary := rmap["primary"].(float64)
				require.Equal(t, 0, int(primary))
				index := rmap["index"].(float64)
				require.Less(t, 4, int(index))
			},
		},
	}

	for name, this := range cases {
		t.Run(name, func(t *testing.T) {
			chain, _, c, respMsgs := initCleanServerAndWSClient(t)

			// It's used as an end-of-event-stream, so it's always present.
			blockSubID := callSubscribe(t, c, respMsgs, `["block_added"]`)
			subID := callSubscribe(t, c, respMsgs, this.params)

			var (
				lastBlock uint32
				checked   = false
			)
			for _, b := range getTestBlocks(t) {
				require.NoError(t, chain.AddBlock(b))
				lastBlock = b.Index
			}

			for {
				resp := getNotification(t, respMsgs)
				rmap := resp.Payload[0].(map[string]any)
				if resp.Event == neorpc.BlockEventID {
					index := rmap["index"].(float64)
					if uint32(index) == lastBlock {
						break
					}
					continue
				}
				if this.shouldCheck {
					checked = true
					this.check(t, resp)
				} else {
					t.Fatalf("unexpected notification: %s", resp.EventID())
				}
			}

			if this.shouldCheck && !checked {
				t.Fatal("expected check is not performed")
			}

			callUnsubscribe(t, c, respMsgs, subID)
			callUnsubscribe(t, c, respMsgs, blockSubID)
		})
	}
}

func TestFilteredMempoolSubscriptions(t *testing.T) {
	// We can’t fit this into TestFilteredSubscriptions because mempool events
	// doesn't depend on blocks events.
	priv0 := testchain.PrivateKeyByID(0)
	goodSender := priv0.GetScriptHash()

	var cases = map[string]struct {
		params string
		check  func(*testing.T, *neorpc.Notification)
	}{
		"mempool_event matching type": {
			params: `["mempool_event", {"type":"added"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				require.Equal(t, neorpc.MempoolEventID, resp.Event)
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, "added", rmap["type"].(string))
			},
		},
		"mempool_event matching sender": {
			params: `["mempool_event", {"sender":"` + goodSender.StringLE() + `"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				require.Equal(t, neorpc.MempoolEventID, resp.Event)
				txMap := resp.Payload[0].(map[string]any)["transaction"].(map[string]any)
				require.Equal(t, address.Uint160ToString(goodSender), txMap["sender"].(string))
			},
		},
		"mempool_event matching signer": {
			params: `["mempool_event", {"signer":"` + goodSender.StringLE() + `"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				require.Equal(t, neorpc.MempoolEventID, resp.Event)
				txMap := resp.Payload[0].(map[string]any)["transaction"].(map[string]any)
				require.Equal(t, "0x"+goodSender.StringLE(), txMap["signers"].([]any)[0].(map[string]any)["account"].(string))
			},
		},
		"mempool_event matching sender, signer and type": {
			params: `["mempool_event", {"sender":"` + goodSender.StringLE() + `", "signer":"` + goodSender.StringLE() + `", "type":"added"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				require.Equal(t, neorpc.MempoolEventID, resp.Event)
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, "added", rmap["type"].(string))
				txMap := rmap["transaction"].(map[string]any)
				require.Equal(t, address.Uint160ToString(goodSender), txMap["sender"].(string))
				require.Equal(t, "0x"+goodSender.StringLE(), txMap["signers"].([]any)[0].(map[string]any)["account"].(string))
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			chain, _, c, respMsgs := initCleanServerAndWSClient(t)

			subID := callSubscribe(t, c, respMsgs, tc.params)
			defer callUnsubscribe(t, c, respMsgs, subID)

			tx := &transaction.Transaction{
				Signers: []transaction.Signer{
					{Account: goodSender},
				},
			}
			require.NoError(t, chain.GetMemPool().Add(tx, &FeerStub{}))

			resp := getNotification(t, respMsgs)
			tc.check(t, resp)
		})
	}
}

func TestFilteredNotaryRequestSubscriptions(t *testing.T) {
	// We can't fit this into TestFilteredSubscriptions, because notary requests
	// event doesn't depend on blocks events.
	priv0 := testchain.PrivateKeyByID(0)
	var goodSender = priv0.GetScriptHash()

	var cases = map[string]struct {
		params string
		check  func(*testing.T, *neorpc.Notification)
	}{
		"matching sender": {
			params: `["notary_request_event", {"sender":"` + goodSender.StringLE() + `"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.NotaryRequestEventID, resp.Event)
				require.Equal(t, "added", rmap["type"].(string))
				req := rmap["notaryrequest"].(map[string]any)
				fbTx := req["fallbacktx"].(map[string]any)
				sender := fbTx["signers"].([]any)[1].(map[string]any)["account"].(string)
				require.Equal(t, "0x"+goodSender.StringLE(), sender)
			},
		},
		"matching signer": {
			params: `["notary_request_event", {"signer":"` + goodSender.StringLE() + `"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.NotaryRequestEventID, resp.Event)
				require.Equal(t, "added", rmap["type"].(string))
				req := rmap["notaryrequest"].(map[string]any)
				mainTx := req["maintx"].(map[string]any)
				signers := mainTx["signers"].([]any)
				signer0 := signers[0].(map[string]any)
				signer0acc := signer0["account"].(string)
				require.Equal(t, "0x"+goodSender.StringLE(), signer0acc)
			},
		},
		"matching type": {
			params: `["notary_request_event", {"type":"added"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				require.Equal(t, neorpc.NotaryRequestEventID, resp.Event)
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, "added", rmap["type"].(string))
			},
		},
		"matching sender, signer and type": {
			params: `["notary_request_event", {"sender":"` + goodSender.StringLE() + `", "signer":"` + goodSender.StringLE() + `","type":"added"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.NotaryRequestEventID, resp.Event)
				require.Equal(t, "added", rmap["type"].(string))
				req := rmap["notaryrequest"].(map[string]any)
				mainTx := req["maintx"].(map[string]any)
				fbTx := req["fallbacktx"].(map[string]any)
				sender := fbTx["signers"].([]any)[1].(map[string]any)["account"].(string)
				require.Equal(t, "0x"+goodSender.StringLE(), sender)
				signers := mainTx["signers"].([]any)
				signer0 := signers[0].(map[string]any)
				signer0acc := signer0["account"].(string)
				require.Equal(t, "0x"+goodSender.StringLE(), signer0acc)
			},
		},
	}

	chain, rpcSrv, c, respMsgs := initCleanServerAndWSClient(t, true)

	// blocks are needed to make GAS deposit for priv0
	blocks := getTestBlocks(t)
	for _, b := range blocks {
		require.NoError(t, chain.AddBlock(b))
	}

	var nonce uint32 = 100
	for name, this := range cases {
		t.Run(name, func(t *testing.T) {
			subID := callSubscribe(t, c, respMsgs, this.params)

			err := rpcSrv.coreServer.RelayP2PNotaryRequest(createValidNotaryRequest(chain, priv0, nonce, 2_0000_0000, nil))
			require.NoError(t, err)
			nonce++

			var resp = new(neorpc.Notification)
			select {
			case body := <-respMsgs:
				require.NoError(t, json.Unmarshal(body, resp))
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for event")
			}

			require.Equal(t, neorpc.NotaryRequestEventID, resp.Event)
			this.check(t, resp)

			callUnsubscribe(t, c, respMsgs, subID)
		})
	}
}

func TestFilteredBlockSubscriptions(t *testing.T) {
	// We can't fit this into TestFilteredSubscriptions, because it uses
	// blocks as EOF events to wait for.
	const numBlocks = 10
	chain, _, c, respMsgs := initCleanServerAndWSClient(t)

	blockSubID := callSubscribe(t, c, respMsgs, `["block_added", {"primary":3}]`)

	var expectedCnt int
	for i := range numBlocks {
		primary := uint32(i % 4)
		if primary == 3 {
			expectedCnt++
		}
		b := testchain.NewBlock(t, chain, 1, primary)
		require.NoError(t, chain.AddBlock(b))
	}

	for range expectedCnt {
		var resp = new(neorpc.Notification)
		select {
		case body := <-respMsgs:
			require.NoError(t, json.Unmarshal(body, resp))
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}

		require.Equal(t, neorpc.BlockEventID, resp.Event)
		rmap := resp.Payload[0].(map[string]any)
		primary := rmap["primary"].(float64)
		require.Equal(t, 3, int(primary))
	}
	callUnsubscribe(t, c, respMsgs, blockSubID)
}

func TestHeaderOfAddedBlockSubscriptions(t *testing.T) {
	const numBlocks = 10
	chain, _, c, respMsgs := initCleanServerAndWSClient(t)

	headerSubID := callSubscribe(t, c, respMsgs, `["header_of_added_block", {"primary":3}]`)

	var expectedCnt int
	for i := range numBlocks {
		primary := uint32(i % 4)
		if primary == 3 {
			expectedCnt++
		}
		b := testchain.NewBlock(t, chain, 1, primary)
		require.NoError(t, chain.AddBlock(b))
	}

	for range expectedCnt {
		var resp = new(neorpc.Notification)
		select {
		case body := <-respMsgs:
			require.NoError(t, json.Unmarshal(body, resp))
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}

		require.Equal(t, neorpc.HeaderOfAddedBlockEventID, resp.Event)
		rmap := resp.Payload[0].(map[string]any)
		primary := rmap["primary"].(float64)
		require.Equal(t, 3, int(primary))
	}
	callUnsubscribe(t, c, respMsgs, headerSubID)
}

func testMaxSubscriptions(t *testing.T, f func(*config.Config), maxFeeds int) {
	var subIDs = make([]string, 0)
	_, rpcSrv, httpSrv := initClearServerWithCustomConfig(t, f)
	c, respMsgs := initWSClient(t, httpSrv, rpcSrv)

	for i := range maxFeeds + 1 {
		var s string
		resp := callWSGetRaw(t, c, `{"jsonrpc": "2.0", "method": "subscribe", "params": ["block_added"], "id": 1}`, respMsgs)
		if i < maxFeeds {
			require.Nil(t, resp.Error)
			require.NotNil(t, resp.Result)
			require.NoError(t, json.Unmarshal(resp.Result, &s))
			// Each ID must be unique.
			for _, id := range subIDs {
				require.NotEqual(t, id, s)
			}
			subIDs = append(subIDs, s)
		} else {
			require.NotNil(t, resp.Error)
			require.Nil(t, resp.Result)
		}
	}
}

func TestMaxSubscriptions(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		testMaxSubscriptions(t, nil, defaultMaxFeeds)
	})
	t.Run("maxfeeds=x2", func(t *testing.T) {
		testMaxSubscriptions(t, func(c *config.Config) {
			c.ApplicationConfiguration.RPC.MaxWebSocketFeeds = defaultMaxFeeds * 2
		}, defaultMaxFeeds*2)
	})
}

func TestBadSubUnsub(t *testing.T) {
	var subCases = map[string]string{
		"no params":              `{"jsonrpc": "2.0", "method": "subscribe", "params": [], "id": 1}`,
		"bad (non-string) event": `{"jsonrpc": "2.0", "method": "subscribe", "params": [1], "id": 1}`,
		"bad (wrong) event":      `{"jsonrpc": "2.0", "method": "subscribe", "params": ["block_removed"], "id": 1}`,
		"missed event":           `{"jsonrpc": "2.0", "method": "subscribe", "params": ["event_missed"], "id": 1}`,
		"block invalid filter":   `{"jsonrpc": "2.0", "method": "subscribe", "params": ["block_added", 1], "id": 1}`,
		"tx filter 1":            `{"jsonrpc": "2.0", "method": "subscribe", "params": ["transaction_added", 1], "id": 1}`,
		"tx filter 2":            `{"jsonrpc": "2.0", "method": "subscribe", "params": ["transaction_added", {"state": "HALT"}], "id": 1}`,
		"notification filter 1":  `{"jsonrpc": "2.0", "method": "subscribe", "params": ["notification_from_execution", "contract"], "id": 1}`,
		"notification filter 2":  `{"jsonrpc": "2.0", "method": "subscribe", "params": ["notification_from_execution", "name"], "id": 1}`,
		"execution filter 1":     `{"jsonrpc": "2.0", "method": "subscribe", "params": ["transaction_executed", "FAULT"], "id": 1}`,
		"execution filter 2":     `{"jsonrpc": "2.0", "method": "subscribe", "params": ["transaction_executed", {"state": "STOP"}], "id": 1}`,
	}
	var unsubCases = map[string]string{
		"no params":         `{"jsonrpc": "2.0", "method": "unsubscribe", "params": [], "id": 1}`,
		"bad id":            `{"jsonrpc": "2.0", "method": "unsubscribe", "params": ["vasiliy"], "id": 1}`,
		"not subscribed id": `{"jsonrpc": "2.0", "method": "unsubscribe", "params": ["7"], "id": 1}`,
	}
	_, _, c, respMsgs := initCleanServerAndWSClient(t)

	testF := func(t *testing.T, cases map[string]string) func(t *testing.T) {
		return func(t *testing.T) {
			for n, s := range cases {
				t.Run(n, func(t *testing.T) {
					resp := callWSGetRaw(t, c, s, respMsgs)
					require.NotNil(t, resp.Error)
					require.Nil(t, resp.Result)
				})
			}
		}
	}
	t.Run("subscribe", testF(t, subCases))
	t.Run("unsubscribe", testF(t, unsubCases))
}

func doSomeWSRequest(t *testing.T, ws *websocket.Conn) {
	require.NoError(t, ws.SetWriteDeadline(time.Now().Add(5*time.Second)))
	// It could be just about anything including invalid request,
	// we only care about server handling being active.
	require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc": "2.0", "method": "getversion", "params": [], "id": 1}`)))
	err := ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	require.NoError(t, err)
	_, _, err = ws.ReadMessage()
	require.NoError(t, err)
}

func TestWSClientsLimit(t *testing.T) {
	for tname, limit := range map[string]int{"8": 8, "disabled": -1} {
		effectiveClients := max(limit, 0)
		t.Run(tname, func(t *testing.T) {
			_, _, httpSrv := initClearServerWithCustomConfig(t, func(cfg *config.Config) {
				cfg.ApplicationConfiguration.RPC.MaxWebSocketClients = limit
			})

			dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
			url := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"
			wss := make([]*websocket.Conn, effectiveClients)
			var wg sync.WaitGroup

			// Dial effectiveClients connections in parallel
			for i := range effectiveClients {
				wg.Add(1)
				j := i
				go func() {
					defer wg.Done()
					ws, r, err := dialer.Dial(url, nil)
					if r != nil {
						defer r.Body.Close()
					}
					require.NoError(t, err)
					wss[j] = ws
					doSomeWSRequest(t, ws)
				}()
			}

			wg.Wait()

			// Attempt one more connection, which should fail
			_, r, err := dialer.Dial(url, nil)
			require.Error(t, err, "The connection beyond the limit should fail")
			if r != nil {
				r.Body.Close()
			}
			// Check connections are still alive (it actually is necessary to add
			// some use of wss to keep connections alive).
			for _, ws := range wss {
				doSomeWSRequest(t, ws)
				ws.Close()
			}
		})
	}
}

// The purpose of this test is to overflow buffers on server side to
// receive a 'missed' event. But it's actually hard to tell when exactly
// that's going to happen because of network-level buffering, typical
// number seen in tests is around ~3500 events, but it's not reliable enough,
// thus this test is disabled.
func TestSubscriptionOverflow(t *testing.T) {
	if !testOverflow {
		return
	}
	const blockCnt = notificationBufSize * 5
	var receivedMiss bool

	chain, _, c, respMsgs := initCleanServerAndWSClient(t)

	resp := callWSGetRaw(t, c, `{"jsonrpc": "2.0","method": "subscribe","params": ["block_added"],"id": 1}`, respMsgs)
	require.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)

	// Push a lot of new blocks, but don't read events for them.
	for range blockCnt {
		b := testchain.NewBlock(t, chain, 1, 0)
		require.NoError(t, chain.AddBlock(b))
	}
	for range blockCnt {
		resp := getNotification(t, respMsgs)
		if resp.Event != neorpc.BlockEventID {
			require.Equal(t, neorpc.MissedEventID, resp.Event)
			receivedMiss = true
			break
		}
	}
	require.Equal(t, true, receivedMiss)
	// `Missed` is the last event and there is nothing afterwards.
	require.Equal(t, 0, len(respMsgs))
}

func TestFilteredSubscriptions_InvalidFilter(t *testing.T) {
	var cases = map[string]struct {
		params string
	}{
		"notification with long name": {
			params: `["notification_from_execution", {"name":"notification_from_execution_with_long_name"}]`,
		},
		"execution with invalid vm state": {
			params: `["transaction_executed", {"state":"NOTHALT"}]`,
		},
	}
	_, _, c, respMsgs := initCleanServerAndWSClient(t)

	for name, this := range cases {
		t.Run(name, func(t *testing.T) {
			resp := callWSGetRaw(t, c, fmt.Sprintf(`{"jsonrpc": "2.0","method": "subscribe","params": %s,"id": 1}`, this.params), respMsgs)
			require.NotNil(t, resp.Error)
			require.Nil(t, resp.Result)
			require.Contains(t, resp.Error.Error(), neorpc.ErrInvalidSubscriptionFilter.Error())
		})
	}
}
