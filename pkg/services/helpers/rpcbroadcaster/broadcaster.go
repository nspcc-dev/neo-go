package rpcbroadcaster

import (
	"time"

	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"go.uber.org/zap"
)

// RPCBroadcaster represents a generic RPC broadcaster.
type RPCBroadcaster struct {
	Clients   map[string]*RPCClient
	Log       *zap.Logger
	Responses chan request.RawParams

	close       chan struct{}
	sendTimeout time.Duration
}

// NewRPCBroadcaster returns a new RPC broadcaster instance.
func NewRPCBroadcaster(log *zap.Logger, sendTimeout time.Duration) *RPCBroadcaster {
	return &RPCBroadcaster{
		Clients:     make(map[string]*RPCClient),
		Log:         log,
		close:       make(chan struct{}),
		Responses:   make(chan request.RawParams),
		sendTimeout: sendTimeout,
	}
}

// Run implements oracle.Broadcaster.
func (r *RPCBroadcaster) Run() {
	for _, c := range r.Clients {
		go c.run()
	}
	for {
		select {
		case <-r.close:
			return
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
}

// Shutdown implements oracle.Broadcaster.
func (r *RPCBroadcaster) Shutdown() {
	close(r.close)
}
