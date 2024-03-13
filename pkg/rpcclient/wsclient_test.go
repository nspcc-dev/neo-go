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
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv/params"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWSClientClose(t *testing.T) {
	srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": "55aaff00"}`)
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
	require.NoError(t, err)
	wsc.cache.initDone = true
	wsc.getNextRequestID = getTestRequestID
	bCh := make(chan *block.Block)
	_, err = wsc.ReceiveBlocks(nil, bCh)
	require.NoError(t, err)
	wsc.Close()
	// Subscriber channel must be closed by server.
	_, ok := <-bCh
	require.False(t, ok)
}

func TestWSClientSubscription(t *testing.T) {
	bCh := make(chan *block.Block)
	txCh := make(chan *transaction.Transaction)
	aerCh := make(chan *state.AppExecResult)
	ntfCh := make(chan *state.ContainedNotificationEvent)
	ntrCh := make(chan *result.NotaryRequestEvent)
	var cases = map[string]func(*WSClient) (string, error){
		"blocks": func(wsc *WSClient) (string, error) {
			return wsc.ReceiveBlocks(nil, bCh)
		},
		"transactions": func(wsc *WSClient) (string, error) {
			return wsc.ReceiveTransactions(nil, txCh)
		},
		"notifications": func(wsc *WSClient) (string, error) {
			return wsc.ReceiveExecutionNotifications(nil, ntfCh)
		},
		"executions": func(wsc *WSClient) (string, error) {
			return wsc.ReceiveExecutions(nil, aerCh)
		},
		"notary requests": func(wsc *WSClient) (string, error) {
			return wsc.ReceiveNotaryRequests(nil, ntrCh)
		},
	}
	t.Run("good", func(t *testing.T) {
		for name, f := range cases {
			t.Run(name, func(t *testing.T) {
				srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": "55aaff00"}`)
				wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
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
				srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "error":{"code":-32602,"message":"Invalid params"}}`)
				wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
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
			wsc.subscriptions["0"] = &blockReceiver{}
			err := wsc.Unsubscribe("0")
			require.NoError(t, err)
		}},
		"all": {`{"jsonrpc": "2.0", "id": 1, "result": true}`, func(t *testing.T, wsc *WSClient) {
			// We can't really subscribe using this stub server, so set up wsc internals.
			wsc.subscriptions["0"] = &blockReceiver{}
			err := wsc.UnsubscribeAll()
			require.NoError(t, err)
			require.Equal(t, 0, len(wsc.subscriptions))
		}},
		"not subscribed": {`{"jsonrpc": "2.0", "id": 1, "result": true}`, func(t *testing.T, wsc *WSClient) {
			err := wsc.Unsubscribe("0")
			require.Error(t, err)
		}},
		"error returned": {`{"jsonrpc": "2.0", "id": 1, "error":{"code":-32602,"message":"Invalid params"}}`, func(t *testing.T, wsc *WSClient) {
			// We can't really subscribe using this stub server, so set up wsc internals.
			wsc.subscriptions["0"] = &blockReceiver{}
			err := wsc.Unsubscribe("0")
			require.Error(t, err)
		}},
		"false returned": {`{"jsonrpc": "2.0", "id": 1, "result": false}`, func(t *testing.T, wsc *WSClient) {
			// We can't really subscribe using this stub server, so set up wsc internals.
			wsc.subscriptions["0"] = &blockReceiver{}
			err := wsc.Unsubscribe("0")
			require.Error(t, err)
		}},
	}
	for name, rc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := initTestServer(t, rc.response)
			wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
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
		`{"jsonrpc":"2.0","method":"event_missed","params":[]}`, // the last one, will trigger receiver channels closing.
	}
	startSending := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/ws" && req.Method == "GET" {
			var upgrader = websocket.Upgrader{}
			ws, err := upgrader.Upgrade(w, req, nil)
			require.NoError(t, err)
			<-startSending
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
	t.Cleanup(srv.Close)
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
	require.NoError(t, err)
	wsc.getNextRequestID = getTestRequestID
	wsc.cacheLock.Lock()
	wsc.cache.initDone = true // Our server mock is restricted, so perform initialisation manually.
	wsc.cache.network = netmode.UnitTestNet
	wsc.cacheLock.Unlock()

	// Our server mock is restricted, so perform subscriptions manually with default notifications channel.
	bCh1 := make(chan *block.Block)
	bCh2 := make(chan *block.Block)
	aerCh1 := make(chan *state.AppExecResult)
	aerCh2 := make(chan *state.AppExecResult)
	aerCh3 := make(chan *state.AppExecResult)
	ntfCh := make(chan *state.ContainedNotificationEvent)
	halt := "HALT"
	fault := "FAULT"
	wsc.subscriptionsLock.Lock()
	wsc.subscriptions["0"] = &blockReceiver{ch: bCh1}
	wsc.receivers[chan<- *block.Block(bCh1)] = []string{"0"}
	wsc.subscriptions["1"] = &blockReceiver{ch: bCh2} // two different channels subscribed for same notifications
	wsc.receivers[chan<- *block.Block(bCh2)] = []string{"1"}

	wsc.subscriptions["2"] = &executionNotificationReceiver{ch: ntfCh}
	wsc.subscriptions["3"] = &executionNotificationReceiver{ch: ntfCh} // check duplicating subscriptions
	wsc.receivers[chan<- *state.ContainedNotificationEvent(ntfCh)] = []string{"2", "3"}

	wsc.subscriptions["4"] = &executionReceiver{ch: aerCh1}
	wsc.receivers[chan<- *state.AppExecResult(aerCh1)] = []string{"4"}
	wsc.subscriptions["5"] = &executionReceiver{filter: &neorpc.ExecutionFilter{State: &halt}, ch: aerCh2}
	wsc.receivers[chan<- *state.AppExecResult(aerCh2)] = []string{"5"}
	wsc.subscriptions["6"] = &executionReceiver{filter: &neorpc.ExecutionFilter{State: &fault}, ch: aerCh3}
	wsc.receivers[chan<- *state.AppExecResult(aerCh3)] = []string{"6"}
	// MissedEvent must close the channels above.

	wsc.subscriptionsLock.Unlock()
	close(startSending)

	var (
		b1Cnt, b2Cnt                                      int
		aer1Cnt, aer2Cnt, aer3Cnt                         int
		ntfCnt                                            int
		expectedb1Cnt, expectedb2Cnt                      = 1, 1    // single Block event
		expectedaer1Cnt, expectedaer2Cnt, expectedaer3Cnt = 2, 2, 0 // two HALTED AERs
		expectedntfCnt                                    = 1       // single notification event
		aer                                               *state.AppExecResult
	)
	for b1Cnt+b2Cnt+
		aer1Cnt+aer2Cnt+aer3Cnt+
		ntfCnt !=
		expectedb1Cnt+expectedb2Cnt+
			expectedaer1Cnt+expectedaer2Cnt+expectedaer3Cnt+
			expectedntfCnt {
		select {
		case _, ok = <-bCh1:
			if ok {
				b1Cnt++
			}
		case _, ok = <-bCh2:
			if ok {
				b2Cnt++
			}
		case _, ok = <-aerCh1:
			if ok {
				aer1Cnt++
			}
		case aer, ok = <-aerCh2:
			if ok {
				require.Equal(t, vmstate.Halt, aer.VMState)
				aer2Cnt++
			}
		case _, ok = <-aerCh3:
			if ok {
				aer3Cnt++
			}
		case _, ok = <-ntfCh:
			if ok {
				ntfCnt++
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
	assert.Equal(t, expectedb1Cnt, b1Cnt)
	assert.Equal(t, expectedb2Cnt, b2Cnt)
	assert.Equal(t, expectedaer1Cnt, aer1Cnt)
	assert.Equal(t, expectedaer2Cnt, aer2Cnt)
	assert.Equal(t, expectedaer3Cnt, aer3Cnt)
	assert.Equal(t, expectedntfCnt, ntfCnt)

	// Channels must be closed by server
	_, ok = <-bCh1
	require.False(t, ok)
	_, ok = <-bCh2
	require.False(t, ok)
	_, ok = <-aerCh1
	require.False(t, ok)
	_, ok = <-aerCh2
	require.False(t, ok)
	_, ok = <-aerCh3
	require.False(t, ok)
	_, ok = <-ntfCh
	require.False(t, ok)
	_, ok = <-ntfCh
	require.False(t, ok)
}

func TestWSClientNonBlockingEvents(t *testing.T) {
	// Use buffered channel as a receiver to check it will be closed by WSClient
	// after overflow if CloseNotificationChannelIfFull option is enabled.
	const chCap = 3
	bCh := make(chan *block.Block, chCap)

	// Events from RPC server testchain. Require events len to be larger than chCap to reach
	// subscriber's chanel overflow.
	var events = []string{
		fmt.Sprintf(`{"jsonrpc":"2.0","method":"block_added","params":[%s]}`, b1Verbose),
		fmt.Sprintf(`{"jsonrpc":"2.0","method":"block_added","params":[%s]}`, b1Verbose),
		fmt.Sprintf(`{"jsonrpc":"2.0","method":"block_added","params":[%s]}`, b1Verbose),
		fmt.Sprintf(`{"jsonrpc":"2.0","method":"block_added","params":[%s]}`, b1Verbose),
		fmt.Sprintf(`{"jsonrpc":"2.0","method":"block_added","params":[%s]}`, b1Verbose),
	}
	require.True(t, chCap < len(events))

	var blocksSent atomic.Bool
	startSending := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/ws" && req.Method == "GET" {
			var upgrader = websocket.Upgrader{}
			ws, err := upgrader.Upgrade(w, req, nil)
			require.NoError(t, err)
			<-startSending
			for _, event := range events {
				err = ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
				require.NoError(t, err)
				err = ws.WriteMessage(1, []byte(event))
				if err != nil {
					break
				}
			}
			blocksSent.Store(true)
			ws.Close()
			return
		}
	}))
	t.Cleanup(srv.Close)
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{CloseNotificationChannelIfFull: true})
	require.NoError(t, err)
	wsc.getNextRequestID = getTestRequestID
	wsc.cacheLock.Lock()
	wsc.cache.initDone = true // Our server mock is restricted, so perform initialisation manually.
	wsc.cache.network = netmode.UnitTestNet
	wsc.cacheLock.Unlock()

	// Our server mock is restricted, so perform subscriptions manually.
	wsc.subscriptionsLock.Lock()
	wsc.subscriptions["0"] = &blockReceiver{ch: bCh}
	wsc.subscriptions["1"] = &blockReceiver{ch: bCh}
	wsc.receivers[chan<- *block.Block(bCh)] = []string{"0", "1"}
	wsc.subscriptionsLock.Unlock()

	close(startSending)
	// Check that events are sent to WSClient.
	require.Eventually(t, func() bool {
		return blocksSent.Load()
	}, time.Second, 100*time.Millisecond)

	// Check that block receiver channel was removed from the receivers list due to overflow.
	require.Eventually(t, func() bool {
		wsc.subscriptionsLock.RLock()
		defer wsc.subscriptionsLock.RUnlock()
		return len(wsc.receivers) == 0
	}, 2*time.Second, 200*time.Millisecond)

	// Check that subscriptions are still there and waiting for the call to Unsubscribe()
	// to be excluded from the subscriptions map.
	wsc.subscriptionsLock.RLock()
	require.True(t, len(wsc.subscriptions) == 2)
	wsc.subscriptionsLock.RUnlock()

	// Check that receiver was closed after overflow.
	for i := 0; i < chCap; i++ {
		_, ok := <-bCh
		require.True(t, ok)
	}
	select {
	case _, ok := <-bCh:
		require.False(t, ok)
	default:
		t.Fatal("channel wasn't closed by WSClient")
	}
}

func TestWSExecutionVMStateCheck(t *testing.T) {
	// Will answer successfully if request slips through.
	srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": "55aaff00"}`)
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
	require.NoError(t, err)
	wsc.getNextRequestID = getTestRequestID
	require.NoError(t, wsc.Init())
	filter := "NONE"
	_, err = wsc.ReceiveExecutions(&neorpc.ExecutionFilter{State: &filter}, make(chan *state.AppExecResult))
	require.ErrorIs(t, err, neorpc.ErrInvalidSubscriptionFilter)
	wsc.Close()
}

func TestWSExecutionNotificationNameCheck(t *testing.T) {
	// Will answer successfully if request slips through.
	srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": "55aaff00"}`)
	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
	require.NoError(t, err)
	wsc.getNextRequestID = getTestRequestID
	require.NoError(t, wsc.Init())
	filter := "notification_from_execution_with_long_name"
	_, err = wsc.ReceiveExecutionNotifications(&neorpc.NotificationFilter{Name: &filter}, make(chan *state.ContainedNotificationEvent))
	require.ErrorIs(t, err, neorpc.ErrInvalidSubscriptionFilter)
	wsc.Close()
}

func TestWSFilteredSubscriptions(t *testing.T) {
	var cases = []struct {
		name       string
		clientCode func(*testing.T, *WSClient)
		serverCode func(*testing.T, *params.Params)
	}{
		{"block header primary",
			func(t *testing.T, wsc *WSClient) {
				primary := byte(3)
				_, err := wsc.ReceiveHeadersOfAddedBlocks(&neorpc.BlockFilter{Primary: &primary}, make(chan *block.Header))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, byte(3), *filt.Primary)
				require.Equal(t, (*uint32)(nil), filt.Since)
				require.Equal(t, (*uint32)(nil), filt.Till)
			},
		},
		{"header since",
			func(t *testing.T, wsc *WSClient) {
				var since uint32 = 3
				_, err := wsc.ReceiveHeadersOfAddedBlocks(&neorpc.BlockFilter{Since: &since}, make(chan *block.Header))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, (*byte)(nil), filt.Primary)
				require.Equal(t, uint32(3), *filt.Since)
				require.Equal(t, (*uint32)(nil), filt.Till)
			},
		},
		{"header till",
			func(t *testing.T, wsc *WSClient) {
				var till uint32 = 3
				_, err := wsc.ReceiveHeadersOfAddedBlocks(&neorpc.BlockFilter{Till: &till}, make(chan *block.Header))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, (*byte)(nil), filt.Primary)
				require.Equal(t, (*uint32)(nil), filt.Since)
				require.Equal(t, (uint32)(3), *filt.Till)
			},
		},
		{"header primary, since and till",
			func(t *testing.T, wsc *WSClient) {
				var (
					since   uint32 = 3
					primary        = byte(2)
					till    uint32 = 5
				)
				_, err := wsc.ReceiveHeadersOfAddedBlocks(&neorpc.BlockFilter{
					Primary: &primary,
					Since:   &since,
					Till:    &till,
				}, make(chan *block.Header))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, byte(2), *filt.Primary)
				require.Equal(t, uint32(3), *filt.Since)
				require.Equal(t, uint32(5), *filt.Till)
			},
		},
		{"blocks primary",
			func(t *testing.T, wsc *WSClient) {
				primary := byte(3)
				_, err := wsc.ReceiveBlocks(&neorpc.BlockFilter{Primary: &primary}, make(chan *block.Block))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, byte(3), *filt.Primary)
				require.Equal(t, (*uint32)(nil), filt.Since)
				require.Equal(t, (*uint32)(nil), filt.Till)
			},
		},
		{"blocks since",
			func(t *testing.T, wsc *WSClient) {
				var since uint32 = 3
				_, err := wsc.ReceiveBlocks(&neorpc.BlockFilter{Since: &since}, make(chan *block.Block))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, (*byte)(nil), filt.Primary)
				require.Equal(t, uint32(3), *filt.Since)
				require.Equal(t, (*uint32)(nil), filt.Till)
			},
		},
		{"blocks till",
			func(t *testing.T, wsc *WSClient) {
				var till uint32 = 3
				_, err := wsc.ReceiveBlocks(&neorpc.BlockFilter{Till: &till}, make(chan *block.Block))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, (*byte)(nil), filt.Primary)
				require.Equal(t, (*uint32)(nil), filt.Since)
				require.Equal(t, (uint32)(3), *filt.Till)
			},
		},
		{"blocks primary, since and till",
			func(t *testing.T, wsc *WSClient) {
				var (
					since   uint32 = 3
					primary        = byte(2)
					till    uint32 = 5
				)
				_, err := wsc.ReceiveBlocks(&neorpc.BlockFilter{
					Primary: &primary,
					Since:   &since,
					Till:    &till,
				}, make(chan *block.Block))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.BlockFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, byte(2), *filt.Primary)
				require.Equal(t, uint32(3), *filt.Since)
				require.Equal(t, uint32(5), *filt.Till)
			},
		},
		{"transactions sender",
			func(t *testing.T, wsc *WSClient) {
				sender := util.Uint160{1, 2, 3, 4, 5}
				_, err := wsc.ReceiveTransactions(&neorpc.TxFilter{Sender: &sender}, make(chan *transaction.Transaction))
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
				_, err := wsc.ReceiveTransactions(&neorpc.TxFilter{Signer: &signer}, make(chan *transaction.Transaction))
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
				_, err := wsc.ReceiveTransactions(&neorpc.TxFilter{Sender: &sender, Signer: &signer}, make(chan *transaction.Transaction))
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
				_, err := wsc.ReceiveExecutionNotifications(&neorpc.NotificationFilter{Contract: &contract}, make(chan *state.ContainedNotificationEvent))
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
				_, err := wsc.ReceiveExecutionNotifications(&neorpc.NotificationFilter{Name: &name}, make(chan *state.ContainedNotificationEvent))
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
				_, err := wsc.ReceiveExecutionNotifications(&neorpc.NotificationFilter{Contract: &contract, Name: &name}, make(chan *state.ContainedNotificationEvent))
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
				vmstate := "FAULT"
				_, err := wsc.ReceiveExecutions(&neorpc.ExecutionFilter{State: &vmstate}, make(chan *state.AppExecResult))
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
				_, err := wsc.ReceiveExecutions(&neorpc.ExecutionFilter{Container: &container}, make(chan *state.AppExecResult))
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
				vmstate := "FAULT"
				container := util.Uint256{1, 2, 3}
				_, err := wsc.ReceiveExecutions(&neorpc.ExecutionFilter{State: &vmstate, Container: &container}, make(chan *state.AppExecResult))
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
		{
			"notary request sender",
			func(t *testing.T, wsc *WSClient) {
				sender := util.Uint160{1, 2, 3, 4, 5}
				_, err := wsc.ReceiveNotaryRequests(&neorpc.NotaryRequestFilter{Sender: &sender}, make(chan *result.NotaryRequestEvent))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.NotaryRequestFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Sender)
				require.Nil(t, filt.Signer)
				require.Nil(t, filt.Type)
			},
		},
		{
			"notary request signer",
			func(t *testing.T, wsc *WSClient) {
				signer := util.Uint160{0, 42}
				_, err := wsc.ReceiveNotaryRequests(&neorpc.NotaryRequestFilter{Signer: &signer}, make(chan *result.NotaryRequestEvent))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.NotaryRequestFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Nil(t, filt.Sender)
				require.Equal(t, util.Uint160{0, 42}, *filt.Signer)
				require.Nil(t, filt.Type)
			},
		},
		{
			"notary request type",
			func(t *testing.T, wsc *WSClient) {
				mempoolType := mempoolevent.TransactionAdded
				_, err := wsc.ReceiveNotaryRequests(&neorpc.NotaryRequestFilter{Type: &mempoolType}, make(chan *result.NotaryRequestEvent))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.NotaryRequestFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, mempoolevent.TransactionAdded, *filt.Type)
				require.Nil(t, filt.Sender)
				require.Nil(t, filt.Signer)
			},
		},
		{"notary request sender, signer and type",
			func(t *testing.T, wsc *WSClient) {
				sender := util.Uint160{1, 2, 3, 4, 5}
				signer := util.Uint160{0, 42}
				mempoolType := mempoolevent.TransactionAdded
				_, err := wsc.ReceiveNotaryRequests(&neorpc.NotaryRequestFilter{Type: &mempoolType, Signer: &signer, Sender: &sender}, make(chan *result.NotaryRequestEvent))
				require.NoError(t, err)
			},
			func(t *testing.T, p *params.Params) {
				param := p.Value(1)
				filt := new(neorpc.NotaryRequestFilter)
				require.NoError(t, json.Unmarshal(param.RawMessage, filt))
				require.Equal(t, util.Uint160{1, 2, 3, 4, 5}, *filt.Sender)
				require.Equal(t, util.Uint160{0, 42}, *filt.Signer)
				require.Equal(t, mempoolevent.TransactionAdded, *filt.Type)
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
			t.Cleanup(srv.Close)
			wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
			require.NoError(t, err)
			wsc.getNextRequestID = getTestRequestID
			wsc.cache.network = netmode.UnitTestNet
			wsc.cache.initDone = true
			c.clientCode(t, wsc)
			wsc.Close()
		})
	}
}

func TestNewWS(t *testing.T) {
	srv := initTestServer(t, "")

	t.Run("good", func(t *testing.T) {
		c, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
		require.NoError(t, err)
		c.getNextRequestID = getTestRequestID
		c.cache.network = netmode.UnitTestNet
		require.NoError(t, c.Init())
	})
	t.Run("bad URL", func(t *testing.T) {
		_, err := NewWS(context.TODO(), strings.TrimPrefix(srv.URL, "http://"), WSOptions{})
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

	wsc, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
	require.NoError(t, err)
	batchCount := 100
	completed := &atomic.Int32{}
	for i := 0; i < batchCount; i++ {
		go func() {
			_, err := wsc.GetBlockCount()
			require.NoError(t, err)
			completed.Add(1)
		}()
		go func() {
			_, err := wsc.GetBlockHash(123)
			require.NoError(t, err)
			completed.Add(1)
		}()

		go func() {
			_, err := wsc.GetVersion()
			require.NoError(t, err)
			completed.Add(1)
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

	c, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		c.Close()
		c.Close()
	})
}

func TestWS_RequestAfterClose(t *testing.T) {
	srv := initTestServer(t, "")

	c, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
	require.NoError(t, err)

	c.Close()

	require.NotPanics(t, func() {
		_, err = c.GetBlockCount()
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrWSConnLost)
}

func TestWSClient_ConnClosedError(t *testing.T) {
	t.Run("standard closing", func(t *testing.T) {
		srv := initTestServer(t, `{"jsonrpc": "2.0", "id": 1, "result": 123}`)
		c, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
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
		c, err := NewWS(context.TODO(), httpURLtoWS(srv.URL), WSOptions{})
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
