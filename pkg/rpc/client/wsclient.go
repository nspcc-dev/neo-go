package client

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
)

// WSClient is a websocket-enabled RPC client that can be used with appropriate
// servers. It's supposed to be faster than Client because it has persistent
// connection to the server and at the same time is exposes some functionality
// that is only provided via websockets (like event subscription mechanism).
type WSClient struct {
	Client
	// Notifications is a channel that is used to send events received from
	// server. Client's code is supposed to be reading from this channel if
	// it wants to use subscription mechanism, failing to do so will cause
	// WSClient to block even regular requests. This channel is not buffered.
	// In case of protocol error or upon connection closure this channel will
	// be closed, so make sure to handle this.
	Notifications chan Notification

	ws            *websocket.Conn
	done          chan struct{}
	responses     chan *response.Raw
	requests      chan *request.Raw
	shutdown      chan struct{}
	subscriptions map[string]bool
}

// Notification represents server-generated notification for client subscriptions.
// Value can be one of block.Block, result.ApplicationLog, result.NotificationEvent
// or transaction.Transaction based on Type.
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
func NewWS(ctx context.Context, endpoint string, opts Options) (*WSClient, error) {
	cl, err := New(ctx, endpoint, opts)
	cl.cli = nil

	dialer := websocket.Dialer{HandshakeTimeout: opts.DialTimeout}
	ws, _, err := dialer.Dial(endpoint, nil)
	if err != nil {
		return nil, err
	}
	wsc := &WSClient{
		Client:        *cl,
		Notifications: make(chan Notification),

		ws:            ws,
		shutdown:      make(chan struct{}),
		done:          make(chan struct{}),
		responses:     make(chan *response.Raw),
		requests:      make(chan *request.Raw),
		subscriptions: make(map[string]bool),
	}
	go wsc.wsReader()
	go wsc.wsWriter()
	wsc.requestF = wsc.makeWsRequest
	return wsc, nil
}

// Close closes connection to the remote side rendering this client instance
// unusable.
func (c *WSClient) Close() {
	// Closing shutdown channel send signal to wsWriter to break out of the
	// loop. In doing so it does ws.Close() closing the network connection
	// which in turn makes wsReader receieve err from ws,ReadJSON() and also
	// break out of the loop closing c.done channel in its shutdown sequence.
	close(c.shutdown)
	<-c.done
}

func (c *WSClient) wsReader() {
	c.ws.SetReadLimit(wsReadLimit)
	c.ws.SetPongHandler(func(string) error { c.ws.SetReadDeadline(time.Now().Add(wsPongLimit)); return nil })
readloop:
	for {
		rr := new(requestResponse)
		c.ws.SetReadDeadline(time.Now().Add(wsPongLimit))
		err := c.ws.ReadJSON(rr)
		if err != nil {
			// Timeout/connection loss/malformed response.
			break
		}
		if rr.RawID == nil && rr.Method != "" {
			event, err := response.GetEventIDFromString(rr.Method)
			if err != nil {
				// Bad event received.
				break
			}
			var slice []json.RawMessage
			err = json.Unmarshal(rr.RawParams, &slice)
			if err != nil || len(slice) != 1 {
				// Bad event received.
				break
			}
			var val interface{}
			switch event {
			case response.BlockEventID:
				val = new(block.Block)
			case response.TransactionEventID:
				val = new(transaction.Transaction)
			case response.NotificationEventID:
				val = new(result.NotificationEvent)
			case response.ExecutionEventID:
				val = new(result.ApplicationLog)
			default:
				// Bad event received.
				break readloop
			}
			err = json.Unmarshal(slice[0], val)
			if err != nil || len(slice) != 1 {
				// Bad event received.
				break
			}
			c.Notifications <- Notification{event, val}
		} else if rr.RawID != nil && (rr.Error != nil || rr.Result != nil) {
			resp := new(response.Raw)
			resp.ID = rr.RawID
			resp.JSONRPC = rr.JSONRPC
			resp.Error = rr.Error
			resp.Result = rr.Result
			c.responses <- resp
		} else {
			// Malformed response, neither valid request, nor valid response.
			break
		}
	}
	close(c.done)
	close(c.responses)
	close(c.Notifications)
}

func (c *WSClient) wsWriter() {
	pingTicker := time.NewTicker(wsPingPeriod)
	defer c.ws.Close()
	defer pingTicker.Stop()
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
			c.ws.SetWriteDeadline(time.Now().Add(c.opts.RequestTimeout))
			if err := c.ws.WriteJSON(req); err != nil {
				return
			}
		case <-pingTicker.C:
			c.ws.SetWriteDeadline(time.Now().Add(wsWriteLimit))
			if err := c.ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}

}

func (c *WSClient) makeWsRequest(r *request.Raw) (*response.Raw, error) {
	select {
	case <-c.done:
		return nil, errors.New("connection lost")
	case c.requests <- r:
	}
	select {
	case <-c.done:
		return nil, errors.New("connection lost")
	case resp := <-c.responses:
		return resp, nil
	}
}

func (c *WSClient) performSubscription(params request.RawParams) (string, error) {
	var resp string

	if err := c.performRequest("subscribe", params, &resp); err != nil {
		return "", err
	}
	c.subscriptions[resp] = true
	return resp, nil
}

func (c *WSClient) performUnsubscription(id string) error {
	var resp bool

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
// of client.
func (c *WSClient) SubscribeForNewBlocks() (string, error) {
	params := request.NewRawParams("block_added")
	return c.performSubscription(params)
}

// SubscribeForNewTransactions adds subscription for new transaction events to
// this instance of client.
func (c *WSClient) SubscribeForNewTransactions() (string, error) {
	params := request.NewRawParams("transaction_added")
	return c.performSubscription(params)
}

// SubscribeForExecutionNotifications adds subscription for notifications
// generated during transaction execution to this instance of client.
func (c *WSClient) SubscribeForExecutionNotifications() (string, error) {
	params := request.NewRawParams("notification_from_execution")
	return c.performSubscription(params)
}

// SubscribeForTransactionExecutions adds subscription for application execution
// results generated during transaction execution to this instance of client.
func (c *WSClient) SubscribeForTransactionExecutions() (string, error) {
	params := request.NewRawParams("transaction_executed")
	return c.performSubscription(params)
}

// Unsubscribe removes subscription for given event stream.
func (c *WSClient) Unsubscribe(id string) error {
	return c.performUnsubscription(id)
}

// UnsubscribeAll removes all active subscriptions of current client.
func (c *WSClient) UnsubscribeAll() error {
	for id := range c.subscriptions {
		err := c.performUnsubscription(id)
		if err != nil {
			return err
		}
	}
	return nil
}
