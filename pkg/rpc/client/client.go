package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	defaultDialTimeout    = 4 * time.Second
	defaultRequestTimeout = 4 * time.Second
	// Number of blocks after which cache is expired.
	cacheTimeout = 100
)

// Client represents the middleman for executing JSON RPC calls
// to remote NEO RPC nodes.
type Client struct {
	cli               *http.Client
	endpoint          *url.URL
	network           netmode.Magic
	stateRootInHeader bool
	initDone          bool
	ctx               context.Context
	opts              Options
	requestF          func(*request.Raw) (*response.Raw, error)
	cache             cache
}

// Options defines options for the RPC client.
// All values are optional. If any duration is not specified
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
	calculateValidUntilBlock calculateValidUntilBlockCache
	nativeHashes             map[string]util.Uint160
}

// calculateValidUntilBlockCache stores cached number of validators and
// cache expiration value in blocks.
type calculateValidUntilBlockCache struct {
	validatorsCount uint32
	expiresAt       uint32
}

// New returns a new Client ready to use. You should call Init method to
// initialize network magic the client is operating on.
func New(ctx context.Context, endpoint string, opts Options) (*Client, error) {
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
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

	cl := &Client{
		ctx:      ctx,
		cli:      httpClient,
		endpoint: url,
		cache: cache{
			nativeHashes: make(map[string]util.Uint160),
		},
	}
	cl.opts = opts
	cl.requestF = cl.makeHTTPRequest
	return cl, nil
}

// Init sets magic of the network client connected to and native NEO and GAS
// contracts scripthashes. This method should be called before any transaction-,
// header- or block-related requests in order to deserialize responses properly.
func (c *Client) Init() error {
	version, err := c.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get network magic: %w", err)
	}
	c.network = version.Magic
	c.stateRootInHeader = version.StateRootInHeader
	neoContractHash, err := c.GetContractStateByAddressOrName(nativenames.Neo)
	if err != nil {
		return fmt.Errorf("failed to get NEO contract scripthash: %w", err)
	}
	c.cache.nativeHashes[nativenames.Neo] = neoContractHash.Hash
	gasContractHash, err := c.GetContractStateByAddressOrName(nativenames.Gas)
	if err != nil {
		return fmt.Errorf("failed to get GAS contract scripthash: %w", err)
	}
	c.cache.nativeHashes[nativenames.Gas] = gasContractHash.Hash
	policyContractHash, err := c.GetContractStateByAddressOrName(nativenames.Policy)
	if err != nil {
		return fmt.Errorf("failed to get Policy contract scripthash: %w", err)
	}
	c.cache.nativeHashes[nativenames.Policy] = policyContractHash.Hash
	c.initDone = true
	return nil
}

func (c *Client) performRequest(method string, p request.RawParams, v interface{}) error {
	var r = request.Raw{
		JSONRPC:   request.JSONRPCVersion,
		Method:    method,
		RawParams: p.Values,
		ID:        1,
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

func (c *Client) makeHTTPRequest(r *request.Raw) (*response.Raw, error) {
	var (
		buf = new(bytes.Buffer)
		raw = new(response.Raw)
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

	// The node might send us proper JSON anyway, so look there first and if
	// it parses, then it has more relevant data than HTTP error code.
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

// Ping attempts to create a connection to the endpoint.
// and returns an error if there is one.
func (c *Client) Ping() error {
	conn, err := net.DialTimeout("tcp", c.endpoint.Host, defaultDialTimeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
