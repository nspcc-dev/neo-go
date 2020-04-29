package client

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
)

// WSClient is a websocket-enabled RPC client that can be used with appropriate
// servers. It's supposed to be faster than Client because it has persistent
// connection to the server and at the same time is exposes some functionality
// that is only provided via websockets (like event subscription mechanism).
type WSClient struct {
	Client
	ws            *websocket.Conn
	done          chan struct{}
	notifications chan *request.In
	responses     chan *response.Raw
	requests      chan *request.Raw
	shutdown      chan struct{}
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
		Client:    *cl,
		ws:        ws,
		shutdown:  make(chan struct{}),
		done:      make(chan struct{}),
		responses: make(chan *response.Raw),
		requests:  make(chan *request.Raw),
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
	for {
		rr := new(requestResponse)
		c.ws.SetReadDeadline(time.Now().Add(wsPongLimit))
		err := c.ws.ReadJSON(rr)
		if err != nil {
			// Timeout/connection loss/malformed response.
			break
		}
		if rr.RawID == nil && rr.Method != "" {
			if c.notifications != nil {
				c.notifications <- &rr.In
			}
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
	if c.notifications != nil {
		close(c.notifications)
	}
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
