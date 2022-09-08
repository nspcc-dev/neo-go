package rpcclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/atomic"
)

const (
	defaultDialTimeout    = 4 * time.Second
	defaultRequestTimeout = 4 * time.Second
	// Number of blocks after which cache is expired.
	cacheTimeout = 100
)

// Client represents the middleman for executing JSON RPC calls
// to remote NEO RPC nodes. Client is thread-safe and can be used from
// multiple goroutines.
type Client struct {
	cli      *http.Client
	endpoint *url.URL
	ctx      context.Context
	opts     Options
	requestF func(*neorpc.Request) (*neorpc.Response, error)

	// reader is an Invoker that has no signers and uses current state,
	// it's used to implement various getters. It'll be removed eventually,
	// but for now it keeps Client's API compatibility.
	reader *invoker.Invoker

	cacheLock sync.RWMutex
	// cache stores RPC node related information the client is bound to.
	// cache is mostly filled in during Init(), but can also be updated
	// during regular Client lifecycle.
	cache cache

	latestReqID *atomic.Uint64
	// getNextRequestID returns an ID to be used for the subsequent request creation.
	// It is defined on Client, so that our testing code can override this method
	// for the sake of more predictable request IDs generation behavior.
	getNextRequestID func() uint64
}

// Options defines options for the RPC client.
// All values are optional. If any duration is not specified,
// a default of 4 seconds will be used.
type Options struct {
	// Cert is a client-side certificate, it doesn't work at the moment along
	// with the other two options below.
	Cert           string
	Key            string
	CACert         string
	DialTimeout    time.Duration
	RequestTimeout time.Duration
	// Limit total number of connections per host. No limit by default.
	MaxConnsPerHost int
}

// cache stores cache values for the RPC client methods.
type cache struct {
	initDone                 bool
	network                  netmode.Magic
	stateRootInHeader        bool
	calculateValidUntilBlock calculateValidUntilBlockCache
	nativeHashes             map[string]util.Uint160
}

// calculateValidUntilBlockCache stores a cached number of validators and
// cache expiration value in blocks.
type calculateValidUntilBlockCache struct {
	validatorsCount uint32
	expiresAt       uint32
}

// New returns a new Client ready to use. You should call Init method to
// initialize stateroot setting for the network the client is operating on if
// you plan using GetBlock*.
func New(ctx context.Context, endpoint string, opts Options) (*Client, error) {
	cl := new(Client)
	err := initClient(ctx, cl, endpoint, opts)
	if err != nil {
		return nil, err
	}
	return cl, nil
}

func initClient(ctx context.Context, cl *Client, endpoint string, opts Options) error {
	url, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	if opts.DialTimeout <= 0 {
		opts.DialTimeout = defaultDialTimeout
	}

	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = defaultRequestTimeout
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: opts.DialTimeout,
			}).DialContext,
			MaxConnsPerHost: opts.MaxConnsPerHost,
		},
		Timeout: opts.RequestTimeout,
	}

	// TODO(@antdm): Enable SSL.
	//	if opts.Cert != "" && opts.Key != "" {
	//	}

	cl.ctx = ctx
	cl.cli = httpClient
	cl.endpoint = url
	cl.cache = cache{
		nativeHashes: make(map[string]util.Uint160),
	}
	cl.latestReqID = atomic.NewUint64(0)
	cl.getNextRequestID = (cl).getRequestID
	cl.opts = opts
	cl.requestF = cl.makeHTTPRequest
	cl.reader = invoker.New(cl, nil)
	return nil
}

func (c *Client) getRequestID() uint64 {
	return c.latestReqID.Inc()
}

// Init sets magic of the network client connected to, stateRootInHeader option
// and native NEO, GAS and Policy contracts scripthashes. This method should be
// called before any header- or block-related requests in order to deserialize
// responses properly.
func (c *Client) Init() error {
	version, err := c.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get network magic: %w", err)
	}
	natives, err := c.GetNativeContracts()
	if err != nil {
		return fmt.Errorf("failed to get native contracts: %w", err)
	}

	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	c.cache.network = version.Protocol.Network
	c.cache.stateRootInHeader = version.Protocol.StateRootInHeader
	if version.Protocol.MillisecondsPerBlock == 0 {
		c.cache.network = version.Magic
		c.cache.stateRootInHeader = version.StateRootInHeader
	}
	for _, ctr := range natives {
		c.cache.nativeHashes[ctr.Manifest.Name] = ctr.Hash
	}

	c.cache.initDone = true
	return nil
}

// Close closes unused underlying networks connections.
func (c *Client) Close() {
	c.cli.CloseIdleConnections()
}

func (c *Client) performRequest(method string, p []interface{}, v interface{}) error {
	if p == nil {
		p = []interface{}{} // neo-project/neo-modules#742
	}
	var r = neorpc.Request{
		JSONRPC: neorpc.JSONRPCVersion,
		Method:  method,
		Params:  p,
		ID:      c.getNextRequestID(),
	}

	raw, err := c.requestF(&r)

	if raw != nil && raw.Error != nil {
		return raw.Error
	} else if err != nil {
		return err
	} else if raw == nil || raw.Result == nil {
		return errors.New("no result returned")
	}
	return json.Unmarshal(raw.Result, v)
}

func (c *Client) makeHTTPRequest(r *neorpc.Request) (*neorpc.Response, error) {
	var (
		buf = new(bytes.Buffer)
		raw = new(neorpc.Response)
	)

	if err := json.NewEncoder(buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.endpoint.String(), buf)
	if err != nil {
		return nil, err
	}
	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// The node might send us a proper JSON anyway, so look there first and if
	// it parses, it has more relevant data than HTTP error code.
	err = json.NewDecoder(resp.Body).Decode(raw)
	if err != nil {
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("HTTP %d/%s", resp.StatusCode, http.StatusText(resp.StatusCode))
		} else {
			err = fmt.Errorf("JSON decoding: %w", err)
		}
	}
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// Ping attempts to create a connection to the endpoint
// and returns an error if there is any.
func (c *Client) Ping() error {
	conn, err := net.DialTimeout("tcp", c.endpoint.Host, defaultDialTimeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
