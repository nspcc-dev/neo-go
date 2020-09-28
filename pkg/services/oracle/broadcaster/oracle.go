package broadcaster

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle"
	"go.uber.org/zap"
)

type rpcBroascaster struct {
	clients map[string]*client.Client
	log     *zap.Logger

	sendTimeout time.Duration
}

const (
	defaultSendTimeout = time.Second * 4
)

// New returns new struct capable of broadcasting oracle responses.
func New(cfg config.OracleConfiguration, log *zap.Logger) oracle.Broadcaster {
	if cfg.ResponseTimeout == 0 {
		cfg.ResponseTimeout = defaultSendTimeout
	}
	r := &rpcBroascaster{
		clients:     make(map[string]*client.Client, len(cfg.Nodes)),
		log:         log,
		sendTimeout: cfg.ResponseTimeout,
	}
	for i := range cfg.Nodes {
		// We ignore error as not every node can be available on startup.
		r.clients[cfg.Nodes[i]], _ = client.New(context.Background(), "http://"+cfg.Nodes[i], client.Options{
			DialTimeout:    cfg.ResponseTimeout,
			RequestTimeout: cfg.ResponseTimeout,
		})
	}
	return r
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
	for addr, c := range r.clients {
		if c == nil {
			var err error
			c, err = client.New(context.Background(), addr, client.Options{
				DialTimeout:    r.sendTimeout,
				RequestTimeout: r.sendTimeout,
			})
			if err != nil {
				r.log.Debug("can't connect to oracle node", zap.String("address", addr), zap.Error(err))
				continue
			}
			r.clients[addr] = c
		}
		err := c.SubmitRawOracleResponse(params)
		r.log.Debug("error during oracle response submit", zap.String("address", addr), zap.Error(err))
	}
}

// GetMessage returns data which is signed upon sending response by RPC.
func GetMessage(pubBytes []byte, reqID uint64, txSig []byte) []byte {
	data := make([]byte, len(pubBytes)+8+len(txSig))
	copy(data, pubBytes)
	binary.LittleEndian.PutUint64(data[len(pubBytes):], uint64(reqID))
	copy(data[len(pubBytes)+8:], txSig)
	return data
}
