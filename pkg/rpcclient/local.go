package rpcclient

import (
	"context"

	"github.com/nspcc-dev/neo-go/pkg/neorpc"
)

// InternalHook is a function signature that is required to create a local client
// (see NewInternal). It performs registration of local client's event channel
// and returns a request handler function.
type InternalHook func(context.Context, chan<- neorpc.Notification) func(*neorpc.Request) (*neorpc.Response, error)

// Internal is an experimental "local" client that does not connect to RPC via
// network. It's made for deeply integrated applications like NeoFS that have
// blockchain running in the same process, so use it only if you know what you're
// doing. It provides the same interface WSClient does.
type Internal struct {
	WSClient

	events chan neorpc.Notification
}

// NewInternal creates an instance of internal client. It accepts a method
// provided by RPC server.
func NewInternal(ctx context.Context, register InternalHook) (*Internal, error) {
	c := &Internal{
		WSClient: WSClient{
			Client: Client{},

			shutdown:      make(chan struct{}),
			done:          make(chan struct{}),
			subscriptions: make(map[string]notificationReceiver),
			receivers:     make(map[any][]string),
		},
		events: make(chan neorpc.Notification),
	}

	err := initClient(ctx, &c.WSClient.Client, "localhost:0", Options{})
	if err != nil {
		return nil, err // Can't really happen for internal client.
	}
	c.cli = nil
	go c.eventLoop()
	// c.ctx is inherited from ctx in fact (see initClient).
	c.requestF = register(c.ctx, c.events) //nolint:contextcheck // Non-inherited new context, use function like `context.WithXXX` instead
	return c, nil
}

func (c *Internal) eventLoop() {
eventloop:
	for {
		select {
		case <-c.ctx.Done():
			break eventloop
		case <-c.shutdown:
			break eventloop
		case ev := <-c.events:
			ntf := Notification{Type: ev.Event}
			if len(ev.Payload) > 0 {
				ntf.Value = ev.Payload[0]
			}
			c.notifySubscribers(ntf)
		}
	}
	close(c.done)
	c.ctxCancel()
	// ctx is cancelled, server is notified and will finish soon.
drainloop:
	for {
		select {
		case <-c.events:
		default:
			break drainloop
		}
	}
	close(c.events)
}
