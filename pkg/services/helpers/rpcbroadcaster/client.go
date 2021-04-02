package rpcbroadcaster

import (
	"context"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"go.uber.org/zap"
)

// RPCClient represent rpc client for a single node.
type RPCClient struct {
	client      *client.Client
	addr        string
	close       chan struct{}
	responses   chan request.RawParams
	log         *zap.Logger
	sendTimeout time.Duration
	method      SendMethod
}

// SendMethod represents rpc method for sending data to other nodes.
type SendMethod func(*client.Client, request.RawParams) error

// NewRPCClient returns new rpc client for provided address and method.
func (r *RPCBroadcaster) NewRPCClient(addr string, method SendMethod, timeout time.Duration, ch chan request.RawParams) *RPCClient {
	return &RPCClient{
		addr:        addr,
		close:       r.close,
		responses:   ch,
		log:         r.Log.With(zap.String("address", addr)),
		sendTimeout: timeout,
		method:      method,
	}
}

func (c *RPCClient) run() {
	// We ignore error as not every node can be available on startup.
	c.client, _ = client.New(context.Background(), c.addr, client.Options{
		DialTimeout:    c.sendTimeout,
		RequestTimeout: c.sendTimeout,
	})
	for {
		select {
		case <-c.close:
			return
		case ps := <-c.responses:
			if c.client == nil {
				var err error
				c.client, err = client.New(context.Background(), c.addr, client.Options{
					DialTimeout:    c.sendTimeout,
					RequestTimeout: c.sendTimeout,
				})
				if err != nil {
					continue
				}
			}
			err := c.method(c.client, ps)
			if err != nil {
				c.log.Error("error while submitting oracle response", zap.Error(err))
			}
		}
	}
}
