package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result/subscriptions"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/atomic"
)

// WSClient is a websocket-enabled RPC client that can be used with appropriate
// servers. It's supposed to be faster than Client because it has persistent
// connection to the server and at the same time it exposes some functionality
// that is only provided via websockets (like event subscription mechanism).
// WSClient is thread-safe and can be used from multiple goroutines to perform
// RPC requests.
type WSClient struct {
	Client
	// Notifications is a channel that is used to send events received from
	// the server. Client's code is supposed to be reading from this channel if
	// it wants to use subscription mechanism. Failing to do so will cause
	// WSClient to block even regular requests. This channel is not buffered.
	// In case of protocol error or upon connection closure, this channel will
	// be closed, so make sure to handle this.
	Notifications chan Notification

	ws          *websocket.Conn
	done        chan struct{}
	requests    chan *request.Raw
	shutdown    chan struct{}
	closeCalled atomic.Bool

	closeErrLock sync.RWMutex
	closeErr     error

	subscriptionsLock sync.RWMutex
	subscriptions     map[string]bool

	respLock     sync.RWMutex
	respChannels map[uint64]chan *response.Raw
}

// Notification represents a server-generated notification for client subscriptions.
// Value can be one of block.Block, state.AppExecResult, subscriptions.NotificationEvent
// transaction.Transaction or subscriptions.NotaryRequestEvent based on Type.
type Notification struct {
	Type  response.EventID
	Value interface{}
}

// requestResponse is a combined type for request and response since we can get
// any of them here.
type requestResponse struct {
	request.In
	Error  *response.Error `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

const (
	// Message limit for receiving side.
	wsReadLimit = 10 * 1024 * 1024

	// Disconnection timeout.
	wsPongLimit = 60 * time.Second

	// Ping period for connection liveness check.
	wsPingPeriod = wsPongLimit / 2

	// Write deadline.
	wsWriteLimit = wsPingPeriod / 2
)

// NewWS returns a new WSClient ready to use (with established websocket
// connection). You need to use websocket URL for it like `ws://1.2.3.4/ws`.
// You should call Init method to initialize the network magic the client is
// operating on.
func NewWS(ctx context.Context, endpoint string, opts Options) (*WSClient, error) {
	dialer := websocket.Dialer{HandshakeTimeout: opts.DialTimeout}
	ws, _, err := dialer.Dial(endpoint, nil)
	if err != nil {
		return nil, err
	}
	wsc := &WSClient{
		Client:        Client{},
		Notifications: make(chan Notification),

		ws:            ws,
		shutdown:      make(chan struct{}),
		done:          make(chan struct{}),
		closeCalled:   *atomic.NewBool(false),
		respChannels:  make(map[uint64]chan *response.Raw),
		requests:      make(chan *request.Raw),
		subscriptions: make(map[string]bool),
	}

	err = initClient(ctx, &wsc.Client, endpoint, opts)
	if err != nil {
		return nil, err
	}
	wsc.Client.cli = nil

	go wsc.wsReader()
	go wsc.wsWriter()
	wsc.requestF = wsc.makeWsRequest
	return wsc, nil
}

// Close closes connection to the remote side rendering this client instance
// unusable.
func (c *WSClient) Close() {
	if c.closeCalled.CAS(false, true) {
		// Closing shutdown channel sends a signal to wsWriter to break out of the
		// loop. In doing so it does ws.Close() closing the network connection
		// which in turn makes wsReader receive an err from ws.ReadJSON() and also
		// break out of the loop closing c.done channel in its shutdown sequence.
		close(c.shutdown)
	}
	<-c.done
}

func (c *WSClient) wsReader() {
	c.ws.SetReadLimit(wsReadLimit)
	c.ws.SetPongHandler(func(string) error {
		err := c.ws.SetReadDeadline(time.Now().Add(wsPongLimit))
		if err != nil {
			c.setCloseErr(fmt.Errorf("failed to set pong read deadline: %w", err))
		}
		return err
	})
	var connCloseErr error
readloop:
	for {
		rr := new(requestResponse)
		err := c.ws.SetReadDeadline(time.Now().Add(wsPongLimit))
		if err != nil {
			connCloseErr = fmt.Errorf("failed to set response read deadline: %w", err)
			break
		}
		err = c.ws.ReadJSON(rr)
		if err != nil {
			// Timeout/connection loss/malformed response.
			connCloseErr = fmt.Errorf("failed to read JSON response (timeout/connection loss/malformed response): %w", err)
			break
		}
		if rr.RawID == nil && rr.Method != "" {
			event, err := response.GetEventIDFromString(rr.Method)
			if err != nil {
				// Bad event received.
				connCloseErr = fmt.Errorf("failed to perse event ID from string %s: %w", rr.Method, err)
				break
			}
			if event != response.MissedEventID && len(rr.RawParams) != 1 {
				// Bad event received.
				connCloseErr = fmt.Errorf("bad event received: %s / %d", event, len(rr.RawParams))
				break
			}
			var val interface{}
			switch event {
			case response.BlockEventID:
				sr, err := c.StateRootInHeader()
				if err != nil {
					// Client is not initialized.
					connCloseErr = fmt.Errorf("failed to fetch StateRootInHeader: %w", err)
					break readloop
				}
				val = block.New(sr)
			case response.TransactionEventID:
				val = &transaction.Transaction{}
			case response.NotificationEventID:
				val = new(subscriptions.NotificationEvent)
			case response.ExecutionEventID:
				val = new(state.AppExecResult)
			case response.NotaryRequestEventID:
				val = new(subscriptions.NotaryRequestEvent)
			case response.MissedEventID:
				// No value.
			default:
				// Bad event received.
				connCloseErr = fmt.Errorf("unknown event received: %d", event)
				break readloop
			}
			if event != response.MissedEventID {
				err = json.Unmarshal(rr.RawParams[0].RawMessage, val)
				if err != nil {
					// Bad event received.
					connCloseErr = fmt.Errorf("failed to unmarshal event of type %s from JSON: %w", event, err)
					break
				}
			}
			c.Notifications <- Notification{event, val}
		} else if rr.RawID != nil && (rr.Error != nil || rr.Result != nil) {
			resp := new(response.Raw)
			resp.ID = rr.RawID
			resp.JSONRPC = rr.JSONRPC
			resp.Error = rr.Error
			resp.Result = rr.Result
			id, err := strconv.Atoi(string(resp.ID))
			if err != nil {
				connCloseErr = fmt.Errorf("failed to retrieve response ID from string %s: %w", string(resp.ID), err)
				break // Malformed response (invalid response ID).
			}
			ch := c.getResponseChannel(uint64(id))
			if ch == nil {
				connCloseErr = fmt.Errorf("unknown response channel for response %d", id)
				break // Unknown response (unexpected response ID).
			}
			ch <- resp
		} else {
			// Malformed response, neither valid request, nor valid response.
			connCloseErr = fmt.Errorf("malformed response")
			break
		}
	}
	if connCloseErr != nil {
		c.setCloseErr(connCloseErr)
	}
	close(c.done)
	c.respLock.Lock()
	for _, ch := range c.respChannels {
		close(ch)
	}
	c.respChannels = nil
	c.respLock.Unlock()
	close(c.Notifications)
}

func (c *WSClient) wsWriter() {
	pingTicker := time.NewTicker(wsPingPeriod)
	defer c.ws.Close()
	defer pingTicker.Stop()
	var connCloseErr error
writeloop:
	for {
		select {
		case <-c.shutdown:
			return
		case <-c.done:
			return
		case req, ok := <-c.requests:
			if !ok {
				return
			}
			if err := c.ws.SetWriteDeadline(time.Now().Add(c.opts.RequestTimeout)); err != nil {
				connCloseErr = fmt.Errorf("failed to set request write deadline: %w", err)
				break writeloop
			}
			if err := c.ws.WriteJSON(req); err != nil {
				connCloseErr = fmt.Errorf("failed to write JSON request (%s / %d): %w", req.Method, len(req.RawParams), err)
				break writeloop
			}
		case <-pingTicker.C:
			if err := c.ws.SetWriteDeadline(time.Now().Add(wsWriteLimit)); err != nil {
				connCloseErr = fmt.Errorf("failed to set ping write deadline: %w", err)
				break writeloop
			}
			if err := c.ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				connCloseErr = fmt.Errorf("failed to write ping message: %w", err)
				break writeloop
			}
		}
	}
	if connCloseErr != nil {
		c.setCloseErr(connCloseErr)
	}
}

func (c *WSClient) unregisterRespChannel(id uint64) {
	c.respLock.Lock()
	defer c.respLock.Unlock()
	if ch, ok := c.respChannels[id]; ok {
		delete(c.respChannels, id)
		close(ch)
	}
}

func (c *WSClient) getResponseChannel(id uint64) chan *response.Raw {
	c.respLock.RLock()
	defer c.respLock.RUnlock()
	return c.respChannels[id]
}

func (c *WSClient) makeWsRequest(r *request.Raw) (*response.Raw, error) {
	ch := make(chan *response.Raw)
	c.respLock.Lock()
	select {
	case <-c.done:
		c.respLock.Unlock()
		return nil, errors.New("connection lost before registering response channel")
	default:
		c.respChannels[r.ID] = ch
		c.respLock.Unlock()
	}
	select {
	case <-c.done:
		return nil, errors.New("connection lost before sending the request")
	case c.requests <- r:
	}
	select {
	case <-c.done:
		return nil, errors.New("connection lost while waiting for the response")
	case resp := <-ch:
		c.unregisterRespChannel(r.ID)
		return resp, nil
	}
}

func (c *WSClient) performSubscription(params request.RawParams) (string, error) {
	var resp string

	if err := c.performRequest("subscribe", params, &resp); err != nil {
		return "", err
	}

	c.subscriptionsLock.Lock()
	defer c.subscriptionsLock.Unlock()

	c.subscriptions[resp] = true
	return resp, nil
}

func (c *WSClient) performUnsubscription(id string) error {
	var resp bool

	c.subscriptionsLock.Lock()
	defer c.subscriptionsLock.Unlock()

	if !c.subscriptions[id] {
		return errors.New("no subscription with this ID")
	}
	if err := c.performRequest("unsubscribe", request.NewRawParams(id), &resp); err != nil {
		return err
	}
	if !resp {
		return errors.New("unsubscribe method returned false result")
	}
	delete(c.subscriptions, id)
	return nil
}

// SubscribeForNewBlocks adds subscription for new block events to this instance
// of the client. It can be filtered by primary consensus node index, nil value doesn't
// add any filters.
func (c *WSClient) SubscribeForNewBlocks(primary *int) (string, error) {
	params := request.NewRawParams("block_added")
	if primary != nil {
		params.Values = append(params.Values, request.BlockFilter{Primary: *primary})
	}
	return c.performSubscription(params)
}

// SubscribeForNewTransactions adds subscription for new transaction events to
// this instance of the client. It can be filtered by the sender and/or the signer, nil
// value is treated as missing filter.
func (c *WSClient) SubscribeForNewTransactions(sender *util.Uint160, signer *util.Uint160) (string, error) {
	params := request.NewRawParams("transaction_added")
	if sender != nil || signer != nil {
		params.Values = append(params.Values, request.TxFilter{Sender: sender, Signer: signer})
	}
	return c.performSubscription(params)
}

// SubscribeForExecutionNotifications adds subscription for notifications
// generated during transaction execution to this instance of the client. It can be
// filtered by the contract's hash (that emits notifications), nil value puts no such
// restrictions.
func (c *WSClient) SubscribeForExecutionNotifications(contract *util.Uint160, name *string) (string, error) {
	params := request.NewRawParams("notification_from_execution")
	if contract != nil || name != nil {
		params.Values = append(params.Values, request.NotificationFilter{Contract: contract, Name: name})
	}
	return c.performSubscription(params)
}

// SubscribeForTransactionExecutions adds subscription for application execution
// results generated during transaction execution to this instance of the client. It can
// be filtered by state (HALT/FAULT) to check for successful or failing
// transactions, nil value means no filtering.
func (c *WSClient) SubscribeForTransactionExecutions(state *string) (string, error) {
	params := request.NewRawParams("transaction_executed")
	if state != nil {
		if *state != "HALT" && *state != "FAULT" {
			return "", errors.New("bad state parameter")
		}
		params.Values = append(params.Values, request.ExecutionFilter{State: *state})
	}
	return c.performSubscription(params)
}

// SubscribeForNotaryRequests adds subscription for notary request payloads
// addition or removal events to this instance of client. It can be filtered by
// request sender's hash, or main tx signer's hash, nil value puts no such
// restrictions.
func (c *WSClient) SubscribeForNotaryRequests(sender *util.Uint160, mainSigner *util.Uint160) (string, error) {
	params := request.NewRawParams("notary_request_event")
	if sender != nil {
		params.Values = append(params.Values, request.TxFilter{Sender: sender, Signer: mainSigner})
	}
	return c.performSubscription(params)
}

// Unsubscribe removes subscription for the given event stream.
func (c *WSClient) Unsubscribe(id string) error {
	return c.performUnsubscription(id)
}

// UnsubscribeAll removes all active subscriptions of the current client.
func (c *WSClient) UnsubscribeAll() error {
	c.subscriptionsLock.Lock()
	defer c.subscriptionsLock.Unlock()

	for id := range c.subscriptions {
		var resp bool
		if err := c.performRequest("unsubscribe", request.NewRawParams(id), &resp); err != nil {
			return err
		}
		if !resp {
			return errors.New("unsubscribe method returned false result")
		}
		delete(c.subscriptions, id)
	}
	return nil
}

// setCloseErr is a thread-safe method setting closeErr in case if it's not yet set.
func (c *WSClient) setCloseErr(err error) {
	c.closeErrLock.Lock()
	defer c.closeErrLock.Unlock()

	if c.closeErr == nil {
		c.closeErr = err
	}
}

// GetError returns the reason of WS connection closing.
func (c *WSClient) GetError() error {
	c.closeErrLock.RLock()
	defer c.closeErrLock.RUnlock()

	return c.closeErr
}
