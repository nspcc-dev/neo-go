package oracle

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle/neofs"
	"go.uber.org/zap"
)

const defaultMaxConcurrentRequests = 10

type request struct {
	ID  uint64
	Req *state.OracleRequest
}

func (o *Oracle) runRequestWorker() {
	for {
		select {
		case <-o.close:
			return
		case req := <-o.requestCh:
			acc := o.getAccount()
			if acc == nil {
				continue
			}
			err := o.processRequest(acc.PrivateKey(), req)
			if err != nil {
				o.Log.Debug("can't process request", zap.Uint64("id", req.ID), zap.Error(err))
			}
		}
	}
}

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
	for id, req := range reqs {
		if err := o.processRequest(acc.PrivateKey(), request{ID: id, Req: req}); err != nil {
			o.Log.Debug("can't process request", zap.Error(err))
		}
	}
}

func (o *Oracle) processRequest(priv *keys.PrivateKey, req request) error {
	if req.Req == nil {
		o.processFailedRequest(priv, req)
		return nil
	}

	incTx := o.getResponse(req.ID, true)
	if incTx == nil {
		return nil
	}
	resp := &transaction.OracleResponse{ID: req.ID}
	u, err := url.ParseRequestURI(req.Req.URL)
	if err == nil && !o.MainCfg.AllowPrivateHost {
		err = o.URIValidator(u)
	}
	if err != nil {
		resp.Code = transaction.Forbidden
	} else if u.Scheme == "http" {
		r, err := o.Client.Get(req.Req.URL)
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
			resp.Code, resp.Result = filterRequest(result, req.Req)
		case r.StatusCode == http.StatusForbidden:
			resp.Code = transaction.Forbidden
		case r.StatusCode == http.StatusNotFound:
			resp.Code = transaction.NotFound
		case r.StatusCode == http.StatusRequestTimeout:
			resp.Code = transaction.Timeout
		default:
			resp.Code = transaction.Error
		}
	} else if err == nil && u.Scheme == neofs.URIScheme {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(o.MainCfg.NeoFS.Timeout)*time.Millisecond)
		defer cancel()
		index := (int(req.ID) + incTx.attempts) % len(o.MainCfg.NeoFS.Nodes)
		res, err := neofs.Get(ctx, priv, u, o.MainCfg.NeoFS.Nodes[index])
		if err != nil {
			resp.Code = transaction.Error
		} else {
			resp.Code, resp.Result = filterRequest(res, req.Req)
		}
	}

	currentHeight := o.Chain.BlockHeight()
	_, h, err := o.Chain.GetTransaction(req.Req.OriginalTxID)
	if err != nil {
		if !errors.Is(err, storage.ErrKeyNotFound) {
			return err
		}
		// The only reason tx can be not found is if it wasn't yet persisted from DAO.
		h = currentHeight
	}
	tx, err := o.CreateResponseTx(int64(req.Req.GasForResponse), h, resp)
	if err != nil {
		return err
	}
	backupTx, err := o.CreateResponseTx(int64(req.Req.GasForResponse), h, &transaction.OracleResponse{
		ID:   req.ID,
		Code: transaction.ConsensusUnreachable,
	})
	if err != nil {
		return err
	}

	incTx.Lock()
	incTx.request = req.Req
	incTx.tx = tx
	incTx.backupTx = backupTx
	incTx.reverifyTx(o.Network)

	txSig := priv.SignHashable(uint32(o.Network), tx)
	incTx.addResponse(priv.PublicKey(), txSig, false)

	backupSig := priv.SignHashable(uint32(o.Network), backupTx)
	incTx.addResponse(priv.PublicKey(), backupSig, true)

	readyTx, ready := incTx.finalize(o.getOracleNodes(), false)
	if ready {
		ready = !incTx.isSent
		incTx.isSent = true
	}
	incTx.time = time.Now()
	incTx.attempts++
	incTx.Unlock()

	o.getBroadcaster().SendResponse(priv, resp, txSig)
	if ready {
		o.getOnTransaction()(readyTx)
	}
	return nil
}

func (o *Oracle) processFailedRequest(priv *keys.PrivateKey, req request) {
	// Request is being processed again.
	incTx := o.getResponse(req.ID, false)
	if incTx == nil {
		// Request was processed by other oracle nodes.
		return
	} else if incTx.isSent {
		// Tx was sent but not yet persisted. Try to pool it again.
		o.getOnTransaction()(incTx.tx)
		return
	}

	// Don't process request again, fallback to backup tx.
	incTx.Lock()
	readyTx, ready := incTx.finalize(o.getOracleNodes(), true)
	if ready {
		ready = !incTx.isSent
		incTx.isSent = true
	}
	incTx.time = time.Now()
	incTx.attempts++
	txSig := incTx.backupSigs[string(priv.PublicKey().Bytes())].sig
	incTx.Unlock()

	o.getBroadcaster().SendResponse(priv, getFailedResponse(req.ID), txSig)
	if ready {
		o.getOnTransaction()(readyTx)
	}
}
