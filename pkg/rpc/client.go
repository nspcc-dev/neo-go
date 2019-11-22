package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/pkg/errors"
)

const (
	defaultDialTimeout    = 4 * time.Second
	defaultRequestTimeout = 4 * time.Second
	defaultClientVersion  = "2.0"
)

// Client represents the middleman for executing JSON RPC calls
// to remote NEO RPC nodes.
type Client struct {
	// The underlying http client. It's never a good practice to use
	// the http.DefaultClient, therefore we will role our own.
	cliMu      *sync.Mutex
	cli        *http.Client
	endpoint   *url.URL
	ctx        context.Context
	version    string
	wifMu      *sync.Mutex
	wif        *keys.WIF
	balancerMu *sync.Mutex
	balancer   BalanceGetter
}

// ClientOptions defines options for the RPC client.
// All Values are optional. If any duration is not specified
// a default of 3 seconds will be used.
type ClientOptions struct {
	Cert        string
	Key         string
	CACert      string
	DialTimeout time.Duration
	Client      *http.Client
	// Version is the version of the client that will be send
	// along with the request body. If no version is specified
	// the default version (currently 2.0) will be used.
	Version string
}

// NewClient returns a new Client ready to use.
func NewClient(ctx context.Context, endpoint string, opts ClientOptions) (*Client, error) {
	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if opts.Version == "" {
		opts.Version = defaultClientVersion
	}

	if opts.Client == nil {
		opts.Client = &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: opts.DialTimeout,
				}).DialContext,
			},
		}
	}

	// TODO(@antdm): Enable SSL.
	if opts.Cert != "" && opts.Key != "" {
	}

	if opts.Client.Timeout == 0 {
		opts.Client.Timeout = defaultRequestTimeout
	}

	return &Client{
		ctx:        ctx,
		cli:        opts.Client,
		cliMu:      new(sync.Mutex),
		balancerMu: new(sync.Mutex),
		wifMu:      new(sync.Mutex),
		endpoint:   url,
		version:    opts.Version,
	}, nil
}

// WIF returns WIF structure associated with the client.
func (c *Client) WIF() keys.WIF {
	c.wifMu.Lock()
	defer c.wifMu.Unlock()
	return keys.WIF{
		Version:    c.wif.Version,
		Compressed: c.wif.Compressed,
		PrivateKey: c.wif.PrivateKey,
		S:          c.wif.S,
	}
}

// SetWIF decodes given WIF and adds some wallet
// data to client. Useful for RPC calls that require an open wallet.
func (c *Client) SetWIF(wif string) error {
	c.wifMu.Lock()
	defer c.wifMu.Unlock()
	decodedWif, err := keys.WIFDecode(wif, 0x00)
	if err != nil {
		return errors.Wrap(err, "Failed to decode WIF; failed to add WIF to client ")
	}
	c.wif = decodedWif
	return nil
}

// Balancer is a getter for balance field.
func (c *Client) Balancer() BalanceGetter {
	c.balancerMu.Lock()
	defer c.balancerMu.Unlock()
	return c.balancer
}

// SetBalancer is a setter for balance field.
func (c *Client) SetBalancer(b BalanceGetter) {
	c.balancerMu.Lock()
	defer c.balancerMu.Unlock()

	if b != nil {
		c.balancer = b
	}
}

// Client is a getter for client field.
func (c *Client) Client() *http.Client {
	c.cliMu.Lock()
	defer c.cliMu.Unlock()
	return c.cli
}

// SetClient is a setter for client field.
func (c *Client) SetClient(cli *http.Client) {
	c.cliMu.Lock()
	defer c.cliMu.Unlock()

	if cli != nil {
		c.cli = cli
	}
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
	resp, err := c.Client().Do(req)
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
