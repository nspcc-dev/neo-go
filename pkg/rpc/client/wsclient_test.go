package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestWSClientClose(t *testing.T) {
	srv := initTestServer(t, "")
	defer srv.Close()
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
				wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
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
				wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
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
			wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
			require.NoError(t, err)
			rc.code(t, wsc)
		})
	}
}

func TestWSClientEvents(t *testing.T) {
	var ok bool
	// Events from RPC server test chain.
	var events = []string{
		`{"jsonrpc":"2.0","method":"transaction_executed","params":[{"txid":"0xe1cd5e57e721d2a2e05fb1f08721b12057b25ab1dd7fd0f33ee1639932fdfad7","executions":[{"trigger":"Application","contract":"0x0000000000000000000000000000000000000000","vmstate":"HALT","gas_consumed":"2.291","stack":[],"notifications":[{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","state":{"type":"Array","value":[{"type":"ByteArray","value":"636f6e74726163742063616c6c"},{"type":"ByteArray","value":"7472616e73666572"},{"type":"Array","value":[{"type":"ByteArray","value":"769162241eedf97c2481652adf1ba0f5bf57431b"},{"type":"ByteArray","value":"316e851039019d39dfc2c37d6c3fee19fd580987"},{"type":"Integer","value":"1000"}]}]}},{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","state":{"type":"Array","value":[{"type":"ByteArray","value":"7472616e73666572"},{"type":"ByteArray","value":"769162241eedf97c2481652adf1ba0f5bf57431b"},{"type":"ByteArray","value":"316e851039019d39dfc2c37d6c3fee19fd580987"},{"type":"Integer","value":"1000"}]}}]}]}]}`,
		`{"jsonrpc":"2.0","method":"notification_from_execution","params":[{"contract":"0x1b4357bff5a01bdf2a6581247cf9ed1e24629176","state":{"type":"Array","value":[{"type":"ByteArray","value":"636f6e74726163742063616c6c"},{"type":"ByteArray","value":"7472616e73666572"},{"type":"Array","value":[{"type":"ByteArray","value":"769162241eedf97c2481652adf1ba0f5bf57431b"},{"type":"ByteArray","value":"316e851039019d39dfc2c37d6c3fee19fd580987"},{"type":"Integer","value":"1000"}]}]}}]}`,
		`{"jsonrpc":"2.0","method":"transaction_added","params":[{"txid":"0xe1e0200a69f3096010c75dd003653e5bd945e414d0b0a3b95e3dfe291534c8e5","size":267,"version":0,"nonce":9,"sender":"ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG","sys_fee":"0","net_fee":"0.0036721","valid_until_block":1200,"attributes":[],"cosigners":[{"account":"0x870958fd19ee3f6c7dc3c2df399d013910856e31","scopes":1}],"script":"007b0c1420728274afafc36f43a071d328cfa3e629d9cbb00c14316e851039019d39dfc2c37d6c3fee19fd58098713c00c087472616e736665720c14769162241eedf97c2481652adf1ba0f5bf57431b41627d5b5238","vin":[],"vout":[],"scripts":[{"invocation":"0c40b61d89290451852143490194e09a547d79b8b6cda05691c085160cc0ddebc6e3a508defae7ac04e015c85af6b6fcb487a954c2a884d4cc147c9959f776844b3c","verification":"0c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906ad4"}]}]}`,
		`{"jsonrpc":"2.0","method":"block_added","params":[{"hash":"0x73f4157bc918f0af115522380b13dadf505773f91e818288edb0de4fcce104b4","version":0,"previousblockhash":"0x3c664cf156f2f418e92fd24621ac3b56da4a4c1e56c443fe360d52324d8f060f","merkleroot":"0xfa9e8e7a506c1a38fa1f090237156debd055c8cf41d743ae4f37fe3c278ff2e0","time":1591360099006,"index":6,"nextconsensus":"AXSvJVzydxXuL9da4GVwK25zdesCrVKkHL","witnesses":[{"invocation":"0c40d7690b2335898b192d408f3565fb8da0f994a305999f1b7b87000934831f76d680442ea2c44200c2b154c12a0ee587046eeec45f749b73ce3bc97ac172487f350c400835fd349d184a691087b24a827d82bad3026ffa0e7f2841e85fb91f5c9549e6a1430883586cbbfba3a450a0198608e19c0b1630677bbd9495e3f48c8c4835c40c400fd2d4dfee36f99de6df35b1c857eb7c8b9b26574a781114081b066b806a1bcae129d05329298b0e6b4728da90928d4a375157dd7472fb37351a20d1a84996790c404b58d5f6374bf6c2ab12f51e6f78a07852e027279879a5fb25b5e6371ad9d89749127e7b8f8eae3df45bfe187ae226aa1aeead061006da7c4e665d97950c9a3e","verification":"130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb"}],"consensus_data":{"primary":0,"nonce":"0000000000000457"},"tx":[{"txid":"0xe1e0200a69f3096010c75dd003653e5bd945e414d0b0a3b95e3dfe291534c8e5","size":267,"version":0,"nonce":9,"sender":"ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG","sys_fee":"0","net_fee":"0.0036721","valid_until_block":1200,"attributes":[],"cosigners":[{"account":"0x870958fd19ee3f6c7dc3c2df399d013910856e31","scopes":1}],"script":"007b0c1420728274afafc36f43a071d328cfa3e629d9cbb00c14316e851039019d39dfc2c37d6c3fee19fd58098713c00c087472616e736665720c14769162241eedf97c2481652adf1ba0f5bf57431b41627d5b5238","vin":[],"vout":[],"scripts":[{"invocation":"0c40b61d89290451852143490194e09a547d79b8b6cda05691c085160cc0ddebc6e3a508defae7ac04e015c85af6b6fcb487a954c2a884d4cc147c9959f776844b3c","verification":"0c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906ad4"}]}]}]}`,
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
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
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
				param, ok := p.Value(1)
				require.Equal(t, true, ok)
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
				param, ok := p.Value(1)
				require.Equal(t, true, ok)
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
				param, ok := p.Value(1)
				require.Equal(t, true, ok)
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
				param, ok := p.Value(1)
				require.Equal(t, true, ok)
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
				param, ok := p.Value(1)
				require.Equal(t, true, ok)
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
				param, ok := p.Value(1)
				require.Equal(t, true, ok)
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
			wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
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
		_, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), Options{})
		require.NoError(t, err)
	})
	t.Run("bad URL", func(t *testing.T) {
		_, err := NewWS(context.TODO(), strings.Trim(srv.URL, "http://"), Options{})
		require.Error(t, err)
	})
}
