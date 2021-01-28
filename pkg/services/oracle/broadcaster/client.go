package broadcaster

import (
	"context"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"go.uber.org/zap"
)

type oracleClient struct {
	client      *client.Client
	addr        string
	close       chan struct{}
	responses   chan request.RawParams
	log         *zap.Logger
	sendTimeout time.Duration
}

func (r *rpcBroascaster) newOracleClient(addr string, timeout time.Duration, ch chan request.RawParams) *oracleClient {
	return &oracleClient{
		addr:        addr,
		close:       r.close,
		responses:   ch,
		log:         r.log.With(zap.String("address", addr)),
		sendTimeout: timeout,
	}
}

func (c *oracleClient) run() {
	// We ignore error as not every node can be available on startup.
	c.client, _ = client.New(context.Background(), "http://"+c.addr, client.Options{
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
				c.client, err = client.New(context.Background(), "http://"+c.addr, client.Options{
					DialTimeout:    c.sendTimeout,
					RequestTimeout: c.sendTimeout,
				})
				if err != nil {
					continue
				}
			}
			err := c.client.SubmitRawOracleResponse(ps)
			if err != nil {
				c.log.Error("error while submitting oracle response", zap.Error(err))
			}
		}
	}
}
