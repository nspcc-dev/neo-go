package rpcsrv

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

const testOverflow = false

func wsReader(t *testing.T, ws *websocket.Conn, msgCh chan<- []byte, isFinished *atomic.Bool) {
	for {
		err := ws.SetReadDeadline(time.Now().Add(time.Second))
		if isFinished.Load() {
			require.Error(t, err)
			break
		}
		require.NoError(t, err)
		_, body, err := ws.ReadMessage()
		if isFinished.Load() {
			require.Error(t, err)
			break
		}
		require.NoError(t, err)
		msgCh <- body
	}
}

func callWSGetRaw(t *testing.T, ws *websocket.Conn, msg string, respCh <-chan []byte) *neorpc.Response {
	var resp = new(neorpc.Response)

	require.NoError(t, ws.SetWriteDeadline(time.Now().Add(time.Second)))
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

func initCleanServerAndWSClient(t *testing.T) (*core.Blockchain, *Server, *websocket.Conn, chan []byte, *atomic.Bool) {
	chain, rpcSrv, httpSrv := initClearServerWithInMemoryChain(t)

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	url := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"
	ws, r, err := dialer.Dial(url, nil)
	require.NoError(t, err)
	defer r.Body.Close()

	// Use buffered channel to read server's messages and then read expected
	// responses from it.
	respMsgs := make(chan []byte, 16)
	finishedFlag := &atomic.Bool{}
	go wsReader(t, ws, respMsgs, finishedFlag)
	return chain, rpcSrv, ws, respMsgs, finishedFlag
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
	var subFeeds = []string{"block_added", "transaction_added", "notification_from_execution", "transaction_executed", "notary_request_event", "header_of_added_block"}

	chain, rpcSrv, c, respMsgs, finishedFlag := initCleanServerAndWSClient(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	go rpcSrv.coreServer.Start()
	defer rpcSrv.coreServer.Shutdown()

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
		for i := 0; i < len(b.Transactions); i++ {
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

	for _, id := range subIDs {
		callUnsubscribe(t, c, respMsgs, id)
	}
	finishedFlag.CompareAndSwap(false, true)
	c.Close()
}

func TestFilteredSubscriptions(t *testing.T) {
	priv0 := testchain.PrivateKeyByID(0)
	var goodSender = priv0.GetScriptHash()

	var cases = map[string]struct {
		params string
		check  func(*testing.T, *neorpc.Notification)
	}{
		"tx matching sender": {
			params: `["transaction_added", {"sender":"` + goodSender.StringLE() + `"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.TransactionEventID, resp.Event)
				sender := rmap["sender"].(string)
				require.Equal(t, address.Uint160ToString(goodSender), sender)
			},
		},
		"tx matching signer": {
			params: `["transaction_added", {"signer":"` + goodSender.StringLE() + `"}]`,
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
			params: `["transaction_added", {"sender":"` + goodSender.StringLE() + `", "signer":"` + goodSender.StringLE() + `"}]`,
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
			params: `["notification_from_execution", {"contract":"` + testContractHash + `"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.NotificationEventID, resp.Event)
				c := rmap["contract"].(string)
				require.Equal(t, "0x"+testContractHash, c)
			},
		},
		"notification matching name": {
			params: `["notification_from_execution", {"name":"my_pretty_notification"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.NotificationEventID, resp.Event)
				n := rmap["name"].(string)
				require.Equal(t, "my_pretty_notification", n)
			},
		},
		"notification matching contract hash and name": {
			params: `["notification_from_execution", {"contract":"` + testContractHash + `", "name":"my_pretty_notification"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.NotificationEventID, resp.Event)
				c := rmap["contract"].(string)
				require.Equal(t, "0x"+testContractHash, c)
				n := rmap["name"].(string)
				require.Equal(t, "my_pretty_notification", n)
			},
		},
		"execution matching state": {
			params: `["transaction_executed", {"state":"HALT"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.ExecutionEventID, resp.Event)
				st := rmap["vmstate"].(string)
				require.Equal(t, "HALT", st)
			},
		},
		"execution matching container": {
			params: `["transaction_executed", {"container":"` + deploymentTxHash + `"}]`,
			check: func(t *testing.T, resp *neorpc.Notification) {
				rmap := resp.Payload[0].(map[string]any)
				require.Equal(t, neorpc.ExecutionEventID, resp.Event)
				tx := rmap["container"].(string)
				require.Equal(t, "0x"+deploymentTxHash, tx)
			},
		},
		"execution matching state and container": {
			params: `["transaction_executed", {"state":"HALT", "container":"` + deploymentTxHash + `"}]`,
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
			params: `["transaction_added", {"sender":"00112233445566778899aabbccddeeff00112233"}]`,
			check: func(t *testing.T, _ *neorpc.Notification) {
				t.Fatal("unexpected match for EnrollmentTransaction")
			},
		},
		"notification non-matching": {
			params: `["notification_from_execution", {"contract":"00112233445566778899aabbccddeeff00112233"}]`,
			check: func(t *testing.T, _ *neorpc.Notification) {
				t.Fatal("unexpected match for contract 00112233445566778899aabbccddeeff00112233")
			},
		},
		"execution non-matching": {
			// We have single FAULTed transaction in chain, this, use the wrong hash for this test instead of FAULT state.
			params: `["transaction_executed", {"container":"0x` + util.Uint256{}.StringLE() + `"}]`,
			check: func(t *testing.T, n *neorpc.Notification) {
				t.Fatal("unexpected match for faulted execution")
			},
		},
		"header of added block": {
			params: `["header_of_added_block", {"primary": 0, "since": 5}]`,
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
			chain, rpcSrv, c, respMsgs, finishedFlag := initCleanServerAndWSClient(t)

			defer chain.Close()
			defer rpcSrv.Shutdown()

			// It's used as an end-of-event-stream, so it's always present.
			blockSubID := callSubscribe(t, c, respMsgs, `["block_added"]`)
			subID := callSubscribe(t, c, respMsgs, this.params)

			var lastBlock uint32
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
				this.check(t, resp)
			}

			callUnsubscribe(t, c, respMsgs, subID)
			callUnsubscribe(t, c, respMsgs, blockSubID)
			finishedFlag.CompareAndSwap(false, true)
			c.Close()
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

	chain, rpcSrv, c, respMsgs, finishedFlag := initCleanServerAndWSClient(t)
	go rpcSrv.coreServer.Start()

	defer chain.Close()
	defer rpcSrv.Shutdown()

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
	finishedFlag.CompareAndSwap(false, true)
	c.Close()
}

func TestFilteredBlockSubscriptions(t *testing.T) {
	// We can't fit this into TestFilteredSubscriptions, because it uses
	// blocks as EOF events to wait for.
	const numBlocks = 10
	chain, rpcSrv, c, respMsgs, finishedFlag := initCleanServerAndWSClient(t)

	defer chain.Close()
	defer rpcSrv.Shutdown()

	blockSubID := callSubscribe(t, c, respMsgs, `["block_added", {"primary":3}]`)

	var expectedCnt int
	for i := 0; i < numBlocks; i++ {
		primary := uint32(i % 4)
		if primary == 3 {
			expectedCnt++
		}
		b := testchain.NewBlock(t, chain, 1, primary)
		require.NoError(t, chain.AddBlock(b))
	}

	for i := 0; i < expectedCnt; i++ {
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
	finishedFlag.CompareAndSwap(false, true)
	c.Close()
}

func TestHeaderOfAddedBlockSubscriptions(t *testing.T) {
	const numBlocks = 10
	chain, rpcSrv, c, respMsgs, finishedFlag := initCleanServerAndWSClient(t)

	defer chain.Close()
	defer rpcSrv.Shutdown()

	headerSubID := callSubscribe(t, c, respMsgs, `["header_of_added_block", {"primary":3}]`)

	var expectedCnt int
	for i := 0; i < numBlocks; i++ {
		primary := uint32(i % 4)
		if primary == 3 {
			expectedCnt++
		}
		b := testchain.NewBlock(t, chain, 1, primary)
		require.NoError(t, chain.AddBlock(b))
	}

	for i := 0; i < expectedCnt; i++ {
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
	finishedFlag.CompareAndSwap(false, true)
	c.Close()
}

func TestMaxSubscriptions(t *testing.T) {
	var subIDs = make([]string, 0)
	chain, rpcSrv, c, respMsgs, finishedFlag := initCleanServerAndWSClient(t)

	defer chain.Close()
	defer rpcSrv.Shutdown()

	for i := 0; i < maxFeeds+1; i++ {
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

	finishedFlag.CompareAndSwap(false, true)
	c.Close()
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
	chain, rpcSrv, c, respMsgs, finishedFlag := initCleanServerAndWSClient(t)

	defer chain.Close()
	defer rpcSrv.Shutdown()

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

	finishedFlag.CompareAndSwap(false, true)
	c.Close()
}

func doSomeWSRequest(t *testing.T, ws *websocket.Conn) {
	require.NoError(t, ws.SetWriteDeadline(time.Now().Add(time.Second)))
	// It could be just about anything including invalid request,
	// we only care about server handling being active.
	require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc": "2.0", "method": "getversion", "params": [], "id": 1}`)))
	err := ws.SetReadDeadline(time.Now().Add(time.Second))
	require.NoError(t, err)
	_, _, err = ws.ReadMessage()
	require.NoError(t, err)
}

func TestWSClientsLimit(t *testing.T) {
	for tname, limit := range map[string]int{"default": 0, "8": 8, "disabled": -1} {
		effectiveClients := limit
		if limit == 0 {
			effectiveClients = defaultMaxWebSocketClients
		} else if limit < 0 {
			effectiveClients = 0
		}
		t.Run(tname, func(t *testing.T) {
			chain, rpcSrv, httpSrv := initClearServerWithCustomConfig(t, func(cfg *config.Config) {
				cfg.ApplicationConfiguration.RPC.MaxWebSocketClients = limit
			})
			defer chain.Close()
			defer rpcSrv.Shutdown()

			dialer := websocket.Dialer{HandshakeTimeout: time.Second}
			url := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"
			wss := make([]*websocket.Conn, effectiveClients)

			for i := 0; i < len(wss)+1; i++ {
				ws, r, err := dialer.Dial(url, nil)
				if r != nil && r.Body != nil {
					defer r.Body.Close()
				}
				if i < effectiveClients {
					require.NoError(t, err)
					wss[i] = ws
					// Check that it's completely ready.
					doSomeWSRequest(t, ws)
				} else {
					require.Error(t, err)
				}
			}
			// Check connections are still alive (it actually is necessary to add
			// some use of wss to keep connections alive).
			for i := 0; i < len(wss); i++ {
				doSomeWSRequest(t, wss[i])
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

	chain, rpcSrv, c, respMsgs, finishedFlag := initCleanServerAndWSClient(t)

	defer chain.Close()
	defer rpcSrv.Shutdown()

	resp := callWSGetRaw(t, c, `{"jsonrpc": "2.0","method": "subscribe","params": ["block_added"],"id": 1}`, respMsgs)
	require.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)

	// Push a lot of new blocks, but don't read events for them.
	for i := 0; i < blockCnt; i++ {
		b := testchain.NewBlock(t, chain, 1, 0)
		require.NoError(t, chain.AddBlock(b))
	}
	for i := 0; i < blockCnt; i++ {
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

	finishedFlag.CompareAndSwap(false, true)
	c.Close()
}
