package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wallet"
	"github.com/pkg/errors"
)

var (
	defaultDialTimeout    = 4 * time.Second
	defaultRequestTimeout = 4 * time.Second
	defaultClientVersion  = "2.0"
)

// Client represents the middleman for executing JSON RPC calls
// to remote NEO RPC nodes.
type Client struct {
	// The underlying http client. It's never a good practice to use
	// the http.DefaultClient, therefore we will role our own.
	http.Client
	endpoint *url.URL
	ctx      context.Context
	version  string
	Wif      *wallet.WIF
	Balancer BalanceGetter
}

// ClientOptions defines options for the RPC client.
// All Values are optional. If any duration is not specified
// a default of 3 seconds will be used.
type ClientOptions struct {
	Cert           string
	Key            string
	CACert         string
	DialTimeout    time.Duration
	RequestTimeout time.Duration
	// Version is the version of the client that will be send
	// along with the request body. If no version is specified
	// the default version (currently 2.0) will be used.
	Version string
}

// NewClient return a new Client ready to use.
func NewClient(ctx context.Context, endpoint string, opts ClientOptions) (*Client, error) {
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if opts.DialTimeout == 0 {
		opts.DialTimeout = defaultDialTimeout
	}
	if opts.RequestTimeout == 0 {
		opts.RequestTimeout = defaultRequestTimeout
	}
	if opts.Version == "" {
		opts.Version = defaultClientVersion
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: opts.DialTimeout,
		}).DialContext,
	}

	// TODO(@antdm): Enable SSL.
	if opts.Cert != "" && opts.Key != "" {

	}

	return &Client{
		Client: http.Client{
			Timeout:   opts.RequestTimeout,
			Transport: transport,
		},
		endpoint: url,
		ctx:      ctx,
		version:  opts.Version,
	}, nil
}

// SetWIF decodes given WIF and adds some wallet
// data to client. Useful for RPC calls that require an open wallet.
func (c *Client) SetWIF(wif string) error {
	decodedWif, err := wallet.WIFDecode(wif, 0x00)
	if err != nil {
		return errors.Wrap(err, "Failed to decode WIF; failed to add WIF to client ")
	}
	c.Wif = decodedWif
	return nil
}

func (c *Client) SetBalancer(b BalanceGetter) {
	c.Balancer = b
}

func (c *Client) performRequest(method string, p params, v interface{}) error {
	var (
		r = request{
			JSONRPC: c.version,
			Method:  method,
			Params:  p.values,
			ID:      1,
		}
		buf = new(bytes.Buffer)
	)

	if err := json.NewEncoder(buf).Encode(r); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.endpoint.String(), buf)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote responded with a non 200 response: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(v)
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
