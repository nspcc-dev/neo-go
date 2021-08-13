package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestWSClientClose(t *testing.T) {
	srv := initTestServer(t, "")
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
	require.NoError(t, err)
	wsc.Close()
}

func TestWSClientSubscription(t *testing.T) {
	var cases = map[string]func(*WSClient) (string, error){
		"blocks": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForNewBlocks(nil)
		},
		"transactions": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForNewTransactions(nil, nil)
		},
		"notifications": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForExecutionNotifications(nil, nil)
		},
		"executions": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForTransactionExecutions(nil)
		},
	}
	t.Run("good", func(t *testing.T) {
		for name, f := range cases {
			t.Run(name, func(t *testing.T) {
				srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": "55aaff00"}`)
				wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
				require.NoError(t, err)
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
			wsc.subscriptions["0"] = true
			err := wsc.Unsubscribe("0")
			require.NoError(t, err)
		}},
		"all": {`{"jsonrpc": "2.0", "id": 1, "result": true}`, func(t *testing.T, wsc *WSClient) {
			// We can't really subscribe using this stub server, so set up wsc internals.
			wsc.subscriptions["0"] = true
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
			wsc.subscriptions["0"] = true
			err := wsc.Unsubscribe("0")
			require.Error(t, err)
		}},
		"false returned": {`{"jsonrpc": "2.0", "id": 1, "result": false}`, func(t *testing.T, wsc *WSClient) {
			// We can't really subscribe using this stub server, so set up wsc internals.
			wsc.subscriptions["0"] = true
			err := wsc.Unsubscribe("0")
			require.Error(t, err)
		}},
	}
	for name, rc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := initTestServer(t, rc.response)
			wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
			require.NoError(t, err)
			require.NoError(t, wsc.Init())
			rc.code(t, wsc)
		})
	}
}

func TestWSClientEvents(t *testing.T) {
	var ok bool
	// Events from RPC server test chain.
	var events = []string{
		`{"jsonrpc":"2.0","method":"transaction_executed","params":[{"container":"0xe1cd5e57e721d2a2e05fb1f08721b12057b25ab1dd7fd0f33ee1639932fdfad7","trigger":"Application","vmstate":"HALT","gasconsumed":"22910000","stack":[],"notifications":[{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","eventname":"contract call","state":{"type":"Array","value":[{"type":"ByteString","value":"dHJhbnNmZXI="},{"type":"Array","value":[{"type":"ByteString","value":"dpFiJB7t+XwkgWUq3xug9b9XQxs="},{"type":"ByteString","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"Integer","value":"1000"}]}]}},{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","eventname":"transfer","state":{"type":"Array","value":[{"type":"ByteString","value":"dpFiJB7t+XwkgWUq3xug9b9XQxs="},{"type":"ByteString","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"Integer","value":"1000"}]}}]}]}`,
		`{"jsonrpc":"2.0","method":"notification_from_execution","params":[{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","eventname":"contract call","state":{"type":"Array","value":[{"type":"ByteString","value":"dHJhbnNmZXI="},{"type":"Array","value":[{"type":"ByteString","value":"dpFiJB7t+XwkgWUq3xug9b9XQxs="},{"type":"ByteString","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"Integer","value":"1000"}]}]}}]}`,
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

	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
	require.NoError(t, err)
	wsc.network = netmode.UnitTestNet
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
}

func TestWSExecutionVMStateCheck(t *testing.T) {
	// Will answer successfully if request slips through.
	srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": "55aaff00"}`)
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
	require.NoError(t, err)
	require.NoError(t, wsc.Init())
	filter := "NONE"
	_, err = wsc.SubscribeForTransactionExecutions(&filter)
	require.Error(t, err)
	wsc.Close()
}

func TestWSFilteredSubscriptions(t *testing.T) {
	var cases = []struct {
		name       string
		clientCode func(*testing.T, *WSClient)
		serverCode func(*testing.T, *request.Params)
	}{
		{"blocks",
			func(t *testing.T, wsc *WSClient) {
				primary := 3
				_, err := wsc.SubscribeForNewBlocks(&primary)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				raw, ok := param.Value.(json.RawMessage)
				require.True(t, ok)
				filt := new(request.BlockFilter)
				require.NoError(t, json.Unmarshal(raw, filt))
				require.Equal(t, 3, filt.Primary)
			},
		},
		{"transactions sender",
			func(t *testing.T, wsc *WSClient) {
				sender := util.Uint160{1, 2, 3, 4, 5}
				_, err := wsc.SubscribeForNewTransactions(&sender, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				raw, ok := param.Value.(json.RawMessage)
				require.True(t, ok)
				filt := new(request.TxFilter)
				require.NoError(t, json.Unmarshal(raw, filt))
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Sender)
				require.Nil(t, filt.Signer)
			},
		},
		{"transactions signer",
			func(t *testing.T, wsc *WSClient) {
				signer := util.Uint160{0, 42}
				_, err := wsc.SubscribeForNewTransactions(nil, &signer)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				raw, ok := param.Value.(json.RawMessage)
				require.True(t, ok)
				filt := new(request.TxFilter)
				require.NoError(t, json.Unmarshal(raw, filt))
				require.Nil(t, filt.Sender)
				require.Equal(t, util.Uint160{0, 42}, *filt.Signer)
			},
		},
		{"transactions sender and signer",
			func(t *testing.T, wsc *WSClient) {
				sender := util.Uint160{1, 2, 3, 4, 5}
				signer := util.Uint160{0, 42}
				_, err := wsc.SubscribeForNewTransactions(&sender, &signer)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				raw, ok := param.Value.(json.RawMessage)
				require.True(t, ok)
				filt := new(request.TxFilter)
				require.NoError(t, json.Unmarshal(raw, filt))
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Sender)
				require.Equal(t, util.Uint160{0, 42}, *filt.Signer)
			},
		},
		{"notifications contract hash",
			func(t *testing.T, wsc *WSClient) {
				contract := util.Uint160{1, 2, 3, 4, 5}
				_, err := wsc.SubscribeForExecutionNotifications(&contract, nil)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				raw, ok := param.Value.(json.RawMessage)
				require.True(t, ok)
				filt := new(request.NotificationFilter)
				require.NoError(t, json.Unmarshal(raw, filt))
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Contract)
				require.Nil(t, filt.Name)
			},
		},
		{"notifications name",
			func(t *testing.T, wsc *WSClient) {
				name := "my_pretty_notification"
				_, err := wsc.SubscribeForExecutionNotifications(nil, &name)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				raw, ok := param.Value.(json.RawMessage)
				require.True(t, ok)
				filt := new(request.NotificationFilter)
				require.NoError(t, json.Unmarshal(raw, filt))
				require.Equal(t, "my_pretty_notification", *filt.Name)
				require.Nil(t, filt.Contract)
			},
		},
		{"notifications contract hash and name",
			func(t *testing.T, wsc *WSClient) {
				contract := util.Uint160{1, 2, 3, 4, 5}
				name := "my_pretty_notification"
				_, err := wsc.SubscribeForExecutionNotifications(&contract, &name)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				raw, ok := param.Value.(json.RawMessage)
				require.True(t, ok)
				filt := new(request.NotificationFilter)
				require.NoError(t, json.Unmarshal(raw, filt))
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Contract)
				require.Equal(t, "my_pretty_notification", *filt.Name)
			},
		},
		{"executions",
			func(t *testing.T, wsc *WSClient) {
				state := "FAULT"
				_, err := wsc.SubscribeForTransactionExecutions(&state)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				raw, ok := param.Value.(json.RawMessage)
				require.True(t, ok)
				filt := new(request.ExecutionFilter)
				require.NoError(t, json.Unmarshal(raw, filt))
				require.Equal(t, "FAULT", filt.State)
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
					req := request.In{}
					err = ws.ReadJSON(&req)
					require.NoError(t, err)
					params, err := req.Params()
					require.NoError(t, err)
					c.serverCode(t, params)
					err = ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
					require.NoError(t, err)
					err = ws.WriteMessage(1, []byte(`{"jsonrpc": "2.0", "id": 1, "result": "0"}`))
					require.NoError(t, err)
					ws.Close()
				}
			}))
			wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
			require.NoError(t, err)
			wsc.network = netmode.UnitTestNet
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
		require.NoError(t, c.Init())
	})
	t.Run("bad URL", func(t *testing.T) {
		_, err := NewWS(context.TODO(), strings.TrimPrefix(srv.URL, "http://"), Options{})
		require.Error(t, err)
	})
}
