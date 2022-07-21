package rpcbroadcaster

import (
	"context"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"go.uber.org/zap"
)

// RPCClient represent an rpc client for a single node.
type RPCClient struct {
	client      *rpcclient.Client
	addr        string
	close       chan struct{}
	finished    chan struct{}
	responses   chan []interface{}
	log         *zap.Logger
	sendTimeout time.Duration
	method      SendMethod
}

// SendMethod represents an rpc method for sending data to other nodes.
type SendMethod func(*rpcclient.Client, []interface{}) error

// NewRPCClient returns a new rpc client for the provided address and method.
func (r *RPCBroadcaster) NewRPCClient(addr string, method SendMethod, timeout time.Duration, ch chan []interface{}) *RPCClient {
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
	c.client, _ = rpcclient.New(context.Background(), c.addr, rpcclient.Options{
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
				c.client, err = rpcclient.New(context.Background(), c.addr, rpcclient.Options{
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
