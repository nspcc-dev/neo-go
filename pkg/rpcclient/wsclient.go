package rpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/rpcevent"
)

// WSClient is a websocket-enabled RPC client that can be used with appropriate
// servers. It's supposed to be faster than Client because it has persistent
// connection to the server and at the same time it exposes some functionality
// that is only provided via websockets (like event subscription mechanism).
// WSClient is thread-safe and can be used from multiple goroutines to perform
// RPC requests.
//
// It exposes a set of Receive* methods with the same behaviour pattern that
// is caused by the fact that the client itself receives every message from the
// server via a single channel. This includes any subscriptions and any replies
// to ordinary requests at the same. The client then routes these messages to
// channels provided on subscription (passed to Receive*) or to the respective
// receivers (API callers) if it's an ordinary JSON-RPC reply. While synchronous
// API users are blocked during their calls and wake up on reply, subscription
// channels must be read from to avoid blocking the client. Failure to do so
// will make WSClient wait for the channel reader to get the event and while
// it waits every other messages (subscription-related or request replies)
// will be blocked. This also means that subscription channel must be properly
// drained after unsubscription. If CloseNotificationChannelIfFull option is on
// then the receiver channel will be closed immediately in case if a subsequent
// notification can't be sent to it, which means WSClient's operations are
// unblocking in this mode. No unsubscription is performed in this case, so it's
// still the user responsibility to unsubscribe.
//
// All Receive* methods provide notifications ordering and persistence guarantees.
// See https://github.com/nspcc-dev/neo-go/blob/master/docs/notifications.md#ordering-and-persistence-guarantees
// for more details on this topic.
//
// Any received subscription items (blocks/transactions/nofitications) are passed
// via pointers for efficiency, but the actual structures MUST NOT be changed, as
// it may affect the functionality of other notification receivers. If multiple
// subscriptions share the same receiver channel, then matching notification is
// only sent once per channel. The receiver channel will be closed by the WSClient
// immediately after MissedEvent is received from the server; no unsubscription
// is performed in this case, so it's the user responsibility to unsubscribe. It
// will also be closed on disconnection from server or on situation when it's
// impossible to send a subsequent notification to the subscriber's channel and
// CloseNotificationChannelIfFull option is on.
type WSClient struct {
	Client

	ws          *websocket.Conn
	wsOpts      WSOptions
	readerDone  chan struct{}
	writerDone  chan struct{}
	requests    chan *neorpc.Request
	shutdown    chan struct{}
	closeCalled atomic.Bool

	closeErrLock sync.RWMutex
	closeErr     error

	subscriptionsLock sync.RWMutex
	subscriptions     map[string]notificationReceiver
	// receivers is a mapping from receiver channel to a set of corresponding subscription IDs.
	// It must be accessed with subscriptionsLock taken. Its keys must be used to deliver
	// notifications, if channel is not in the receivers list and corresponding subscription
	// still exists, notification must not be sent.
	receivers map[any][]string
	// subscriptionsOrderLock manages sequential order of "subscribe" and "unsubscribe" WS
	// requests processing in order to avoid server-side subscription ID conflicts.
	subscriptionsOrderLock sync.Mutex

	respLock     sync.RWMutex
	respChannels map[uint64]chan *neorpc.Response
}

// WSOptions defines options for the web-socket RPC client. It contains a
// set of options for the underlying standard RPC client as far as
// WSClient-specific options. See Options documentation for more details.
type WSOptions struct {
	Options
	// CloseNotificationChannelIfFull allows WSClient to close a subscriber's
	// receive channel in case if the channel isn't read properly and no more
	// events can be pushed to it. This option, if set, allows to avoid WSClient
	// blocking on a subsequent notification dispatch. However, if enabled, the
	// corresponding subscription is kept even after receiver's channel closing,
	// thus it's still the caller's duty to call Unsubscribe() for this
	// subscription.
	CloseNotificationChannelIfFull bool
}

// notificationReceiver is an interface aimed to provide WS subscriber functionality
// for different types of subscriptions.
type notificationReceiver interface {
	// Comparator provides notification filtering functionality.
	rpcevent.Comparator
	// Receiver returns notification receiver channel.
	Receiver() any
	// TrySend checks whether notification passes receiver filter and sends it
	// to the underlying channel if so. It is performed under subscriptions lock
	// taken. nonBlocking denotes whether the receiving operation shouldn't block
	// the client's operation. It returns whether notification matches the filter
	// and whether the receiver channel is overflown.
	TrySend(ntf Notification, nonBlocking bool) (bool, bool)
	// Close closes underlying receiver channel.
	Close()
}

// blockReceiver stores information about block events subscriber.
type blockReceiver struct {
	filter *neorpc.BlockFilter
	ch     chan<- *block.Block
}

// EventID implements neorpc.Comparator interface.
func (r *blockReceiver) EventID() neorpc.EventID {
	return neorpc.BlockEventID
}

// Filter implements neorpc.Comparator interface.
func (r *blockReceiver) Filter() neorpc.SubscriptionFilter {
	if r.filter == nil {
		return nil
	}
	return *r.filter
}

// Receiver implements notificationReceiver interface.
func (r *blockReceiver) Receiver() any {
	return r.ch
}

// TrySend implements notificationReceiver interface.
func (r *blockReceiver) TrySend(ntf Notification, nonBlocking bool) (bool, bool) {
	if rpcevent.Matches(r, ntf) {
		if nonBlocking {
			select {
			case r.ch <- ntf.Value.(*block.Block):
			default:
				return true, true
			}
		} else {
			r.ch <- ntf.Value.(*block.Block)
		}

		return true, false
	}
	return false, false
}

// Close implements notificationReceiver interface.
func (r *blockReceiver) Close() {
	close(r.ch)
}

// headerOfAddedBlockReceiver stores information about header of added block events subscriber.
type headerOfAddedBlockReceiver struct {
	filter *neorpc.BlockFilter
	ch     chan<- *block.Header
}

// EventID implements neorpc.Comparator interface.
func (r *headerOfAddedBlockReceiver) EventID() neorpc.EventID {
	return neorpc.HeaderOfAddedBlockEventID
}

// Filter implements neorpc.Comparator interface.
func (r *headerOfAddedBlockReceiver) Filter() neorpc.SubscriptionFilter {
	if r.filter == nil {
		return nil
	}
	return *r.filter
}

// Receiver implements notificationReceiver interface.
func (r *headerOfAddedBlockReceiver) Receiver() any {
	return r.ch
}

// TrySend implements notificationReceiver interface.
func (r *headerOfAddedBlockReceiver) TrySend(ntf Notification, nonBlocking bool) (bool, bool) {
	if rpcevent.Matches(r, ntf) {
		if nonBlocking {
			select {
			case r.ch <- ntf.Value.(*block.Header):
			default:
				return true, true
			}
		} else {
			r.ch <- ntf.Value.(*block.Header)
		}
		return true, false
	}
	return false, false
}

// Close implements notificationReceiver interface.
func (r *headerOfAddedBlockReceiver) Close() {
	close(r.ch)
}

// txReceiver stores information about transaction events subscriber.
type txReceiver struct {
	filter *neorpc.TxFilter
	ch     chan<- *transaction.Transaction
}

// EventID implements neorpc.Comparator interface.
func (r *txReceiver) EventID() neorpc.EventID {
	return neorpc.TransactionEventID
}

// Filter implements neorpc.Comparator interface.
func (r *txReceiver) Filter() neorpc.SubscriptionFilter {
	if r.filter == nil {
		return nil
	}
	return *r.filter
}

// Receiver implements notificationReceiver interface.
func (r *txReceiver) Receiver() any {
	return r.ch
}

// TrySend implements notificationReceiver interface.
func (r *txReceiver) TrySend(ntf Notification, nonBlocking bool) (bool, bool) {
	if rpcevent.Matches(r, ntf) {
		if nonBlocking {
			select {
			case r.ch <- ntf.Value.(*transaction.Transaction):
			default:
				return true, true
			}
		} else {
			r.ch <- ntf.Value.(*transaction.Transaction)
		}

		return true, false
	}
	return false, false
}

// Close implements notificationReceiver interface.
func (r *txReceiver) Close() {
	close(r.ch)
}

// executionNotificationReceiver stores information about execution notifications subscriber.
type executionNotificationReceiver struct {
	filter *neorpc.NotificationFilter
	ch     chan<- *state.ContainedNotificationEvent
}

// EventID implements neorpc.Comparator interface.
func (r *executionNotificationReceiver) EventID() neorpc.EventID {
	return neorpc.NotificationEventID
}

// Filter implements neorpc.Comparator interface.
func (r *executionNotificationReceiver) Filter() neorpc.SubscriptionFilter {
	if r.filter == nil {
		return nil
	}
	return *r.filter
}

// Receiver implements notificationReceiver interface.
func (r *executionNotificationReceiver) Receiver() any {
	return r.ch
}

// TrySend implements notificationReceiver interface.
func (r *executionNotificationReceiver) TrySend(ntf Notification, nonBlocking bool) (bool, bool) {
	if rpcevent.Matches(r, ntf) {
		if nonBlocking {
			select {
			case r.ch <- ntf.Value.(*state.ContainedNotificationEvent):
			default:
				return true, true
			}
		} else {
			r.ch <- ntf.Value.(*state.ContainedNotificationEvent)
		}

		return true, false
	}
	return false, false
}

// Close implements notificationReceiver interface.
func (r *executionNotificationReceiver) Close() {
	close(r.ch)
}

// executionReceiver stores information about application execution results subscriber.
type executionReceiver struct {
	filter *neorpc.ExecutionFilter
	ch     chan<- *state.AppExecResult
}

// EventID implements neorpc.Comparator interface.
func (r *executionReceiver) EventID() neorpc.EventID {
	return neorpc.ExecutionEventID
}

// Filter implements neorpc.Comparator interface.
func (r *executionReceiver) Filter() neorpc.SubscriptionFilter {
	if r.filter == nil {
		return nil
	}
	return *r.filter
}

// Receiver implements notificationReceiver interface.
func (r *executionReceiver) Receiver() any {
	return r.ch
}

// TrySend implements notificationReceiver interface.
func (r *executionReceiver) TrySend(ntf Notification, nonBlocking bool) (bool, bool) {
	if rpcevent.Matches(r, ntf) {
		if nonBlocking {
			select {
			case r.ch <- ntf.Value.(*state.AppExecResult):
			default:
				return true, true
			}
		} else {
			r.ch <- ntf.Value.(*state.AppExecResult)
		}

		return true, false
	}
	return false, false
}

// Close implements notificationReceiver interface.
func (r *executionReceiver) Close() {
	close(r.ch)
}

// notaryRequestReceiver stores information about notary requests subscriber.
type notaryRequestReceiver struct {
	filter *neorpc.NotaryRequestFilter
	ch     chan<- *result.NotaryRequestEvent
}

// EventID implements neorpc.Comparator interface.
func (r *notaryRequestReceiver) EventID() neorpc.EventID {
	return neorpc.NotaryRequestEventID
}

// Filter implements neorpc.Comparator interface.
func (r *notaryRequestReceiver) Filter() neorpc.SubscriptionFilter {
	if r.filter == nil {
		return nil
	}
	return *r.filter
}

// Receiver implements notificationReceiver interface.
func (r *notaryRequestReceiver) Receiver() any {
	return r.ch
}

// TrySend implements notificationReceiver interface.
func (r *notaryRequestReceiver) TrySend(ntf Notification, nonBlocking bool) (bool, bool) {
	if rpcevent.Matches(r, ntf) {
		if nonBlocking {
			select {
			case r.ch <- ntf.Value.(*result.NotaryRequestEvent):
			default:
				return true, true
			}
		} else {
			r.ch <- ntf.Value.(*result.NotaryRequestEvent)
		}

		return true, false
	}
	return false, false
}

// Close implements notificationReceiver interface.
func (r *notaryRequestReceiver) Close() {
	close(r.ch)
}

// mempoolEventReceiver stores information about mempool events subscriber.
type mempoolEventReceiver struct {
	filter *neorpc.MempoolEventFilter
	ch     chan<- *result.MempoolEvent
}

// EventID implements neorpc.Comparator interface.
func (r *mempoolEventReceiver) EventID() neorpc.EventID {
	return neorpc.MempoolEventID
}

// Filter implements neorpc.Comparator interface.
func (r *mempoolEventReceiver) Filter() neorpc.SubscriptionFilter {
	if r.filter == nil {
		return nil
	}
	return *r.filter
}

// Receiver implements notificationReceiver interface.
func (r *mempoolEventReceiver) Receiver() any {
	return r.ch
}

// TrySend implements notificationReceiver interface.
func (r *mempoolEventReceiver) TrySend(ntf Notification, nonBlocking bool) (bool, bool) {
	if rpcevent.Matches(r, ntf) {
		if nonBlocking {
			select {
			case r.ch <- ntf.Value.(*result.MempoolEvent):
			default:
				return true, true
			}
		} else {
			r.ch <- ntf.Value.(*result.MempoolEvent)
		}

		return true, false
	}
	return false, false
}

// Close implements notificationReceiver interface.
func (r *mempoolEventReceiver) Close() {
	close(r.ch)
}

// Notification represents a server-generated notification for client subscriptions.
// Value can be one of *block.Block, *state.AppExecResult, *state.ContainedNotificationEvent
// *transaction.Transaction or *subscriptions.NotaryRequestEvent based on Type.
type Notification struct {
	Type  neorpc.EventID
	Value any
}

// EventID implements Container interface and returns notification ID.
func (n Notification) EventID() neorpc.EventID {
	return n.Type
}

// EventPayload implements Container interface and returns notification
// object.
func (n Notification) EventPayload() any {
	return n.Value
}

// requestResponse is a combined type for request and response since we can get
// any of them here.
type requestResponse struct {
	neorpc.Response
	Method    string            `json:"method"`
	RawParams []json.RawMessage `json:"params,omitzero"`
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

// ErrNilNotificationReceiver is returned when notification receiver channel is nil.
var ErrNilNotificationReceiver = errors.New("nil notification receiver")

// ErrWSConnLost is a WSClient-specific error that will be returned for any
// requests after disconnection (including intentional ones via
// (*WSClient).Close). Use errors.Is(err, ErrConnClosedByUser) check to
// distinguish the latter case from a side reason of lost connection.
var ErrWSConnLost = errors.New("connection lost")

// ErrConnClosedByUser is a WSClient error returned iff RPC request is sent after
// the user calls (*WSClient).Close method by himself. Note that this error won't
// be returned from (*WSClient).GetError method.
var ErrConnClosedByUser = errors.New("closed by user")

// NewWS returns a new WSClient ready to use (with established websocket
// connection). You need to use websocket URL for it like `ws://1.2.3.4/ws`.
// You should call Init method to initialize the network magic the client is
// operating on.
func NewWS(ctx context.Context, endpoint string, opts WSOptions) (*WSClient, error) {
	dialer := websocket.Dialer{HandshakeTimeout: opts.DialTimeout}
	ws, resp, err := dialer.DialContext(ctx, endpoint, nil)
	if resp != nil && resp.Body != nil { // Can be non-nil even with error returned.
		defer resp.Body.Close() // Not exactly required by websocket, but let's do this for bodyclose checker.
	}
	if err != nil {
		if resp != nil && resp.Body != nil {
			var srvErr neorpc.HeaderAndError

			dec := json.NewDecoder(resp.Body)
			decErr := dec.Decode(&srvErr)
			if decErr == nil && srvErr.Error != nil {
				err = srvErr.Error
			}
		}
		return nil, err
	}
	wsc := &WSClient{
		Client: Client{},

		ws:            ws,
		wsOpts:        opts,
		shutdown:      make(chan struct{}),
		readerDone:    make(chan struct{}),
		writerDone:    make(chan struct{}),
		respChannels:  make(map[uint64]chan *neorpc.Response),
		requests:      make(chan *neorpc.Request),
		subscriptions: make(map[string]notificationReceiver),
		receivers:     make(map[any][]string),
	}

	err = initClient(ctx, &wsc.Client, endpoint, opts.Options)
	if err != nil {
		return nil, err
	}
	wsc.cli = nil

	go wsc.wsReader()
	go wsc.wsWriter()
	wsc.requestF = wsc.makeWsRequest
	return wsc, nil
}

// Close closes connection to the remote side rendering this client instance
// unusable.
func (c *WSClient) Close() {
	if c.closeCalled.CompareAndSwap(false, true) {
		c.setCloseErr(ErrConnClosedByUser)
		// Closing shutdown channel sends a signal to wsWriter to break out of the
		// loop. In doing so it does ws.Close() closing the network connection
		// which in turn makes wsReader receive an err from ws.ReadJSON() and also
		// break out of the loop closing c.done channel in its shutdown sequence.
		close(c.shutdown)
		// Call to cancel will send signal to all users of Context().
		c.ctxCancel()
	}
	<-c.readerDone
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
			ntf := Notification{Type: event}
			switch event {
			case neorpc.BlockEventID:
				sr, err := c.stateRootInHeader()
				if err != nil {
					// Client is not initialized.
					connCloseErr = fmt.Errorf("failed to fetch StateRootInHeader: %w", err)
					break readloop
				}
				ntf.Value = block.New(sr)
			case neorpc.TransactionEventID:
				ntf.Value = &transaction.Transaction{}
			case neorpc.NotificationEventID:
				ntf.Value = new(state.ContainedNotificationEvent)
			case neorpc.ExecutionEventID:
				ntf.Value = new(state.AppExecResult)
			case neorpc.NotaryRequestEventID:
				ntf.Value = new(result.NotaryRequestEvent)
			case neorpc.HeaderOfAddedBlockEventID:
				sr, err := c.stateRootInHeader()
				if err != nil {
					// Client is not initialized.
					connCloseErr = fmt.Errorf("failed to fetch StateRootInHeader: %w", err)
					break readloop
				}
				ntf.Value = &block.New(sr).Header
			case neorpc.MempoolEventID:
				ntf.Value = new(result.MempoolEvent)
			case neorpc.MissedEventID:
				// No value.
			default:
				// Bad event received.
				connCloseErr = fmt.Errorf("unknown event received: %d", event)
				break readloop
			}
			if event != neorpc.MissedEventID {
				err = json.Unmarshal(rr.RawParams[0], ntf.Value)
				if err != nil {
					// Bad event received.
					connCloseErr = fmt.Errorf("failed to unmarshal event of type %s from JSON: %w", event, err)
					break readloop
				}
			}
			c.notifySubscribers(ntf)
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
			select {
			case <-c.writerDone:
				break readloop
			case <-c.shutdown:
				break readloop
			case ch <- &rr.Response:
			}
		} else {
			// Malformed response, neither valid request, nor valid response.
			connCloseErr = fmt.Errorf("malformed response")
			break readloop
		}
	}
	if connCloseErr != nil {
		c.setCloseErr(connCloseErr)
	}
	close(c.readerDone)
	c.respLock.Lock()
	for _, ch := range c.respChannels {
		close(ch)
	}
	c.respChannels = nil
	c.respLock.Unlock()
	c.subscriptionsLock.Lock()
	for rcvrCh, ids := range c.receivers {
		c.dropSubCh(rcvrCh, ids[0], true)
	}
	c.subscriptionsLock.Unlock()
	c.ctxCancel()
}

// dropSubCh closes corresponding subscriber's channel and removes it from the
// receivers map. The channel is still being kept in
// the subscribers map as technically the server-side subscription still exists
// and the user is responsible for unsubscription. This method must be called
// under subscriptionsLock taken. It's the caller's duty to ensure dropSubCh
// will be called once per channel, otherwise panic will occur.
func (c *WSClient) dropSubCh(rcvrCh any, id string, ignoreCloseNotificationChannelIfFull bool) {
	if ignoreCloseNotificationChannelIfFull || c.wsOpts.CloseNotificationChannelIfFull {
		c.subscriptions[id].Close()
		delete(c.receivers, rcvrCh)
	}
}

func (c *WSClient) wsWriter() {
	pingTicker := time.NewTicker(wsPingPeriod)
	defer c.ws.Close()
	defer close(c.writerDone)
	var connCloseErr error
writeloop:
	for {
		select {
		case <-c.shutdown:
			return
		case <-c.readerDone:
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

func (c *WSClient) notifySubscribers(ntf Notification) {
	if ntf.Type == neorpc.MissedEventID {
		c.subscriptionsLock.Lock()
		for rcvr, ids := range c.receivers {
			c.subscriptions[ids[0]].Close()
			delete(c.receivers, rcvr)
		}
		c.subscriptionsLock.Unlock()
		return
	}
	c.subscriptionsLock.Lock()
	for rcvrCh, ids := range c.receivers {
		for _, id := range ids {
			ok, dropCh := c.subscriptions[id].TrySend(ntf, c.wsOpts.CloseNotificationChannelIfFull)
			if dropCh {
				c.dropSubCh(rcvrCh, id, false)
				break // strictly single drop per channel
			}
			if ok {
				break // strictly one notification per channel
			}
		}
	}
	c.subscriptionsLock.Unlock()
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

// closeErrOrConnLost returns ErrWSConnLost with details (set by wsReader or
// wsWriter) if available.
func (c *WSClient) closeErrOrConnLost() error {
	var (
		err      = ErrWSConnLost
		closeErr = c.getErrorOrClosedByUser()
	)
	if closeErr != nil {
		err = fmt.Errorf("%w: %w", err, closeErr)
	}
	return err
}

func (c *WSClient) makeWsRequest(r *neorpc.Request) (*neorpc.Response, error) {
	ch := make(chan *neorpc.Response)
	c.respLock.Lock()
	select {
	case <-c.readerDone:
		c.respLock.Unlock()
		return nil, fmt.Errorf("before registering response channel: %w", c.closeErrOrConnLost())
	default:
		c.respChannels[r.ID] = ch
		c.respLock.Unlock()
	}
	select {
	case <-c.readerDone:
		return nil, fmt.Errorf("before sending the request: %w", c.closeErrOrConnLost())
	case <-c.writerDone:
		return nil, fmt.Errorf("before sending the request: %w", c.closeErrOrConnLost())
	case c.requests <- r:
	}
	select {
	case <-c.readerDone:
		return nil, fmt.Errorf("waiting for the response: %w", c.closeErrOrConnLost())
	case <-c.writerDone:
		return nil, fmt.Errorf("waiting for the response: %w", c.closeErrOrConnLost())
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("waiting for the response: %w", c.closeErrOrConnLost())
		}
		c.unregisterRespChannel(r.ID)
		return resp, nil
	}
}

func (c *WSClient) performSubscription(params []any, rcvr notificationReceiver) (string, error) {
	var resp string

	if flt := rcvr.Filter(); flt != nil {
		if err := flt.IsValid(); err != nil {
			return "", err
		}
	}

	// Protect from concurrent subscribe/ubsubscribe requests, ref. #3093.
	c.subscriptionsOrderLock.Lock()
	defer c.subscriptionsOrderLock.Unlock()
	if err := c.performRequest("subscribe", params, &resp); err != nil {
		return "", err
	}

	c.subscriptionsLock.Lock()
	defer c.subscriptionsLock.Unlock()

	c.subscriptions[resp] = rcvr
	ch := rcvr.Receiver()
	c.receivers[ch] = append(c.receivers[ch], resp)
	return resp, nil
}

// ReceiveBlocks registers provided channel as a receiver for the new block events.
// Events can be filtered by the given BlockFilter, nil value doesn't add any filter.
// See WSClient comments for generic Receive* behaviour details.
func (c *WSClient) ReceiveBlocks(flt *neorpc.BlockFilter, rcvr chan<- *block.Block) (string, error) {
	if rcvr == nil {
		return "", ErrNilNotificationReceiver
	}
	if !c.cache.initDone {
		return "", errNetworkNotInitialized
	}
	params := []any{"block_added"}
	if flt != nil {
		flt = flt.Copy()
		params = append(params, *flt)
	}
	r := &blockReceiver{
		filter: flt,
		ch:     rcvr,
	}
	return c.performSubscription(params, r)
}

// ReceiveHeadersOfAddedBlocks registers provided channel as a receiver for new
// block's header events. Events can be filtered by the given [neorpc.BlockFilter],
// nil value doesn't add any filter. See WSClient comments for generic
// Receive* behaviour details.
func (c *WSClient) ReceiveHeadersOfAddedBlocks(flt *neorpc.BlockFilter, rcvr chan<- *block.Header) (string, error) {
	if rcvr == nil {
		return "", ErrNilNotificationReceiver
	}
	if !c.cache.initDone {
		return "", errNetworkNotInitialized
	}
	params := []any{"header_of_added_block"}
	if flt != nil {
		flt = flt.Copy()
		params = append(params, *flt)
	}
	r := &headerOfAddedBlockReceiver{
		filter: flt,
		ch:     rcvr,
	}
	return c.performSubscription(params, r)
}

// ReceiveTransactions registers provided channel as a receiver for new transaction
// events. Events can be filtered by the given TxFilter, nil value doesn't add any
// filter. See WSClient comments for generic Receive* behaviour details.
func (c *WSClient) ReceiveTransactions(flt *neorpc.TxFilter, rcvr chan<- *transaction.Transaction) (string, error) {
	if rcvr == nil {
		return "", ErrNilNotificationReceiver
	}
	params := []any{"transaction_added"}
	if flt != nil {
		flt = flt.Copy()
		params = append(params, *flt)
	}
	r := &txReceiver{
		filter: flt,
		ch:     rcvr,
	}
	return c.performSubscription(params, r)
}

// ReceiveExecutionNotifications registers provided channel as a receiver for execution
// events. Events can be filtered by the given NotificationFilter, nil value doesn't add
// any filter. See WSClient comments for generic Receive* behaviour details.
func (c *WSClient) ReceiveExecutionNotifications(flt *neorpc.NotificationFilter, rcvr chan<- *state.ContainedNotificationEvent) (string, error) {
	if rcvr == nil {
		return "", ErrNilNotificationReceiver
	}
	params := []any{"notification_from_execution"}
	if flt != nil {
		flt = flt.Copy()
		params = append(params, *flt)
	}
	r := &executionNotificationReceiver{
		filter: flt,
		ch:     rcvr,
	}
	return c.performSubscription(params, r)
}

// ReceiveExecutions registers provided channel as a receiver for
// application execution result events generated during transaction execution.
// Events can be filtered by the given ExecutionFilter, nil value doesn't add any filter.
// See WSClient comments for generic Receive* behaviour details.
func (c *WSClient) ReceiveExecutions(flt *neorpc.ExecutionFilter, rcvr chan<- *state.AppExecResult) (string, error) {
	if rcvr == nil {
		return "", ErrNilNotificationReceiver
	}
	params := []any{"transaction_executed"}
	if flt != nil {
		flt = flt.Copy()
		params = append(params, *flt)
	}
	r := &executionReceiver{
		filter: flt,
		ch:     rcvr,
	}
	return c.performSubscription(params, r)
}

// ReceiveNotaryRequests registers provided channel as a receiver for notary request
// payload addition or removal events. Events can be filtered by the given NotaryRequestFilter
// where sender corresponds to notary request sender (the second fallback transaction
// signer), signer corresponds to main transaction signers and type corresponds to the
// [mempoolevent.Type] and denotes whether notary request was added to or removed from
// the notary request pool. nil value doesn't add any filter. See WSClient comments
// for generic Receive* behaviour details.
func (c *WSClient) ReceiveNotaryRequests(flt *neorpc.NotaryRequestFilter, rcvr chan<- *result.NotaryRequestEvent) (string, error) {
	if rcvr == nil {
		return "", ErrNilNotificationReceiver
	}
	params := []any{"notary_request_event"}
	if flt != nil {
		flt = flt.Copy()
		params = append(params, *flt)
	}
	r := &notaryRequestReceiver{
		filter: flt,
		ch:     rcvr,
	}
	return c.performSubscription(params, r)
}

// ReceiveMempoolEvents registers the provided channel as a receiver for mempool
// transaction addition or removal events. Events can be filtered by the given
// MempoolEventFilter where sender corresponds to the transaction’s sender, signer
// corresponds to one of the transaction’s signers, and type corresponds to the
// [mempoolevent.Type] and denotes whether the transaction was added to or removed
// from the mempool. See WSClient comments for generic Receive* behaviour details.
func (c *WSClient) ReceiveMempoolEvents(flt *neorpc.MempoolEventFilter, rcvr chan<- *result.MempoolEvent) (string, error) {
	if rcvr == nil {
		return "", ErrNilNotificationReceiver
	}
	params := []any{"mempool_event"}
	if flt != nil {
		flt = flt.Copy()
		params = append(params, *flt)
	}
	r := &mempoolEventReceiver{
		filter: flt,
		ch:     rcvr,
	}
	return c.performSubscription(params, r)
}

// Unsubscribe removes subscription for the given event stream. It will return an
// error in case if there's no subscription with the provided ID. Call to Unsubscribe
// doesn't block notifications receive process for given subscriber, thus, ensure
// that subscriber channel is properly drained while unsubscription is being
// performed. Failing to do so will cause WSClient to block even regular requests.
// You may probably need to run unsubscription process in a separate
// routine (in parallel with notification receiver routine) to avoid Client's
// notification dispatcher blocking.
func (c *WSClient) Unsubscribe(id string) error {
	return c.performUnsubscription(id)
}

// UnsubscribeAll removes all active subscriptions of the current client. It copies
// the list of subscribers in order not to hold the lock for the whole execution
// time and tries to unsubscribe from us many feeds as possible returning the
// chunk of unsubscription errors afterwards. Call to UnsubscribeAll doesn't block
// notifications receive process for given subscribers, thus, ensure that subscribers
// channels are properly drained while unsubscription is being performed. Failing to
// do so will cause WSClient to block even regular requests. You may probably need
// to run unsubscription process in a separate routine (in parallel with notification
// receiver routines) to avoid Client's notification dispatcher blocking.
func (c *WSClient) UnsubscribeAll() error {
	c.subscriptionsLock.Lock()
	subs := make([]string, 0, len(c.subscriptions))
	for id := range c.subscriptions {
		subs = append(subs, id)
	}
	c.subscriptionsLock.Unlock()

	var resErr error
	for _, id := range subs {
		err := c.performUnsubscription(id)
		if err != nil {
			errFmt := "failed to unsubscribe from feed %d: %w"
			errArgs := []any{err}
			if resErr != nil {
				errFmt = "%w; " + errFmt
				errArgs = append([]any{resErr}, errArgs...)
			}
			resErr = fmt.Errorf(errFmt, errArgs...)
		}
	}
	return resErr
}

// performUnsubscription is internal method that removes subscription with the given
// ID from the list of subscriptions and receivers. It takes the subscriptions lock
// after WS RPC unsubscription request is completed. Until then the subscriber channel
// may still receive WS notifications.
func (c *WSClient) performUnsubscription(id string) error {
	// Protect from concurrent subscribe/ubsubscribe requests, ref. #3093.
	c.subscriptionsOrderLock.Lock()
	defer c.subscriptionsOrderLock.Unlock()

	c.subscriptionsLock.RLock()
	rcvr, ok := c.subscriptions[id]
	c.subscriptionsLock.RUnlock()

	if !ok {
		return errors.New("no subscription with this ID")
	}

	var resp bool
	if err := c.performRequest("unsubscribe", []any{id}, &resp); err != nil {
		return err
	}
	if !resp {
		return errors.New("unsubscribe method returned false result")
	}

	c.subscriptionsLock.Lock()
	defer c.subscriptionsLock.Unlock()

	// Rely on fact that rcvr is still in the c.subscriptions map since only
	// performUnsubscription (protected by subscriptionsOrderLock) is authorized
	// to remove rcvr from this map, although the rcvr channel itself may be
	// closed by this moment by notifySubscribers due to channel overflow or
	// missed event receival.
	ch := rcvr.Receiver()
	ids := c.receivers[ch]
	for i, rcvrID := range ids {
		if rcvrID == id {
			ids = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	if len(ids) == 0 {
		delete(c.receivers, ch)
	} else {
		c.receivers[ch] = ids
	}
	delete(c.subscriptions, id)
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
// was closed by the user via Close() method.
func (c *WSClient) GetError() error {
	err := c.getErrorOrClosedByUser()
	if err != nil && errors.Is(err, ErrConnClosedByUser) {
		return nil
	}
	return err
}

// getErrorOrClosedByUser returns the reason of WS connection closing. An error is returned
// even if connection was closed by the user via Close() method.
func (c *WSClient) getErrorOrClosedByUser() error {
	c.closeErrLock.RLock()
	defer c.closeErrLock.RUnlock()

	return c.closeErr
}

// Context returns WSClient Cancel context that will be terminated on Client shutdown.
func (c *WSClient) Context() context.Context {
	return c.ctx
}
