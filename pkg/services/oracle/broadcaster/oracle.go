package broadcaster

import (
	"encoding/base64"
	"encoding/binary"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle"
	"go.uber.org/zap"
)

type rpcBroascaster struct {
	clients map[string]*oracleClient
	log     *zap.Logger

	close       chan struct{}
	responses   chan request.RawParams
	sendTimeout time.Duration
}

const (
	defaultSendTimeout = time.Second * 4

	defaultChanCapacity = 16
)

// New returns new struct capable of broadcasting oracle responses.
func New(cfg config.OracleConfiguration, log *zap.Logger) oracle.Broadcaster {
	if cfg.ResponseTimeout == 0 {
		cfg.ResponseTimeout = defaultSendTimeout
	}
	r := &rpcBroascaster{
		clients:     make(map[string]*oracleClient, len(cfg.Nodes)),
		log:         log,
		close:       make(chan struct{}),
		responses:   make(chan request.RawParams),
		sendTimeout: cfg.ResponseTimeout,
	}
	for i := range cfg.Nodes {
		r.clients[cfg.Nodes[i]] = r.newOracleClient(cfg.Nodes[i], cfg.ResponseTimeout, make(chan request.RawParams, defaultChanCapacity))
	}
	return r
}

// Run implements oracle.Broadcaster.
func (r *rpcBroascaster) Run() {
	for _, c := range r.clients {
		go c.run()
	}
	for {
		select {
		case <-r.close:
			return
		case ps := <-r.responses:
			for _, c := range r.clients {
				select {
				case c.responses <- ps:
				default:
					c.log.Error("can't send response, channel is full")
				}
			}
		}
	}
}

// Shutdown implements oracle.Broadcaster.
func (r *rpcBroascaster) Shutdown() {
	close(r.close)
}

// SendResponse implements interfaces.Broadcaster.
func (r *rpcBroascaster) SendResponse(priv *keys.PrivateKey, resp *transaction.OracleResponse, txSig []byte) {
	pub := priv.PublicKey()
	data := GetMessage(pub.Bytes(), resp.ID, txSig)
	msgSig := priv.Sign(data)
	params := request.NewRawParams(
		base64.StdEncoding.EncodeToString(pub.Bytes()),
		resp.ID,
		base64.StdEncoding.EncodeToString(txSig),
		base64.StdEncoding.EncodeToString(msgSig),
	)
	r.responses <- params
}

// GetMessage returns data which is signed upon sending response by RPC.
func GetMessage(pubBytes []byte, reqID uint64, txSig []byte) []byte {
	data := make([]byte, len(pubBytes)+8+len(txSig))
	copy(data, pubBytes)
	binary.LittleEndian.PutUint64(data[len(pubBytes):], uint64(reqID))
	copy(data[len(pubBytes)+8:], txSig)
	return data
}
