package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func wsReader(t *testing.T, ws *websocket.Conn, msgCh chan<- []byte, isFinished *atomic.Bool) {
	for {
		ws.SetReadDeadline(time.Now().Add(time.Second))
		_, body, err := ws.ReadMessage()
		if isFinished.Load() {
			require.Error(t, err)
			break
		}
		require.NoError(t, err)
		msgCh <- body
	}
}

func callWSGetRaw(t *testing.T, ws *websocket.Conn, msg string, respCh <-chan []byte) *response.Raw {
	var resp = new(response.Raw)

	ws.SetWriteDeadline(time.Now().Add(time.Second))
	require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte(msg)))

	body := <-respCh
	require.NoError(t, json.Unmarshal(body, resp))
	return resp
}

func getNotification(t *testing.T, respCh <-chan []byte) *response.Notification {
	var resp = new(response.Notification)
	body := <-respCh
	require.NoError(t, json.Unmarshal(body, resp))
	return resp
}

func initCleanServerAndWSClient(t *testing.T) (*core.Blockchain, *Server, *websocket.Conn, chan []byte, *atomic.Bool) {
	chain, rpcSrv, httpSrv := initClearServerWithInMemoryChain(t)

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	url := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"
	ws, _, err := dialer.Dial(url, nil)
	require.NoError(t, err)

	// Use buffered channel to read server's messages and then read expected
	// responses from it.
	respMsgs := make(chan []byte, 16)
	finishedFlag := atomic.NewBool(false)
	go wsReader(t, ws, respMsgs, finishedFlag)
	return chain, rpcSrv, ws, respMsgs, finishedFlag
}

func TestSubscriptions(t *testing.T) {
	var subIDs = make([]string, 0)
	var subFeeds = []string{"block_added", "transaction_added", "notification_from_execution", "transaction_executed"}

	chain, rpcSrv, c, respMsgs, finishedFlag := initCleanServerAndWSClient(t)

	defer chain.Close()
	defer rpcSrv.Shutdown()

	for _, feed := range subFeeds {
		var s string
		resp := callWSGetRaw(t, c, fmt.Sprintf(`{
  "jsonrpc": "2.0",
  "method": "subscribe",
  "params": ["%s"],
  "id": 1
}`, feed), respMsgs)
		require.Nil(t, resp.Error)
		require.NotNil(t, resp.Result)
		require.NoError(t, json.Unmarshal(resp.Result, &s))
		subIDs = append(subIDs, s)
	}

	for _, b := range getTestBlocks(t) {
		require.NoError(t, chain.AddBlock(b))
		for _, tx := range b.Transactions {
			var mayNotify bool

			if tx.Type == transaction.InvocationType {
				resp := getNotification(t, respMsgs)
				require.Equal(t, response.ExecutionEventID, resp.Event)
				mayNotify = true
			}
			for {
				resp := getNotification(t, respMsgs)
				if mayNotify && resp.Event == response.NotificationEventID {
					continue
				}
				require.Equal(t, response.TransactionEventID, resp.Event)
				break
			}
		}
		resp := getNotification(t, respMsgs)
		require.Equal(t, response.BlockEventID, resp.Event)
	}

	for _, id := range subIDs {
		var b bool

		resp := callWSGetRaw(t, c, fmt.Sprintf(`{
  "jsonrpc": "2.0",
  "method": "unsubscribe",
  "params": ["%s"],
  "id": 1
}`, id), respMsgs)
		require.Nil(t, resp.Error)
		require.NotNil(t, resp.Result)
		require.NoError(t, json.Unmarshal(resp.Result, &b))
		require.Equal(t, true, b)
	}
	finishedFlag.CAS(false, true)
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

	finishedFlag.CAS(false, true)
	c.Close()
}

func TestBadSubUnsub(t *testing.T) {
	var subCases = map[string]string{
		"no params":              `{"jsonrpc": "2.0", "method": "subscribe", "params": [], "id": 1}`,
		"bad (non-string) event": `{"jsonrpc": "2.0", "method": "subscribe", "params": [1], "id": 1}`,
		"bad (wrong) event":      `{"jsonrpc": "2.0", "method": "subscribe", "params": ["block_removed"], "id": 1}`,
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

	finishedFlag.CAS(false, true)
	c.Close()
}

func doSomeWSRequest(t *testing.T, ws *websocket.Conn) {
	ws.SetWriteDeadline(time.Now().Add(time.Second))
	// It could be just about anything including invalid request,
	// we only care about server handling being active.
	require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc": "2.0", "method": "getversion", "params": [], "id": 1}`)))
	ws.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err := ws.ReadMessage()
	require.NoError(t, err)
}

func TestWSClientsLimit(t *testing.T) {
	chain, rpcSrv, httpSrv := initClearServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	url := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"
	wss := make([]*websocket.Conn, maxSubscribers)

	for i := 0; i < len(wss)+1; i++ {
		ws, _, err := dialer.Dial(url, nil)
		if i < maxSubscribers {
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
}
