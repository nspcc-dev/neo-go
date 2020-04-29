package client

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

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/pkg/errors"
)

const (
	defaultDialTimeout    = 4 * time.Second
	defaultRequestTimeout = 4 * time.Second
	defaultClientVersion  = "2.0"
	// number of blocks after which cache is expired
	cacheTimeout = 100
)

// Client represents the middleman for executing JSON RPC calls
// to remote NEO RPC nodes.
type Client struct {
	cli      *http.Client
	endpoint *url.URL
	ctx      context.Context
	version  string
	wifMu    *sync.Mutex
	wif      *keys.WIF
	balancer request.BalanceGetter
	cache    cache
}

// Options defines options for the RPC client.
// All Values are optional. If any duration is not specified
// a default of 3 seconds will be used.
type Options struct {
	// Balancer is an implementation of request.BalanceGetter interface,
	// if not set then the default Client's implementation will be used, but
	// it relies on server support for `getunspents` RPC call which is
	// standard for neo-go, but only implemented as a plugin for C# node. So
	// you can override it here to use NeoScanServer for example.
	Balancer       request.BalanceGetter
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

// cache stores cache values for the RPC client methods
type cache struct {
	calculateValidUntilBlock calculateValidUntilBlockCache
}

// calculateValidUntilBlockCache stores cached number of validators and
// cache expiration value in blocks
type calculateValidUntilBlockCache struct {
	validatorsCount uint32
	expiresAt       uint32
}

// New returns a new Client ready to use.
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

	if opts.Version == "" {
		opts.Version = defaultClientVersion
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: opts.DialTimeout,
			}).DialContext,
		},
		Timeout: opts.RequestTimeout,
	}

	// TODO(@antdm): Enable SSL.
	if opts.Cert != "" && opts.Key != "" {
	}

	cl := &Client{
		ctx:      ctx,
		cli:      httpClient,
		wifMu:    new(sync.Mutex),
		endpoint: url,
		version:  opts.Version,
	}
	if opts.Balancer == nil {
		opts.Balancer = cl
	}
	cl.balancer = opts.Balancer
	return cl, nil
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

// CalculateInputs implements request.BalanceGetter interface and returns inputs
// array for the specified amount of given asset belonging to specified address.
// This implementation uses GetUnspents JSON-RPC call internally, so make sure
// your RPC server supports that.
func (c *Client) CalculateInputs(address string, asset util.Uint256, cost util.Fixed8) ([]transaction.Input, util.Fixed8, error) {
	var utxos state.UnspentBalances

	resp, err := c.GetUnspents(address)
	if err != nil {
		return nil, util.Fixed8(0), errors.Wrapf(err, "cannot get balance for address %v", address)
	}
	for _, ubi := range resp.Balance {
		if asset.Equals(ubi.AssetHash) {
			utxos = ubi.Unspents
			break
		}
	}
	return unspentsToInputs(utxos, cost)

}

func (c *Client) performRequest(method string, p request.RawParams, v interface{}) error {
	var (
		r = request.Raw{
			JSONRPC:   c.version,
			Method:    method,
			RawParams: p.Values,
			ID:        1,
		}
		buf = new(bytes.Buffer)
		raw = &response.Raw{}
	)

	if err := json.NewEncoder(buf).Encode(r); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.endpoint.String(), buf)
	if err != nil {
		return err
	}
	resp, err := c.cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// The node might send us proper JSON anyway, so look there first and if
	// it parses, then it has more relevant data than HTTP error code.
	err = json.NewDecoder(resp.Body).Decode(raw)
	if err == nil {
		if raw.Error != nil {
			err = raw.Error
		} else {
			err = json.Unmarshal(raw.Result, v)
		}
	} else if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP %d/%s", resp.StatusCode, http.StatusText(resp.StatusCode))
	} else {
		err = errors.Wrap(err, "JSON decoding")
	}

	return err
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
