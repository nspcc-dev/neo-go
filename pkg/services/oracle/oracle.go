package oracle

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

type (
	// Oracle represents oracle module capable of talking
	// with the external world.
	Oracle struct {
		Config

		// mtx protects setting callbacks.
		mtx sync.RWMutex

		// accMtx protects account and oracle nodes.
		accMtx             sync.RWMutex
		currAccount        *wallet.Account
		oracleNodes        keys.PublicKeys
		oracleSignContract []byte

		// respMtx protects responses map.
		respMtx   sync.RWMutex
		responses map[uint64]*incompleteTx

		wallet *wallet.Wallet
	}

	// Config contains oracle module parameters.
	Config struct {
		Log             *zap.Logger
		Network         netmode.Magic
		Wallet          config.Wallet
		Client          HTTPClient
		Chain           blockchainer.Blockchainer
		ResponseHandler Broadcaster
		OnTransaction   TxCallback
		URIValidator    URIValidator
		OracleScript    []byte
		OracleResponse  []byte
		OracleHash      util.Uint160
	}

	// HTTPClient is an interface capable of doing oracle requests.
	HTTPClient interface {
		Get(string) (*http.Response, error)
	}

	// Broadcaster broadcasts oracle responses.
	Broadcaster interface {
		SendResponse(priv *keys.PrivateKey, resp *transaction.OracleResponse, txSig []byte)
	}

	defaultResponseHandler struct{}

	// TxCallback executes on new transactions when they are ready to be pooled.
	TxCallback = func(tx *transaction.Transaction)
	// URIValidator is used to check if provided URL is valid.
	URIValidator = func(*url.URL) error
)

const (
	// defaultRequestTimeout is default request timeout.
	defaultRequestTimeout = time.Second * 5
)

// NewOracle returns new oracle instance.
func NewOracle(cfg Config) (*Oracle, error) {
	o := &Oracle{
		Config: cfg,

		responses: make(map[uint64]*incompleteTx),
	}

	var err error
	w := cfg.Wallet
	if o.wallet, err = wallet.NewWalletFromFile(w.Path); err != nil {
		return nil, err
	}

	if o.Client == nil {
		var client http.Client
		client.Transport = &http.Transport{DisableKeepAlives: true}
		client.Timeout = defaultRequestTimeout
		o.Client = &client
	}
	if o.ResponseHandler == nil {
		o.ResponseHandler = defaultResponseHandler{}
	}
	if o.OnTransaction == nil {
		o.OnTransaction = func(*transaction.Transaction) {}
	}
	if o.URIValidator == nil {
		o.URIValidator = defaultURIValidator
	}
	return o, nil
}

func (o *Oracle) getBroadcaster() Broadcaster {
	o.mtx.RLock()
	defer o.mtx.RUnlock()
	return o.ResponseHandler
}

// SetBroadcaster sets callback to broadcast response.
func (o *Oracle) SetBroadcaster(b Broadcaster) {
	o.mtx.Lock()
	defer o.mtx.Unlock()
	o.ResponseHandler = b
}

// SendResponse implements Broadcaster interface.
func (defaultResponseHandler) SendResponse(*keys.PrivateKey, *transaction.OracleResponse, []byte) {
}
