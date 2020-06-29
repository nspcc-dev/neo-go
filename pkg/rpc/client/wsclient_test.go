package client

import (
	"context"
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
	defer srv.Close()
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{Network: netmode.UnitTestNet})
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
			return wsc.SubscribeForExecutionNotifications(nil)
		},
		"executions": func(wsc *WSClient) (string, error) {
			return wsc.SubscribeForTransactionExecutions(nil)
		},
	}
	t.Run("good", func(t *testing.T) {
		for name, f := range cases {
			t.Run(name, func(t *testing.T) {
				srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": "55aaff00"}`)
				defer srv.Close()
				wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{Network: netmode.UnitTestNet})
				require.NoError(t, err)
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
				defer srv.Close()
				wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{Network: netmode.UnitTestNet})
				require.NoError(t, err)
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
			defer srv.Close()
			wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{Network: netmode.UnitTestNet})
			require.NoError(t, err)
			rc.code(t, wsc)
		})
	}
}

func TestWSClientEvents(t *testing.T) {
	var ok bool
	// Events from RPC server test chain.
	var events = []string{
		`{"jsonrpc":"2.0","method":"transaction_executed","params":[{"txid":"0xe1cd5e57e721d2a2e05fb1f08721b12057b25ab1dd7fd0f33ee1639932fdfad7","trigger":"Application","vmstate":"HALT","gas_consumed":"2.291","stack":[],"notifications":[{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","state":{"type":"Array","value":[{"type":"ByteArray","value":"Y29udHJhY3QgY2FsbA=="},{"type":"ByteArray","value":"dHJhbnNmZXI="},{"type":"Array","value":[{"type":"ByteArray","value":"dpFiJB7t+XwkgWUq3xug9b9XQxs="},{"type":"ByteArray","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"Integer","value":"1000"}]}]}},{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","state":{"type":"Array","value":[{"type":"ByteArray","value":"dHJhbnNmZXI="},{"type":"ByteArray","value":"dpFiJB7t+XwkgWUq3xug9b9XQxs="},{"type":"ByteArray","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"Integer","value":"1000"}]}}]}]}`,
		`{"jsonrpc":"2.0","method":"notification_from_execution","params":[{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","state":{"type":"Array","value":[{"type":"ByteArray","value":"Y29udHJhY3QgY2FsbA=="},{"type":"ByteArray","value":"dHJhbnNmZXI="},{"type":"Array","value":[{"type":"ByteArray","value":"dpFiJB7t+XwkgWUq3xug9b9XQxs="},{"type":"ByteArray","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"Integer","value":"1000"}]}]}}]}`,
		`{"jsonrpc":"2.0","method":"transaction_executed","params":[{"txid":"0xf97a72b7722c109f909a8bc16c22368c5023d85828b09b127b237aace33cf099","trigger":"Application","vmstate":"HALT","gas_consumed":"0.0604261","stack":[],"notifications":[{"contract":"0xe65ff7b3a02d207b584a5c27057d4e9862ef01da","state":{"type":"Array","value":[{"type":"ByteArray","value":"Y29udHJhY3QgY2FsbA=="},{"type":"ByteArray","value":"dHJhbnNmZXI="},{"type":"Array","value":[{"type":"ByteArray","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"ByteArray","value":"IHKCdK+vw29DoHHTKM+j5inZy7A="},{"type":"Integer","value":"123"}]}]}},{"contract":"0xe65ff7b3a02d207b584a5c27057d4e9862ef01da","state":{"type":"Array","value":[{"type":"ByteArray","value":"dHJhbnNmZXI="},{"type":"ByteArray","value":"MW6FEDkBnTnfwsN9bD/uGf1YCYc="},{"type":"ByteArray","value":"IHKCdK+vw29DoHHTKM+j5inZy7A="},{"type":"Integer","value":"123"}]}}]}]}`,
		`{"jsonrpc":"2.0","method":"block_added","params":[{"hash":"0x2d312f6379ead13cf62634c703091b750e7903728df2a3cf5bd96ce80b84a849","version":0,"previousblockhash":"0xb8237d34c156cac6be7b01578decf8ac8c99a62f0b6f774d622aad7be0fe189d","merkleroot":"0xf89169e89361692b71e671f13c088e84c5325015c413e8f89e7ba38efdb41287","time":1592472500006,"index":6,"nextconsensus":"Nbb1qkwcwNSBs9pAnrVVrnFbWnbWBk91U2","witnesses":[{"invocation":"DEDblVguNGXWbUswDvBfVJzBt76BJyJ0Ga6siquyjioGn4Dbr6zy1IvcLl3xN5akcejRy9e+Mr1qvpe/gkLgtW4QDEDRwPISZagMFjE/plXTnZ/gEU0IbBAAe23U29zVWteUmzRsPxF/MdzXvdffR9W0edkj17AmkWpn+5rqzH9aCOpLDECEvjgxZaRoAHEDNzp1REllLcGzMwrwSjudtzfgRglQL3g1BKerDx6cGHH73medRVkL9QVm4KzSxlywVtvhwBMrDEBuPKvzg5TtakFW2jr/bfmy1bn2FiLARlOySwaGdKRV93ozA5lVEIAvHbBlJtT4/5H8jHjbncXXMrP3OUHqebZz","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBMHOzuw=="}],"consensus_data":{"primary":0,"nonce":"0000000000000457"},"tx":[{"hash":"0xf97a72b7722c109f909a8bc16c22368c5023d85828b09b127b237aace33cf099","size":265,"version":0,"nonce":9,"sender":"NQRLhCpAru9BjGsMwk67vdMwmzKMRgsnnN","sys_fee":"0","net_fee":"0.0036521","valid_until_block":1200,"attributes":[],"cosigners":[{"account":"0x870958fd19ee3f6c7dc3c2df399d013910856e31","scopes":"CalledByEntry"}],"script":"AHsMFCBygnSvr8NvQ6Bx0yjPo+Yp2cuwDBQxboUQOQGdOd/Cw31sP+4Z/VgJhxPADAh0cmFuc2ZlcgwU2gHvYphOfQUnXEpYeyAtoLP3X+ZBYn1bUjg=","scripts":[{"invocation":"DECwklSj3liZOJbktRtkVdUCu8U2LQlrU6Dv8NtMgd0xXbk5lXjc2p68xv6xtJXbJ4aoFMJZ9lkcNpGoeUCcaCet","verification":"DCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcILQQqQatQ="}]}]}]}`,
		`{"jsonrpc":"2.0","method":"event_missed","params":[]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/ws" && req.Method == "GET" {
			var upgrader = websocket.Upgrader{}
			ws, err := upgrader.Upgrade(w, req, nil)
			require.NoError(t, err)
			for _, event := range events {
				ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
				err = ws.WriteMessage(1, []byte(event))
				if err != nil {
					break
				}
			}
			ws.Close()
			return
		}
	}))

	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{Network: netmode.UnitTestNet})
	require.NoError(t, err)
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
	defer srv.Close()
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{Network: netmode.UnitTestNet})
	require.NoError(t, err)
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
				require.NotNil(t, param)
				require.Equal(t, request.BlockFilterT, param.Type)
				filt, ok := param.Value.(request.BlockFilter)
				require.Equal(t, true, ok)
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
				require.NotNil(t, param)
				require.Equal(t, request.TxFilterT, param.Type)
				filt, ok := param.Value.(request.TxFilter)
				require.Equal(t, true, ok)
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Sender)
				require.Nil(t, filt.Cosigner)
			},
		},
		{"transactions cosigner",
			func(t *testing.T, wsc *WSClient) {
				cosigner := util.Uint160{0, 42}
				_, err := wsc.SubscribeForNewTransactions(nil, &cosigner)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				require.NotNil(t, param)
				require.Equal(t, request.TxFilterT, param.Type)
				filt, ok := param.Value.(request.TxFilter)
				require.Equal(t, true, ok)
				require.Nil(t, filt.Sender)
				require.Equal(t, util.Uint160{0, 42}, *filt.Cosigner)
			},
		},
		{"transactions sender and cosigner",
			func(t *testing.T, wsc *WSClient) {
				sender := util.Uint160{1, 2, 3, 4, 5}
				cosigner := util.Uint160{0, 42}
				_, err := wsc.SubscribeForNewTransactions(&sender, &cosigner)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				require.NotNil(t, param)
				require.Equal(t, request.TxFilterT, param.Type)
				filt, ok := param.Value.(request.TxFilter)
				require.Equal(t, true, ok)
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Sender)
				require.Equal(t, util.Uint160{0, 42}, *filt.Cosigner)
			},
		},
		{"notifications",
			func(t *testing.T, wsc *WSClient) {
				contract := util.Uint160{1, 2, 3, 4, 5}
				_, err := wsc.SubscribeForExecutionNotifications(&contract)
				require.NoError(t, err)
			},
			func(t *testing.T, p *request.Params) {
				param := p.Value(1)
				require.NotNil(t, param)
				require.Equal(t, request.NotificationFilterT, param.Type)
				filt, ok := param.Value.(request.NotificationFilter)
				require.Equal(t, true, ok)
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, filt.Contract)
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
				require.NotNil(t, param)
				require.Equal(t, request.ExecutionFilterT, param.Type)
				filt, ok := param.Value.(request.ExecutionFilter)
				require.Equal(t, true, ok)
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
					ws.SetReadDeadline(time.Now().Add(2 * time.Second))
					req := request.In{}
					err = ws.ReadJSON(&req)
					require.NoError(t, err)
					params, err := req.Params()
					require.NoError(t, err)
					c.serverCode(t, params)
					ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
					err = ws.WriteMessage(1, []byte(`{"jsonrpc": "2.0", "id": 1, "result": "0"}`))
					require.NoError(t, err)
					ws.Close()
				}
			}))
			defer srv.Close()
			wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{Network: netmode.UnitTestNet})
			require.NoError(t, err)
			c.clientCode(t, wsc)
			wsc.Close()
		})
	}
}

func TestNewWS(t *testing.T) {
	srv := initTestServer(t, "")
	defer srv.Close()

	t.Run("good", func(t *testing.T) {
		_, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{Network: netmode.UnitTestNet})
		require.NoError(t, err)
	})
	t.Run("bad URL", func(t *testing.T) {
		_, err := NewWS(context.TODO(), strings.Trim(srv.URL, "http://"), Options{Network: netmode.UnitTestNet})
		require.Error(t, err)
	})
}
