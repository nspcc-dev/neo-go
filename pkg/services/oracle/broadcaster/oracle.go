package broadcaster

import (
	"encoding/base64"
	"encoding/binary"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/rpcbroadcaster"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle"
	"go.uber.org/zap"
)

const (
	defaultSendTimeout = time.Second * 4

	defaultChanCapacity = 16
)

type oracleBroadcaster struct {
	rpcbroadcaster.RPCBroadcaster
}

// New returns a new struct capable of broadcasting oracle responses.
func New(cfg config.OracleConfiguration, log *zap.Logger) oracle.Broadcaster {
	if cfg.ResponseTimeout == 0 {
		cfg.ResponseTimeout = defaultSendTimeout
	}
	r := &oracleBroadcaster{
		RPCBroadcaster: *rpcbroadcaster.NewRPCBroadcaster(log, cfg.ResponseTimeout),
	}
	for i := range cfg.Nodes {
		r.Clients[cfg.Nodes[i]] = r.NewRPCClient(cfg.Nodes[i], (*client.Client).SubmitRawOracleResponse,
			cfg.ResponseTimeout, make(chan request.RawParams, defaultChanCapacity))
	}
	return r
}

// SendResponse implements interfaces.Broadcaster.
func (r *oracleBroadcaster) SendResponse(priv *keys.PrivateKey, resp *transaction.OracleResponse, txSig []byte) {
	pub := priv.PublicKey()
	data := GetMessage(pub.Bytes(), resp.ID, txSig)
	msgSig := priv.Sign(data)
	params := request.NewRawParams(
		base64.StdEncoding.EncodeToString(pub.Bytes()),
		resp.ID,
		base64.StdEncoding.EncodeToString(txSig),
		base64.StdEncoding.EncodeToString(msgSig),
	)
	r.SendParams(params)
}

// GetMessage returns data which is signed upon sending response by RPC.
func GetMessage(pubBytes []byte, reqID uint64, txSig []byte) []byte {
	data := make([]byte, len(pubBytes)+8+len(txSig))
	copy(data, pubBytes)
	binary.LittleEndian.PutUint64(data[len(pubBytes):], uint64(reqID))
	copy(data[len(pubBytes)+8:], txSig)
	return data
}
