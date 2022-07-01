package rpcbroadcaster

import (
	"context"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"go.uber.org/zap"
)

// RPCClient represent an rpc client for a single node.
type RPCClient struct {
	client      *client.Client
	addr        string
	close       chan struct{}
	finished    chan struct{}
	responses   chan request.RawParams
	log         *zap.Logger
	sendTimeout time.Duration
	method      SendMethod
}

// SendMethod represents an rpc method for sending data to other nodes.
type SendMethod func(*client.Client, request.RawParams) error

// NewRPCClient returns a new rpc client for the provided address and method.
func (r *RPCBroadcaster) NewRPCClient(addr string, method SendMethod, timeout time.Duration, ch chan request.RawParams) *RPCClient {
	return &RPCClient{
		addr:        addr,
		close:       r.close,
		finished:    make(chan struct{}),
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
run:
	for {
		select {
		case <-c.close:
			break run
		case ps := <-c.responses:
			if c.client == nil {
				var err error
				c.client, err = client.New(context.Background(), c.addr, client.Options{
					DialTimeout:    c.sendTimeout,
					RequestTimeout: c.sendTimeout,
				})
				if err != nil {
					c.log.Error("failed to create client to submit oracle response", zap.Error(err))
					continue
				}
			}
			err := c.method(c.client, ps)
			if err != nil {
				c.log.Error("error while submitting oracle response", zap.Error(err))
			}
		}
	}
	c.client.Close()
drain:
	for {
		select {
		case <-c.responses:
		default:
			break drain
		}
	}
	close(c.finished)
}
