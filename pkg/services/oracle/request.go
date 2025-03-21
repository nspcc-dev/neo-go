package oracle

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
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
	if !o.running {
		for _, id := range ids {
			delete(o.pending, id)
		}
	} else {
		for _, id := range ids {
			delete(o.responses, id)
		}
	}
}

// AddRequests saves all requests in-fly for further processing.
func (o *Oracle) AddRequests(reqs map[uint64]*state.OracleRequest) {
	if len(reqs) == 0 {
		return
	}

	o.respMtx.Lock()
	if !o.running {
		for id, r := range reqs {
			o.pending[id] = r
		}
		o.respMtx.Unlock()
		return
	}
	o.respMtx.Unlock()

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

// ProcessRequestsInternal processes the provided requests synchronously.
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
	resp := &transaction.OracleResponse{ID: req.ID, Code: transaction.Success}
	u, err := url.ParseRequestURI(req.Req.URL)
	if err != nil {
		o.Log.Warn("malformed oracle request", zap.String("url", req.Req.URL), zap.Error(err))
		resp.Code = transaction.ProtocolNotSupported
	} else {
		switch u.Scheme {
		case "https":
			httpReq, err := http.NewRequest("GET", req.Req.URL, nil)
			if err != nil {
				o.Log.Warn("failed to create http request", zap.String("url", req.Req.URL), zap.Error(err))
				resp.Code = transaction.Error
				break
			}
			httpReq.Header.Set("User-Agent", "NeoOracleService/3.0")
			httpReq.Header.Set("Content-Type", "application/json")
			r, err := o.Client.Do(httpReq)
			if err != nil {
				if errors.Is(err, ErrRestrictedRedirect) {
					resp.Code = transaction.Forbidden
				} else {
					resp.Code = transaction.Error
				}
				o.Log.Warn("oracle request failed", zap.String("url", req.Req.URL), zap.Error(err), zap.Stringer("code", resp.Code))
				break
			}
			defer r.Body.Close()
			switch r.StatusCode {
			case http.StatusOK:
				if !checkMediaType(r.Header.Get("Content-Type"), o.MainCfg.AllowedContentTypes) {
					resp.Code = transaction.ContentTypeNotSupported
					break
				}

				resp.Result, resp.Code = o.readResponse(r.Body, req.Req.URL)
			case http.StatusForbidden:
				resp.Code = transaction.Forbidden
			case http.StatusNotFound:
				resp.Code = transaction.NotFound
			case http.StatusRequestTimeout:
				resp.Code = transaction.Timeout
			default:
				resp.Code = transaction.Error
			}
		case neofs.URIScheme:
			if len(o.MainCfg.NeoFS.Nodes) == 0 {
				o.Log.Warn("no NeoFS nodes configured", zap.String("url", req.Req.URL))
				resp.Code = transaction.Error
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), o.MainCfg.NeoFS.Timeout)
			defer cancel()
			index := (int(req.ID) + incTx.attempts) % len(o.MainCfg.NeoFS.Nodes)
			rc, err := neofs.Get(ctx, priv, u, o.MainCfg.NeoFS.Nodes[index])
			if err != nil {
				resp.Code = transaction.Error
				o.Log.Warn("failed to perform oracle request", zap.String("url", req.Req.URL), zap.Error(err))
				if rc != nil {
					rc.Close() // intentionally skip the closing error, make it unified with Oracle `https` protocol.
				}
				break
			}
			resp.Result, resp.Code = o.readResponse(rc, req.Req.URL)
			rc.Close() // intentionally skip the closing error, make it unified with Oracle `https` protocol.
		default:
			resp.Code = transaction.ProtocolNotSupported
			o.Log.Warn("unknown oracle request scheme", zap.String("url", req.Req.URL))
		}
	}
	if resp.Code == transaction.Success {
		resp.Result, err = filterRequest(resp.Result, req.Req)
		if err != nil {
			o.Log.Warn("oracle filter failed", zap.Uint64("request", req.ID), zap.Error(err))
			resp.Code = transaction.Error
		}
	}
	o.Log.Debug("oracle request processed", zap.String("url", req.Req.URL), zap.Int("code", int(resp.Code)), zap.String("result", string(resp.Result)))

	currentHeight := o.Chain.BlockHeight()
	vubInc := o.Chain.GetMaxValidUntilBlockIncrement()
	_, h, err := o.Chain.GetTransaction(req.Req.OriginalTxID)
	if err != nil {
		if !errors.Is(err, storage.ErrKeyNotFound) {
			return err
		}
		// The only reason tx can be not found is that it hasn't been persisted from DAO yet.
		h = currentHeight
	}
	h += vubInc // Main tx is only valid for RequestHeight + ValidUntilBlock.
	tx, err := o.CreateResponseTx(int64(req.Req.GasForResponse), h, resp)
	if err != nil {
		return err
	}
	for h <= currentHeight { // Backup tx must be valid in any event.
		h += vubInc
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

	o.ResponseHandler.SendResponse(priv, resp, txSig)
	if ready {
		o.sendTx(readyTx)
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
		o.sendTx(incTx.tx)
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

	o.ResponseHandler.SendResponse(priv, getFailedResponse(req.ID), txSig)
	if ready {
		o.sendTx(readyTx)
	}
}

func checkMediaType(hdr string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}

	typ, _, err := mime.ParseMediaType(hdr)
	if err != nil {
		return false
	}

	return slices.Contains(allowed, typ)
}
