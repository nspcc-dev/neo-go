package rpcclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv/params"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestWSClientClose(t *testing.T) {
	srv := initTestServer(t, "")
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
	require.NoError(t, err)
	wsc.Close()
}

func TestWSClientSubscription(t *testing.T) {
	ch := make(chan Notification)
	var cases = map[string]func(*WSClient) (string, error){
		"blocks": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForNewBlocksWithChan(nil, nil, nil, nil)
		},
		"blocks_with_custom_ch": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForNewBlocksWithChan(nil, nil, nil, ch)
		},
		"transactions": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForNewTransactionsWithChan(nil, nil, nil)
		},
		"transactions_with_custom_ch": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForNewTransactionsWithChan(nil, nil, ch)
		},
		"notifications": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForExecutionNotificationsWithChan(nil, nil, nil)
		},
		"notifications_with_custom_ch": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForExecutionNotificationsWithChan(nil, nil, ch)
		},
		"executions": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForTransactionExecutionsWithChan(nil, nil, nil)
		},
		"executions_with_custom_ch": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForTransactionExecutionsWithChan(nil, nil, ch)
		},
	}
	t.Run("good", func(t *testing.T) {
		for name, f := range cases {
			t.Run(name, func(t *testing.T) {
				srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": "55aaff00"}`)
				wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
				require.NoError(t, err)
				wsc.getNextRequestID = getTestRequestID
				require.NoError(t, wsc.Init())
				id, err := f(wsc)
				require.NoError(t, err)
				require.Equal(t, "55aaff00", id)
			})
		}
	})
	t.Run("bad", func(t *testing.T) {
		for name, f := range cases {
			t.Run(name, func(t *testing.T) {
				srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "error":{"code":-32602,"message":"Invalid Params"}}`)
				wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
				require.NoError(t, err)
				wsc.getNextRequestID = getTestRequestID
				require.NoError(t, wsc.Init())
				_, err = f(wsc)
				require.Error(t, err)
			})
		}
	})
}

func TestWSClientUnsubscription(t *testing.T) {
	type responseCheck struct {
		response string
		code     func(*testing.T, *WSClient)
	}
	var cases = map[string]responseCheck{
		"good": {`{"jsonrpc": "2.0", "id": 1, "result": true}`, func(t *testing.T, wsc *WSClient) {
			// We can't really subscribe using this stub server, so set up wsc internals.
			wsc.subscriptions["0"] = notificationReceiver{}
			err := wsc.Unsubscribe("0")
			require.NoError(t, err)
		}},
		"all": {`{"jsonrpc": "2.0", "id": 1, "result": true}`, func(t *testing.T, wsc *WSClient) {
			// We can't really subscribe using this stub server, so set up wsc internals.
			wsc.subscriptions["0"] = notificationReceiver{}
			err := wsc.UnsubscribeAll()
			require.NoError(t, err)
			require.Equal(t, 0, len(wsc.subscriptions))
		}},
		"not subscribed": {`{"jsonrpc": "2.0", "id": 1, "result": true}`, func(t *testing.T, wsc *WSClient) {
			err := wsc.Unsubscribe("0")
			require.Error(t, err)
		}},
		"error returned": {`{"jsonrpc": "2.0", "id": 1, "error":{"code":-32602,"message":"Invalid Params"}}`, func(t *testing.T, wsc *WSClient) {
			// We can't really subscribe using this stub server, so set up wsc internals.
			wsc.subscriptions["0"] = notificationReceiver{}
			err := wsc.Unsubscribe("0")
			require.Error(t, err)
		}},
		"false returned": {`{"jsonrpc": "2.0", "id": 1, "result": false}`, func(t *testing.T, wsc *WSClient) {
			// We can't really subscribe using this stub server, so set up wsc internals.
			wsc.subscriptions["0"] = notificationReceiver{}
			err := wsc.Unsubscribe("0")
			require.Error(t, err)
		}},
	}
	for name, rc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := initTestServer(t, rc.response)
			wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
			require.NoError(t, err)
			wsc.getNextRequestID = getTestRequestID
			require.NoError(t, wsc.Init())
			rc.code(t, wsc)
		})
	}
}

func TestWSClientEvents(t *testing.T) {
	var ok bool
	// Events from RPC server testchain.
	var events = []string{
		`{"jsonrpc":"2.0","method":"transaction_executed","params":[{"container":"0xe1cd5e57e721d2a2e05fb1f08721b12057b25ab1dd7fd0f33ee1639932fdfad7","trigger":"Application","vmstate":"HALT","gasconsumed":"22910000","stack":[],"notifications":[{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","eventname":"contract call","state":{"type":"Array","value":[{"type":"ByteString","value":"dHJhbnNmZXI="},{"type":"Array","value":[{"type":"ByteString","value":"dpFiJB7t+XwkgWUq3xug9b9XQxs="},{"type":"ByteString","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"Integer","value":"1000"}]}]}},{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","eventname":"transfer","state":{"type":"Array","value":[{"type":"ByteString","value":"dpFiJB7t+XwkgWUq3xug9b9XQxs="},{"type":"ByteString","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"Integer","value":"1000"}]}}]}]}`,
		`{"jsonrpc":"2.0","method":"notification_from_execution","params":[{"container":"0xe1cd5e57e721d2a2e05fb1f08721b12057b25ab1dd7fd0f33ee1639932fdfad7","contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","eventname":"contract call","state":{"type":"Array","value":[{"type":"ByteString","value":"dHJhbnNmZXI="},{"type":"Array","value":[{"type":"ByteString","value":"dpFiJB7t+XwkgWUq3xug9b9XQxs="},{"type":"ByteString","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"Integer","value":"1000"}]}]}}]}`,
		`{"jsonrpc":"2.0","method":"transaction_executed","params":[{"container":"0xf97a72b7722c109f909a8bc16c22368c5023d85828b09b127b237aace33cf099","trigger":"Application","vmstate":"HALT","gasconsumed":"6042610","stack":[],"notifications":[{"contract":"0xe65ff7b3a02d207b584a5c27057d4e9862ef01da","eventname":"contract call","state":{"type":"Array","value":[{"type":"ByteString","value":"dHJhbnNmZXI="},{"type":"Array","value":[{"type":"ByteString","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"ByteString","value":"IHKCdK+vw29DoHHTKM+j5inZy7A="},{"type":"Integer","value":"123"}]}]}},{"contract":"0xe65ff7b3a02d207b584a5c27057d4e9862ef01da","eventname":"transfer","state":{"type":"Array","value":[{"type":"ByteString","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"ByteString","value":"IHKCdK+vw29DoHHTKM+j5inZy7A="},{"type":"Integer","value":"123"}]}}]}]}`,
		fmt.Sprintf(`{"jsonrpc":"2.0","method":"block_added","params":[%s]}`, b1Verbose),
		`{"jsonrpc":"2.0","method":"event_missed","params":[]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/ws" && req.Method == "GET" {
			var upgrader = websocket.Upgrader{}
			ws, err := upgrader.Upgrade(w, req, nil)
			require.NoError(t, err)
			for _, event := range events {
				err = ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
				require.NoError(t, err)
				err = ws.WriteMessage(1, []byte(event))
				if err != nil {
					break
				}
			}
			ws.Close()
			return
		}
	}))

	t.Run("default ntf channel", func(t *testing.T) {
		wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
		require.NoError(t, err)
		wsc.getNextRequestID = getTestRequestID
		wsc.cacheLock.Lock()
		wsc.cache.initDone = true // Our server mock is restricted, so perform initialisation manually.
		wsc.cache.network = netmode.UnitTestNet
		wsc.cacheLock.Unlock()
		// Our server mock is restricted, so perform subscriptions manually with default notifications channel.
		wsc.subscriptionsLock.Lock()
		wsc.subscriptions["0"] = notificationReceiver{typ: neorpc.BlockEventID, ch: wsc.Notifications}
		wsc.subscriptions["1"] = notificationReceiver{typ: neorpc.ExecutionEventID, ch: wsc.Notifications}
		wsc.subscriptions["2"] = notificationReceiver{typ: neorpc.NotificationEventID, ch: wsc.Notifications}
		// MissedEvent must be delivered without subscription.
		wsc.subscriptionsLock.Unlock()
		for range events {
			select {
			case _, ok = <-wsc.Notifications:
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for event")
			}
			require.True(t, ok)
		}
		select {
		case _, ok = <-wsc.Notifications:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
		// Connection closed by server.
		require.False(t, ok)
	})
	t.Run("multiple ntf channels", func(t *testing.T) {
		wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
		require.NoError(t, err)
		wsc.getNextRequestID = getTestRequestID
		wsc.cacheLock.Lock()
		wsc.cache.initDone = true // Our server mock is restricted, so perform initialisation manually.
		wsc.cache.network = netmode.UnitTestNet
		wsc.cacheLock.Unlock()

		// Our server mock is restricted, so perform subscriptions manually with default notifications channel.
		ch1 := make(chan Notification)
		ch2 := make(chan Notification)
		ch3 := make(chan Notification)
		halt := "HALT"
		fault := "FAULT"
		wsc.subscriptionsLock.Lock()
		wsc.subscriptions["0"] = notificationReceiver{typ: neorpc.BlockEventID, ch: wsc.Notifications}
		wsc.subscriptions["1"] = notificationReceiver{typ: neorpc.ExecutionEventID, ch: wsc.Notifications}
		wsc.subscriptions["2"] = notificationReceiver{typ: neorpc.NotificationEventID, ch: wsc.Notifications}
		wsc.subscriptions["3"] = notificationReceiver{typ: neorpc.BlockEventID, ch: ch1}
		wsc.subscriptions["4"] = notificationReceiver{typ: neorpc.NotificationEventID, ch: ch2}
		wsc.subscriptions["5"] = notificationReceiver{typ: neorpc.NotificationEventID, ch: ch2} // check duplicating subscriptions
		wsc.subscriptions["6"] = notificationReceiver{typ: neorpc.ExecutionEventID, filter: neorpc.ExecutionFilter{State: &halt}, ch: ch2}
		wsc.subscriptions["7"] = notificationReceiver{typ: neorpc.ExecutionEventID, filter: neorpc.ExecutionFilter{State: &fault}, ch: ch3}
		// MissedEvent must be delivered without subscription.
		wsc.subscriptionsLock.Unlock()

		var (
			defaultChCnt           int
			ch1Cnt                 int
			ch2Cnt                 int
			ch3Cnt                 int
			expectedDefaultCnCount = len(events)
			expectedCh1Cnt         = 1 + 1     // Block event + Missed event
			expectedCh2Cnt         = 1 + 2 + 1 // Notification event + 2 Execution events + Missed event
			expectedCh3Cnt         = 1         // Missed event
			ntf                    Notification
		)
		for i := 0; i < expectedDefaultCnCount+expectedCh1Cnt+expectedCh2Cnt+expectedCh3Cnt; i++ {
			select {
			case ntf, ok = <-wsc.Notifications:
				defaultChCnt++
			case ntf, ok = <-ch1:
				require.True(t, ntf.Type == neorpc.BlockEventID || ntf.Type == neorpc.MissedEventID, ntf.Type)
				ch1Cnt++
			case ntf, ok = <-ch2:
				require.True(t, ntf.Type == neorpc.NotificationEventID || ntf.Type == neorpc.MissedEventID || ntf.Type == neorpc.ExecutionEventID)
				ch2Cnt++
			case ntf, ok = <-ch3:
				require.True(t, ntf.Type == neorpc.MissedEventID)
				ch3Cnt++
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for event")
			}
			require.True(t, ok)
		}
		select {
		case _, ok = <-wsc.Notifications:
		case _, ok = <-ch1:
		case _, ok = <-ch2:
		case _, ok = <-ch3:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
		// Connection closed by server.
		require.False(t, ok)
		require.Equal(t, expectedDefaultCnCount, defaultChCnt)
		require.Equal(t, expectedCh1Cnt, ch1Cnt)
		require.Equal(t, expectedCh2Cnt, ch2Cnt)
		require.Equal(t, expectedCh3Cnt, ch3Cnt)
	})
}

func TestWSExecutionVMStateCheck(t *testing.T) {
	// Will answer successfully if request slips through.
	srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": "55aaff00"}`)
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
	require.NoError(t, err)
	wsc.getNextRequestID = getTestRequestID
	require.NoError(t, wsc.Init())
	filter := "NONE"
	_, err = wsc.SubscribeForTransactionExecutionsWithChan(&filter, nil, nil)
	require.Error(t, err)
	wsc.Close()
}

func TestWSFilteredSubscriptions(t *testing.T) {
	var cases = []struct {
		name       string
		clientCode func(*testing.T, *WSClient)
		serverCode func(*testing.T, *params.Params)
	}{
		{"blocks primary",
			func(t *testing.T, wsc *WSClient) {
				primary := 3
				_, err := wsc.SubscribeForNewBlocksWithChan(&primary, nil, nil, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, 3, *filt.Primary)
				require.Equal(t, (*uint32)(nil), filt.Since)
				require.Equal(t, (*uint32)(nil), filt.Till)
			},
		},
		{"blocks since",
			func(t *testing.T, wsc *WSClient) {
				var since uint32 = 3
				_, err := wsc.SubscribeForNewBlocksWithChan(nil, &since, nil, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, (*int)(nil), filt.Primary)
				require.Equal(t, uint32(3), *filt.Since)
				require.Equal(t, (*uint32)(nil), filt.Till)
			},
		},
		{"blocks till",
			func(t *testing.T, wsc *WSClient) {
				var till uint32 = 3
				_, err := wsc.SubscribeForNewBlocksWithChan(nil, nil, &till, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, (*int)(nil), filt.Primary)
				require.Equal(t, (*uint32)(nil), filt.Since)
				require.Equal(t, (uint32)(3), *filt.Till)
			},
		},
		{"blocks primary, since and till",
			func(t *testing.T, wsc *WSClient) {
				var (
					since   uint32 = 3
					primary        = 2
					till    uint32 = 5
				)
				_, err := wsc.SubscribeForNewBlocksWithChan(&primary, &since, &till, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, 2, *filt.Primary)
				require.Equal(t, uint32(3), *filt.Since)
				require.Equal(t, uint32(5), *filt.Till)
			},
		},
		{"transactions sender",
			func(t *testing.T, wsc *WSClient) {
				sender := util.Uint160{1, 2, 3, 4, 5}
				_, err := wsc.SubscribeForNewTransactionsWithChan(&sender, nil, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.TxFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Sender)
				require.Nil(t, filt.Signer)
			},
		},
		{"transactions signer",
			func(t *testing.T, wsc *WSClient) {
				signer := util.Uint160{0, 42}
				_, err := wsc.SubscribeForNewTransactionsWithChan(nil, &signer, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.TxFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Nil(t, filt.Sender)
				require.Equal(t, util.Uint160{0, 42}, *filt.Signer)
			},
		},
		{"transactions sender and signer",
			func(t *testing.T, wsc *WSClient) {
				sender := util.Uint160{1, 2, 3, 4, 5}
				signer := util.Uint160{0, 42}
				_, err := wsc.SubscribeForNewTransactionsWithChan(&sender, &signer, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.TxFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Sender)
				require.Equal(t, util.Uint160{0, 42}, *filt.Signer)
			},
		},
		{"notifications contract hash",
			func(t *testing.T, wsc *WSClient) {
				contract := util.Uint160{1, 2, 3, 4, 5}
				_, err := wsc.SubscribeForExecutionNotificationsWithChan(&contract, nil, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.NotificationFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Contract)
				require.Nil(t, filt.Name)
			},
		},
		{"notifications name",
			func(t *testing.T, wsc *WSClient) {
				name := "my_pretty_notification"
				_, err := wsc.SubscribeForExecutionNotificationsWithChan(nil, &name, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.NotificationFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, "my_pretty_notification", *filt.Name)
				require.Nil(t, filt.Contract)
			},
		},
		{"notifications contract hash and name",
			func(t *testing.T, wsc *WSClient) {
				contract := util.Uint160{1, 2, 3, 4, 5}
				name := "my_pretty_notification"
				_, err := wsc.SubscribeForExecutionNotificationsWithChan(&contract, &name, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.NotificationFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Contract)
				require.Equal(t, "my_pretty_notification", *filt.Name)
			},
		},
		{"executions state",
			func(t *testing.T, wsc *WSClient) {
				state := "FAULT"
				_, err := wsc.SubscribeForTransactionExecutionsWithChan(&state, nil, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.ExecutionFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, "FAULT", *filt.State)
				require.Equal(t, (*util.Uint256)(nil), filt.Container)
			},
		},
		{"executions container",
			func(t *testing.T, wsc *WSClient) {
				container := util.Uint256{1, 2, 3}
				_, err := wsc.SubscribeForTransactionExecutionsWithChan(nil, &container, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.ExecutionFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, (*string)(nil), filt.State)
				require.Equal(t, util.Uint256{1, 2, 3}, *filt.Container)
			},
		},
		{"executions state and container",
			func(t *testing.T, wsc *WSClient) {
				state := "FAULT"
				container := util.Uint256{1, 2, 3}
				_, err := wsc.SubscribeForTransactionExecutionsWithChan(&state, &container, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.ExecutionFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, "FAULT", *filt.State)
				require.Equal(t, util.Uint256{1, 2, 3}, *filt.Container)
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path == "/ws" && req.Method == "GET" {
					var upgrader = websocket.Upgrader{}
					ws, err := upgrader.Upgrade(w, req, nil)
					require.NoError(t, err)
					err = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
					require.NoError(t, err)
					req := params.In{}
					err = ws.ReadJSON(&req)
					require.NoError(t, err)
					params := params.Params(req.RawParams)
					c.serverCode(t, &params)
					err = ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
					require.NoError(t, err)
					err = ws.WriteMessage(1, []byte(`{"jsonrpc": "2.0", "id": 1, "result": "0"}`))
					require.NoError(t, err)
					ws.Close()
				}
			}))
			wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
			require.NoError(t, err)
			wsc.getNextRequestID = getTestRequestID
			wsc.cache.network = netmode.UnitTestNet
			c.clientCode(t, wsc)
			wsc.Close()
		})
	}
}

func TestNewWS(t *testing.T) {
	srv := initTestServer(t, "")

	t.Run("good", func(t *testing.T) {
		c, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
		require.NoError(t, err)
		c.getNextRequestID = getTestRequestID
		c.cache.network = netmode.UnitTestNet
		require.NoError(t, c.Init())
	})
	t.Run("bad URL", func(t *testing.T) {
		_, err := NewWS(context.TODO(), strings.TrimPrefix(srv.URL, "http://"), Options{})
		require.Error(t, err)
	})
}

func TestWSConcurrentAccess(t *testing.T) {
	var ids struct {
		lock sync.RWMutex
		m    map[int]struct{}
	}
	ids.m = make(map[int]struct{})

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
				i, err := strconv.Atoi(string(r.RawID))
				require.NoError(t, err)
				ids.lock.Lock()
				ids.m[i] = struct{}{}
				ids.lock.Unlock()
				var response string
				// Different responses to catch possible unmarshalling errors connected with invalid IDs distribution.
				switch r.Method {
				case "getblockcount":
					response = fmt.Sprintf(`{"id":%s,"jsonrpc":"2.0","result":123}`, r.RawID)
				case "getversion":
					response = fmt.Sprintf(`{"id":%s,"jsonrpc":"2.0","result":{"network":42,"tcpport":20332,"wsport":20342,"nonce":2153672787,"useragent":"/NEO-GO:0.73.1-pre-273-ge381358/"}}`, r.RawID)
				case "getblockhash":
					response = fmt.Sprintf(`{"id":%s,"jsonrpc":"2.0","result":"0x157ca5e5b8cf8f84c9660502a3270b346011612bded1514a6847f877c433a9bb"}`, r.RawID)
				}
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
	}))
	t.Cleanup(srv.Close)

	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
	require.NoError(t, err)
	batchCount := 100
	completed := atomic.NewInt32(0)
	for i := 0; i < batchCount; i++ {
		go func() {
			_, err := wsc.GetBlockCount()
			require.NoError(t, err)
			completed.Inc()
		}()
		go func() {
			_, err := wsc.GetBlockHash(123)
			require.NoError(t, err)
			completed.Inc()
		}()

		go func() {
			_, err := wsc.GetVersion()
			require.NoError(t, err)
			completed.Inc()
		}()
	}
	require.Eventually(t, func() bool {
		return int(completed.Load()) == batchCount*3
	}, time.Second, 100*time.Millisecond)

	ids.lock.RLock()
	require.True(t, len(ids.m) > batchCount)
	idsList := make([]int, 0, len(ids.m))
	for i := range ids.m {
		idsList = append(idsList, i)
	}
	ids.lock.RUnlock()

	sort.Ints(idsList)
	require.Equal(t, 1, idsList[0])
	require.Less(t, idsList[len(idsList)-1],
		batchCount*3+1) // batchCount*requestsPerBatch+1
	wsc.Close()
}

func TestWSDoubleClose(t *testing.T) {
	srv := initTestServer(t, "")

	c, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		c.Close()
		c.Close()
	})
}

func TestWS_RequestAfterClose(t *testing.T) {
	srv := initTestServer(t, "")

	c, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
	require.NoError(t, err)

	c.Close()

	require.NotPanics(t, func() {
		_, err = c.GetBlockCount()
	})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "connection lost before registering response channel"))
}

func TestWSClient_ConnClosedError(t *testing.T) {
	t.Run("standard closing", func(t *testing.T) {
		srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": 123}`)
		c, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
		require.NoError(t, err)

		// Check client is working.
		_, err = c.GetBlockCount()
		require.NoError(t, err)
		err = c.GetError()
		require.NoError(t, err)

		c.Close()
		err = c.GetError()
		require.NoError(t, err)
	})

	t.Run("malformed request", func(t *testing.T) {
		srv := initTestServer(t, "")
		c, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
		require.NoError(t, err)

		defaultMaxBlockSize := 262144
		_, err = c.SubmitP2PNotaryRequest(&payload.P2PNotaryRequest{
			MainTransaction: &transaction.Transaction{
				Script: make([]byte, defaultMaxBlockSize*3),
			},
			FallbackTransaction: &transaction.Transaction{},
		})
		require.Error(t, err)

		err = c.GetError()
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "failed to read JSON response (timeout/connection loss/malformed response)"), err.Error())
	})
}
