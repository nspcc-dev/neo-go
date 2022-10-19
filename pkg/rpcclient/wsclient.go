package rpcclient

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
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/rpcevent"
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
	requests    chan *neorpc.Request
	shutdown    chan struct{}
	closeCalled atomic.Bool

	closeErrLock sync.RWMutex
	closeErr     error

	subscriptionsLock sync.RWMutex
	subscriptions     map[string]notificationReceiver

	respLock     sync.RWMutex
	respChannels map[uint64]chan *neorpc.Response
}

// notificationReceiver is a server events receiver. It stores desired notifications ID
// and filter and a channel used to receive matching notifications.
type notificationReceiver struct {
	typ    neorpc.EventID
	filter interface{}
	ch     chan<- Notification
}

// EventID implements neorpc.Comparator interface and returns notification ID.
func (r notificationReceiver) EventID() neorpc.EventID {
	return r.typ
}

// Filter implements neorpc.Comparator interface and returns notification filter.
func (r notificationReceiver) Filter() interface{} {
	return r.filter
}

// Notification represents a server-generated notification for client subscriptions.
// Value can be one of *block.Block, *state.AppExecResult, *state.ContainedNotificationEvent
// *transaction.Transaction or *subscriptions.NotaryRequestEvent based on Type.
type Notification struct {
	Type  neorpc.EventID
	Value interface{}
}

// EventID implements Container interface and returns notification ID.
func (n Notification) EventID() neorpc.EventID {
	return n.Type
}

// EventPayload implements Container interface and returns notification
// object.
func (n Notification) EventPayload() interface{} {
	return n.Value
}

// requestResponse is a combined type for request and response since we can get
// any of them here.
type requestResponse struct {
	neorpc.Response
	Method    string            `json:"method"`
	RawParams []json.RawMessage `json:"params,omitempty"`
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

// errConnClosedByUser is a WSClient error used iff the user calls (*WSClient).Close method by himself.
var errConnClosedByUser = errors.New("connection closed by user")

// NewWS returns a new WSClient ready to use (with established websocket
// connection). You need to use websocket URL for it like `ws://1.2.3.4/ws`.
// You should call Init method to initialize the network magic the client is
// operating on.
func NewWS(ctx context.Context, endpoint string, opts Options) (*WSClient, error) {
	dialer := websocket.Dialer{HandshakeTimeout: opts.DialTimeout}
	ws, resp, err := dialer.DialContext(ctx, endpoint, nil)
	if resp != nil && resp.Body != nil { // Can be non-nil even with error returned.
		defer resp.Body.Close() // Not exactly required by websocket, but let's do this for bodyclose checker.
	}
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
		respChannels:  make(map[uint64]chan *neorpc.Response),
		requests:      make(chan *neorpc.Request),
		subscriptions: make(map[string]notificationReceiver),
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
		c.setCloseErr(errConnClosedByUser)
		// Closing shutdown channel sends a signal to wsWriter to break out of the
		// loop. In doing so it does ws.Close() closing the network connection
		// which in turn makes wsReader receive an err from ws.ReadJSON() and also
		// break out of the loop closing c.done channel in its shutdown sequence.
		close(c.shutdown)
		// Call to cancel will send signal to all users of Context().
		c.Client.ctxCancel()
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
			break readloop
		}
		err = c.ws.ReadJSON(rr)
		if err != nil {
			// Timeout/connection loss/malformed response.
			connCloseErr = fmt.Errorf("failed to read JSON response (timeout/connection loss/malformed response): %w", err)
			break readloop
		}
		if rr.ID == nil && rr.Method != "" {
			event, err := neorpc.GetEventIDFromString(rr.Method)
			if err != nil {
				// Bad event received.
				connCloseErr = fmt.Errorf("failed to perse event ID from string %s: %w", rr.Method, err)
				break readloop
			}
			if event != neorpc.MissedEventID && len(rr.RawParams) != 1 {
				// Bad event received.
				connCloseErr = fmt.Errorf("bad event received: %s / %d", event, len(rr.RawParams))
				break readloop
			}
			var val interface{}
			switch event {
			case neorpc.BlockEventID:
				sr, err := c.StateRootInHeader()
				if err != nil {
					// Client is not initialized.
					connCloseErr = fmt.Errorf("failed to fetch StateRootInHeader: %w", err)
					break readloop
				}
				val = block.New(sr)
			case neorpc.TransactionEventID:
				val = &transaction.Transaction{}
			case neorpc.NotificationEventID:
				val = new(state.ContainedNotificationEvent)
			case neorpc.ExecutionEventID:
				val = new(state.AppExecResult)
			case neorpc.NotaryRequestEventID:
				val = new(result.NotaryRequestEvent)
			case neorpc.MissedEventID:
				// No value.
			default:
				// Bad event received.
				connCloseErr = fmt.Errorf("unknown event received: %d", event)
				break readloop
			}
			if event != neorpc.MissedEventID {
				err = json.Unmarshal(rr.RawParams[0], val)
				if err != nil {
					// Bad event received.
					connCloseErr = fmt.Errorf("failed to unmarshal event of type %s from JSON: %w", event, err)
					break readloop
				}
			}
			ok := make(map[chan<- Notification]bool)
			c.subscriptionsLock.RLock()
			for _, rcvr := range c.subscriptions {
				ntf := Notification{Type: event, Value: val}
				if (rpcevent.Matches(rcvr, ntf) || event == neorpc.MissedEventID /*missed event must be delivered to each receiver*/) && !ok[rcvr.ch] {
					ok[rcvr.ch] = true // strictly one notification per channel
					rcvr.ch <- ntf     // this will block other receivers
				}
			}
			c.subscriptionsLock.RUnlock()
		} else if rr.ID != nil && (rr.Error != nil || rr.Result != nil) {
			id, err := strconv.ParseUint(string(rr.ID), 10, 64)
			if err != nil {
				connCloseErr = fmt.Errorf("failed to retrieve response ID from string %s: %w", string(rr.ID), err)
				break readloop // Malformed response (invalid response ID).
			}
			ch := c.getResponseChannel(id)
			if ch == nil {
				connCloseErr = fmt.Errorf("unknown response channel for response %d", id)
				break readloop // Unknown response (unexpected response ID).
			}
			ch <- &rr.Response
		} else {
			// Malformed response, neither valid request, nor valid response.
			connCloseErr = fmt.Errorf("malformed response")
			break readloop
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
	c.Client.ctxCancel()
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
				connCloseErr = fmt.Errorf("failed to write JSON request (%s / %d): %w", req.Method, len(req.Params), err)
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

func (c *WSClient) getResponseChannel(id uint64) chan *neorpc.Response {
	c.respLock.RLock()
	defer c.respLock.RUnlock()
	return c.respChannels[id]
}

func (c *WSClient) makeWsRequest(r *neorpc.Request) (*neorpc.Response, error) {
	ch := make(chan *neorpc.Response)
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

func (c *WSClient) performSubscription(params []interface{}, rcvr notificationReceiver) (string, error) {
	var resp string

	if err := c.performRequest("subscribe", params, &resp); err != nil {
		return "", err
	}

	c.subscriptionsLock.Lock()
	defer c.subscriptionsLock.Unlock()

	c.subscriptions[resp] = rcvr
	return resp, nil
}

func (c *WSClient) performUnsubscription(id string) error {
	var resp bool

	c.subscriptionsLock.Lock()
	defer c.subscriptionsLock.Unlock()

	if _, ok := c.subscriptions[id]; !ok {
		return errors.New("no subscription with this ID")
	}
	if err := c.performRequest("unsubscribe", []interface{}{id}, &resp); err != nil {
		return err
	}
	if !resp {
		return errors.New("unsubscribe method returned false result")
	}
	delete(c.subscriptions, id)
	return nil
}

// SubscribeForNewBlocks adds subscription for new block events to this instance
// of the client. It can be filtered by primary consensus node index and/or block
// index allowing to receive blocks since the specified index only, nil value is
// treated as missing filter.
//
// Deprecated: please, use SubscribeForNewBlocksWithChan. This method will be removed in future versions.
func (c *WSClient) SubscribeForNewBlocks(primary *int, sinceIndex, tillIndex *uint32) (string, error) {
	return c.SubscribeForNewBlocksWithChan(primary, sinceIndex, tillIndex, c.Notifications)
}

// SubscribeForNewBlocksWithChan registers provided channel as a receiver for the
// new block events. Events can be filtered by primary consensus node index, nil
// value doesn't add any filters. If the receiver channel is nil, then the default
// Notifications channel will be used. The receiver channel must be properly read
// and drained after usage in order not to block other notification receivers.
func (c *WSClient) SubscribeForNewBlocksWithChan(primary *int, sinceIndex, tillIndex *uint32, rcvrCh chan<- Notification) (string, error) {
	if rcvrCh == nil {
		rcvrCh = c.Notifications
	}
	params := []interface{}{"block_added"}
	var flt *neorpc.BlockFilter
	if primary != nil || sinceIndex != nil || tillIndex != nil {
		flt = &neorpc.BlockFilter{Primary: primary, Since: sinceIndex, Till: tillIndex}
		params = append(params, flt)
	}
	rcvr := notificationReceiver{
		typ:    neorpc.BlockEventID,
		filter: flt,
		ch:     rcvrCh,
	}
	return c.performSubscription(params, rcvr)
}

// SubscribeForNewTransactions adds subscription for new transaction events to
// this instance of the client. It can be filtered by the sender and/or the signer, nil
// value is treated as missing filter.
//
// Deprecated: please, use SubscribeForNewTransactionsWithChan. This method will be removed in future versions.
func (c *WSClient) SubscribeForNewTransactions(sender *util.Uint160, signer *util.Uint160) (string, error) {
	return c.SubscribeForNewTransactionsWithChan(sender, signer, c.Notifications)
}

// SubscribeForNewTransactionsWithChan registers provided channel as a receiver
// for new transaction events. Events can be filtered by the sender and/or the
// signer, nil value is treated as missing filter. If the receiver channel is nil,
// then the default Notifications channel will be used. The receiver channel must be
// properly read and drained after usage in order not to block other notification
// receivers.
func (c *WSClient) SubscribeForNewTransactionsWithChan(sender *util.Uint160, signer *util.Uint160, rcvrCh chan<- Notification) (string, error) {
	if rcvrCh == nil {
		rcvrCh = c.Notifications
	}
	params := []interface{}{"transaction_added"}
	var flt *neorpc.TxFilter
	if sender != nil || signer != nil {
		flt = &neorpc.TxFilter{Sender: sender, Signer: signer}
		params = append(params, *flt)
	}
	rcvr := notificationReceiver{
		typ:    neorpc.TransactionEventID,
		filter: flt,
		ch:     rcvrCh,
	}
	return c.performSubscription(params, rcvr)
}

// SubscribeForExecutionNotifications adds subscription for notifications
// generated during transaction execution to this instance of the client. It can be
// filtered by the contract's hash (that emits notifications), nil value puts no such
// restrictions.
//
// Deprecated: please, use SubscribeForExecutionNotificationsWithChan. This method will be removed in future versions.
func (c *WSClient) SubscribeForExecutionNotifications(contract *util.Uint160, name *string) (string, error) {
	return c.SubscribeForExecutionNotificationsWithChan(contract, name, c.Notifications)
}

// SubscribeForExecutionNotificationsWithChan registers provided channel as a
// receiver for execution events. Events can be filtered by the contract's hash
// (that emits notifications), nil value puts no such restrictions. If the
// receiver channel is nil, then the default Notifications channel will be used.
// The receiver channel must be properly read and drained after usage in order
// not to block other notification receivers.
func (c *WSClient) SubscribeForExecutionNotificationsWithChan(contract *util.Uint160, name *string, rcvrCh chan<- Notification) (string, error) {
	if rcvrCh == nil {
		rcvrCh = c.Notifications
	}
	params := []interface{}{"notification_from_execution"}
	var flt *neorpc.NotificationFilter
	if contract != nil || name != nil {
		flt = &neorpc.NotificationFilter{Contract: contract, Name: name}
		params = append(params, *flt)
	}
	rcvr := notificationReceiver{
		typ:    neorpc.NotificationEventID,
		filter: flt,
		ch:     rcvrCh,
	}
	return c.performSubscription(params, rcvr)
}

// SubscribeForTransactionExecutions adds subscription for application execution
// results generated during transaction execution to this instance of the client. It can
// be filtered by state (HALT/FAULT) to check for successful or failing
// transactions; it can also be filtered by script container hash.
// nil value means no filtering.
//
// Deprecated: please, use SubscribeForTransactionExecutionsWithChan. This method will be removed in future versions.
func (c *WSClient) SubscribeForTransactionExecutions(state *string, container *util.Uint256) (string, error) {
	return c.SubscribeForTransactionExecutionsWithChan(state, container, c.Notifications)
}

// SubscribeForTransactionExecutionsWithChan registers provided channel as a
// receiver for application execution result events generated during transaction
// execution. Events can be filtered by state (HALT/FAULT) to check for successful
// or failing transactions; it can also be filtered by script container hash.
// nil value means no filtering. If the receiver channel is nil, then the default
// Notifications channel will be used. The receiver channel must be properly read
// and drained after usage in order not to block other notification receivers.
func (c *WSClient) SubscribeForTransactionExecutionsWithChan(state *string, container *util.Uint256, rcvrCh chan<- Notification) (string, error) {
	if rcvrCh == nil {
		rcvrCh = c.Notifications
	}
	params := []interface{}{"transaction_executed"}
	var flt *neorpc.ExecutionFilter
	if state != nil || container != nil {
		if state != nil {
			if *state != "HALT" && *state != "FAULT" {
				return "", errors.New("bad state parameter")
			}
		}
		flt = &neorpc.ExecutionFilter{State: state, Container: container}
		params = append(params, *flt)
	}
	rcvr := notificationReceiver{
		typ:    neorpc.ExecutionEventID,
		filter: flt,
		ch:     rcvrCh,
	}
	return c.performSubscription(params, rcvr)
}

// SubscribeForNotaryRequests adds subscription for notary request payloads
// addition or removal events to this instance of client. It can be filtered by
// request sender's hash, or main tx signer's hash, nil value puts no such
// restrictions.
//
// Deprecated: please, use SubscribeForNotaryRequestsWithChan. This method will be removed in future versions.
func (c *WSClient) SubscribeForNotaryRequests(sender *util.Uint160, mainSigner *util.Uint160) (string, error) {
	return c.SubscribeForNotaryRequestsWithChan(sender, mainSigner, c.Notifications)
}

// SubscribeForNotaryRequestsWithChan registers provided channel as a receiver
// for notary request payload addition or removal events. It can be filtered by
// request sender's hash, or main tx signer's hash, nil value puts no such
// restrictions. If the receiver channel is nil, then the default Notifications
// channel will be used. The receiver channel must be properly read and drained
// after usage in order not to block other notification receivers.
func (c *WSClient) SubscribeForNotaryRequestsWithChan(sender *util.Uint160, mainSigner *util.Uint160, rcvrCh chan<- Notification) (string, error) {
	if rcvrCh == nil {
		rcvrCh = c.Notifications
	}
	params := []interface{}{"notary_request_event"}
	var flt *neorpc.TxFilter
	if sender != nil {
		flt = &neorpc.TxFilter{Sender: sender, Signer: mainSigner}
		params = append(params, *flt)
	}
	rcvr := notificationReceiver{
		typ:    neorpc.NotaryRequestEventID,
		filter: flt,
		ch:     rcvrCh,
	}
	return c.performSubscription(params, rcvr)
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
		if err := c.performRequest("unsubscribe", []interface{}{id}, &resp); err != nil {
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

// GetError returns the reason of WS connection closing. It returns nil in case if connection
// was closed by the use via Close() method calling.
func (c *WSClient) GetError() error {
	c.closeErrLock.RLock()
	defer c.closeErrLock.RUnlock()

	if c.closeErr != nil && errors.Is(c.closeErr, errConnClosedByUser) {
		return nil
	}
	return c.closeErr
}

// Context returns WSClient Cancel context that will be terminated on Client shutdown.
func (c *WSClient) Context() context.Context {
	return c.Client.ctx
}
