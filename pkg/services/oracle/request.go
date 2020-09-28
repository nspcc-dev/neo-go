package oracle

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"go.uber.org/zap"
)

// RemoveRequests removes all data associated with requests
// which have been processed by oracle contract.
func (o *Oracle) RemoveRequests(ids []uint64) {
	o.respMtx.Lock()
	defer o.respMtx.Unlock()
	for _, id := range ids {
		delete(o.responses, id)
	}
}

// AddRequests saves all requests in-fly for further processing.
func (o *Oracle) AddRequests(reqs map[uint64]*state.OracleRequest) {
	if len(reqs) == 0 {
		return
	}

	select {
	case o.requestMap <- reqs:
	default:
		select {
		case old := <-o.requestMap:
			for id, r := range old {
				reqs[id] = r
			}
		default:
		}
		o.requestMap <- reqs
	}
}

// ProcessRequestsInternal processes provided requests synchronously.
func (o *Oracle) ProcessRequestsInternal(reqs map[uint64]*state.OracleRequest) {
	acc := o.getAccount()
	if acc == nil {
		return
	}

	// Process actual requests.
	for id := range reqs {
		if err := o.processRequest(acc.PrivateKey(), id, reqs[id]); err != nil {
			o.Log.Debug("can't process request", zap.Error(err))
		}
	}
}

func (o *Oracle) processRequest(priv *keys.PrivateKey, id uint64, req *state.OracleRequest) error {
	resp := &transaction.OracleResponse{ID: id}
	u, err := url.ParseRequestURI(req.URL)
	if err == nil && !o.MainCfg.AllowPrivateHost {
		err = o.URIValidator(u)
	}
	if err != nil {
		resp.Code = transaction.Forbidden
	} else if u.Scheme == "http" {
		r, err := o.Client.Get(req.URL)
		switch {
		case err != nil:
			resp.Code = transaction.Error
		case r.StatusCode == http.StatusOK:
			result, err := readResponse(r.Body, transaction.MaxOracleResultSize)
			if err != nil {
				if errors.Is(err, ErrResponseTooLarge) {
					resp.Code = transaction.ResponseTooLarge
				} else {
					resp.Code = transaction.Error
				}
				break
			}
			resp.Code = transaction.Success
			resp.Result = result
		case r.StatusCode == http.StatusForbidden:
			resp.Code = transaction.Forbidden
		case r.StatusCode == http.StatusNotFound:
			resp.Code = transaction.NotFound
		case r.StatusCode == http.StatusRequestTimeout:
			resp.Code = transaction.Timeout
		default:
			resp.Code = transaction.Error
		}
	}

	currentHeight := o.Chain.BlockHeight()
	_, h, err := o.Chain.GetTransaction(req.OriginalTxID)
	if err != nil {
		if !errors.Is(err, storage.ErrKeyNotFound) {
			return err
		}
		// The only reason tx can be not found is if it wasn't yet persisted from DAO.
		h = currentHeight
	}
	tx, err := o.CreateResponseTx(int64(req.GasForResponse), h, resp)
	if err != nil {
		return err
	}
	backupTx, err := o.CreateResponseTx(int64(req.GasForResponse), h, &transaction.OracleResponse{
		ID:   id,
		Code: transaction.ConsensusUnreachable,
	})
	if err != nil {
		return err
	}

	incTx := o.getResponse(id)
	incTx.Lock()
	incTx.tx = tx
	incTx.backupTx = backupTx
	incTx.reverifyTx()

	txSig := priv.Sign(tx.GetSignedPart())
	incTx.addResponse(priv.PublicKey(), txSig, false)

	backupSig := priv.Sign(backupTx.GetSignedPart())
	incTx.addResponse(priv.PublicKey(), backupSig, true)

	readyTx, ready := incTx.finalize(o.getOracleNodes())
	if ready {
		ready = !incTx.isSent
		incTx.isSent = true
	}
	incTx.Unlock()

	o.getBroadcaster().SendResponse(priv, resp, txSig)
	if ready {
		o.getOnTransaction()(readyTx)
	}
	return nil
}
