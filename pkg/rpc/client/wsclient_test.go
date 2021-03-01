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
		`{"jsonrpc":"2.0","method":"block_added","params":[{"size":1641,"nextblockhash":"0x003abea54aa3c5edba7e33fb7ca96452cb65ff8cd36ce1cdfd412a6c4d3ea38a","confirmations":6,"hash":"0xd9518e322440714b0564d6f84a9a39b527b5480e4e7f7932895777a4c8fa0a9e","version":0,"previousblockhash":"0xa496577895eb8c227bb866dc44f99f21c0cf06417ca8f2a877cc5d761a50dac0","merkleroot":"0x2b0f84636d814f3a952de145c8f4028f5664132f2719f5902e1884c9fba59806","time":1596101407001,"index":1,"nextconsensus":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","witnesses":[{"invocation":"DEANAGtuw7+VLVNvmpESGL4+xqlKgBSIWmMBEtABi86ixft2Q7AcaOC89M+yKVIuTel9doVJcCvfx93CcQ63DZqCDEBtwUEkjuzP9h8ZTL0GEKfGr01pazmh8s2TswJge5sAGryYE/+kjw5NCLFmowhPU73qUYQ9jq1zMNMXF+Deqxp/DEDkytkkwJec5n4x2+l5zsZHT6QTXJsByZOWXaGPVJKK8CeDccZba7Mf4MdSkWqSt61xUtlgM2Iqhe/Iuokf/ZEXDEAOH72S12CuAxVu0XNGyj3cgMtad+Bghxvr16T9+ELaWkpR4ko26FdStYC2XiCkzanXTtAD1Id5rREsxfFeKb83","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}],"consensusdata":{"primary":0,"nonce":"0000000000000457"},"tx":[{"hash":"0x32f9bd3a2707475407c41bf5daacf9560e25ed74f6d85b3afb2ef72edb2325ba","size":555,"version":0,"nonce":2,"sender":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","sysfee":"10000000","netfee":"4488350","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x95307cb9cc8c4578cef9f6845895eb7aa8be125e","scopes":"CalledByEntry"}],"script":"Ahjd9QUMFKqKz4WdT+QCs05nPyFWgheWpIjrDBReEr6oeuuVWIT2+c54RYzMuXwwlRPADAh0cmFuc2ZlcgwUJQWey0h406h1+RxRzt7TMNRXX95BYn1bUjg=","witnesses":[{"invocation":"DEAIcSUsAtRql4t+IEeo+p4+YI7bA6PG+1xxUkPIb2vNlaMl4PumjQVFT+bg2ldxCYa6zccoc4n0Gfryi82EhGpGDECR4fQDr4njo94mF6/GA+OH0Y5k735yGMEZHs96586BRp6f0AQxfmIPvLcS4Yero9p0zgVl9BDg3TxU5piRylR5DEAcjOT7JjEwNRnKgDDkXfh63Yc3MorMbdb2asTiDu0aexy5M5XcikA1jypJT4wkhxjp0rrgFZRSzeYhwV0Klz+yDECIopKxLd4p+hLHxFq07WffXd++sN0WIRWzvMJncCrJqSP8zz65r8TGFFzvZMdGelWKO7KhBOhIK6wryuWNlaDI","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}]},{"hash":"0xd35d6386ec2f29b90839536f6af9466098d1665e951cdd0a20db6b4629b08369","size":559,"version":0,"nonce":3,"sender":"NUVPACMnKFhpuHjsRjhUvXz1XhqfGZYVtY","sysfee":"10000000","netfee":"4492350","validuntilblock":1200,"attributes":[],"signers":[{"account":"0x95307cb9cc8c4578cef9f6845895eb7aa8be125e","scopes":"CalledByEntry"}],"script":"AwDodkgXAAAADBSqis+FnU/kArNOZz8hVoIXlqSI6wwUXhK+qHrrlViE9vnOeEWMzLl8MJUTwAwIdHJhbnNmZXIMFLyvQdaEx9StbuDZnalwe50fDI5mQWJ9W1I4","witnesses":[{"invocation":"DECKUPl9d502XPI564EC2BroqpN274uV3n1z6kCBCmbS715lzmPbh24LESMsAP2TFohhdhm16aDfNsPi5tkB/FE4DEDzJFts9VYc1lIivGAZZSxACzAV/96Kn2WAaS3bDIlAJHCShsfz+Rn3NuvMyutujYM4vyEipAX9gkjcvFWGKRObDECkI883onhG9aYTxwQWDxsmofuiooRJOic/cJ1H8nqUEvMqATYKgdHaBOJBVYsKq9M9oUv/fj6JFbMDrcasvpiaDECEqkq2b50aEc1NGM9DBAsYLEeZHrM1BwX3a2tBOeeD/KLtmTga1IZogsZgpis2BOToZO6LuN9FJYcn+/iGcC5u","verification":"EwwhAhA6f33QFlWFl/eWDSfFFqQ5T9loueZRVetLAT5AQEBuDCECp7xV/oaE4BGXaNEEujB5W9zIZhnoZK3SYVZyPtGFzWIMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwgwhA9kMB99j5pDOd5EuEKtRrMlEtmhgI3tgjE+PgwnnHuaZFAtBE43vrw=="}]}]}]}`,
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
				require.NotNil(t, param)
				require.Equal(t, request.TxFilterT, param.Type)
				filt, ok := param.Value.(request.TxFilter)
				require.Equal(t, true, ok)
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
				require.NotNil(t, param)
				require.Equal(t, request.TxFilterT, param.Type)
				filt, ok := param.Value.(request.TxFilter)
				require.Equal(t, true, ok)
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
				require.NotNil(t, param)
				require.Equal(t, request.NotificationFilterT, param.Type)
				filt, ok := param.Value.(request.NotificationFilter)
				require.Equal(t, true, ok)
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
				require.NotNil(t, param)
				require.Equal(t, request.NotificationFilterT, param.Type)
				filt, ok := param.Value.(request.NotificationFilter)
				require.Equal(t, true, ok)
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
				require.NotNil(t, param)
				require.Equal(t, request.NotificationFilterT, param.Type)
				filt, ok := param.Value.(request.NotificationFilter)
				require.Equal(t, true, ok)
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
		_, err := NewWS(context.TODO(), strings.Trim(srv.URL, "http://"), Options{})
		require.Error(t, err)
	})
}
