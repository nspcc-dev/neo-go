package rpcbroadcaster

import (
	"time"

	"go.uber.org/zap"
)

// RPCBroadcaster represents a generic RPC broadcaster.
type RPCBroadcaster struct {
	Clients   map[string]*RPCClient
	Log       *zap.Logger
	Responses chan []any

	close       chan struct{}
	finished    chan struct{}
	sendTimeout time.Duration
}

// NewRPCBroadcaster returns a new RPC broadcaster instance.
func NewRPCBroadcaster(log *zap.Logger, sendTimeout time.Duration) *RPCBroadcaster {
	return &RPCBroadcaster{
		Clients:     make(map[string]*RPCClient),
		Log:         log,
		close:       make(chan struct{}),
		finished:    make(chan struct{}),
		Responses:   make(chan []any),
		sendTimeout: sendTimeout,
	}
}

// Run implements oracle.Broadcaster.
func (r *RPCBroadcaster) Run() {
	for _, c := range r.Clients {
		go c.run()
	}
run:
	for {
		select {
		case <-r.close:
			break run
		case ps := <-r.Responses:
			for _, c := range r.Clients {
				select {
				case c.responses <- ps:
				default:
					c.log.Error("can't send response, channel is full")
				}
			}
		}
	}
	for _, c := range r.Clients {
		<-c.finished
	}
drain:
	for {
		select {
		case <-r.Responses:
		default:
			break drain
		}
	}
	close(r.Responses)
	close(r.finished)
}

// SendParams sends a request using all clients if the broadcaster is active.
func (r *RPCBroadcaster) SendParams(params []any) {
	select {
	case <-r.close:
	case r.Responses <- params:
	}
}

// Shutdown implements oracle.Broadcaster. The same instance can't be Run again
// after the shutdown.
func (r *RPCBroadcaster) Shutdown() {
	close(r.close)
	<-r.finished
}
