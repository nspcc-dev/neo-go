package oracle

import (
	"errors"
	"fmt"
	gio "io"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"go.uber.org/zap"
)

func (o *Oracle) getResponse(reqID uint64, create bool) *incompleteTx {
	o.respMtx.Lock()
	defer o.respMtx.Unlock()
	incTx, ok := o.responses[reqID]
	if !ok && create && !o.removed[reqID] {
		incTx = newIncompleteTx()
		o.responses[reqID] = incTx
	}
	return incTx
}

// AddResponse handles an oracle response (transaction signature for some identified request) signed by the given key.
// sig is a response transaction signature.
func (o *Oracle) AddResponse(pub *keys.PublicKey, reqID uint64, txSig []byte) {
	incTx := o.getResponse(reqID, true)
	if incTx == nil {
		return
	}

	incTx.Lock()
	isBackup := false
	if incTx.tx != nil {
		ok := pub.VerifyHashable(txSig, uint32(o.Network), incTx.tx)
		if !ok {
			ok = pub.VerifyHashable(txSig, uint32(o.Network), incTx.backupTx)
			if !ok {
				o.Log.Debug("invalid response signature",
					zap.String("pub", pub.StringCompressed()))
				incTx.Unlock()
				return
			}
			isBackup = true
		}
	}
	incTx.addResponse(pub, txSig, isBackup)
	readyTx, ready := incTx.finalize(o.getOracleNodes(), false)
	if ready {
		ready = !incTx.isSent
		incTx.isSent = true
	}
	incTx.Unlock()

	if ready {
		o.sendTx(readyTx)
	}
}

// ErrResponseTooLarge is returned when a response exceeds the max allowed size.
var ErrResponseTooLarge = errors.New("too big response")

func (o *Oracle) readResponse(rc gio.Reader, url string) ([]byte, transaction.OracleResponseCode) {
	const limit = transaction.MaxOracleResultSize
	buf := make([]byte, limit+1)
	n, err := gio.ReadFull(rc, buf)
	if errors.Is(err, gio.ErrUnexpectedEOF) && n <= limit {
		res, err := checkUTF8(buf[:n])
		return o.handleResponseError(res, err, url)
	}
	if err == nil || n > limit {
		return o.handleResponseError(nil, ErrResponseTooLarge, url)
	}

	return o.handleResponseError(nil, err, url)
}

func (o *Oracle) handleResponseError(data []byte, err error, url string) ([]byte, transaction.OracleResponseCode) {
	if err != nil {
		o.Log.Warn("failed to read data for oracle request", zap.String("url", url), zap.Error(err))
		if errors.Is(err, ErrResponseTooLarge) {
			return nil, transaction.ResponseTooLarge
		}
		return nil, transaction.Error
	}
	return data, transaction.Success
}

func checkUTF8(v []byte) ([]byte, error) {
	if !utf8.Valid(v) {
		return nil, errors.New("invalid UTF-8")
	}
	return v, nil
}

// CreateResponseTx creates an unsigned oracle response transaction.
func (o *Oracle) CreateResponseTx(gasForResponse int64, vub uint32, resp *transaction.OracleResponse) (*transaction.Transaction, error) {
	var respScript []byte
	o.oracleInfoLock.RLock()
	respScript = o.oracleResponse
	o.oracleInfoLock.RUnlock()

	tx := transaction.New(respScript, 0)
	tx.Nonce = uint32(resp.ID)
	tx.ValidUntilBlock = vub
	tx.Attributes = []transaction.Attribute{{
		Type:  transaction.OracleResponseT,
		Value: resp,
	}}

	oracleSignContract := o.getOracleSignContract()
	tx.Signers = []transaction.Signer{
		{
			Account: nativehashes.OracleContract,
			Scopes:  transaction.None,
		},
		{
			Account: hash.Hash160(oracleSignContract),
			Scopes:  transaction.None,
		},
	}
	tx.Scripts = []transaction.Witness{
		{}, // native contract witness is fixed, second witness is set later.
	}

	// Calculate network fee.
	size := io.GetVarSize(tx)
	tx.Scripts = append(tx.Scripts, transaction.Witness{VerificationScript: oracleSignContract})

	gasConsumed, ok, err := o.testVerify(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare `verify` invocation: %w", err)
	}
	if !ok {
		return nil, errors.New("can't verify transaction")
	}
	tx.NetworkFee += gasConsumed

	netFee, sizeDelta := fee.Calculate(o.Chain.GetBaseExecFee(), tx.Scripts[1].VerificationScript)
	tx.NetworkFee += netFee
	size += sizeDelta

	currNetFee := tx.NetworkFee + int64(size)*o.Chain.FeePerByte()
	if currNetFee > gasForResponse {
		attrSize := io.GetVarSize(tx.Attributes)
		resp.Code = transaction.InsufficientFunds
		resp.Result = nil
		size = size - attrSize + io.GetVarSize(tx.Attributes)
	}
	tx.NetworkFee += int64(size) * o.Chain.FeePerByte() // 233

	// Calculate system fee.
	tx.SystemFee = gasForResponse - tx.NetworkFee
	return tx, nil
}

func (o *Oracle) testVerify(tx *transaction.Transaction) (int64, bool, error) {
	// (*Blockchain).GetTestVM calls Hash() method of the provided transaction; once being called, this
	// method caches transaction hash, but tx building is not yet completed and hash will be changed.
	// So, make a copy of the tx to avoid wrong hash caching.
	cp := *tx
	ic, err := o.Chain.GetTestVM(trigger.Verification, &cp, nil)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create test VM: %w", err)
	}
	ic.VM.GasLimit = o.Chain.GetMaxVerificationGAS()

	o.oracleInfoLock.RLock()
	ic.VM.LoadScriptWithHash(o.oracleScript, nativehashes.OracleContract, callflag.ReadOnly)
	ic.VM.Context().Jump(o.verifyOffset)
	o.oracleInfoLock.RUnlock()

	ok := isVerifyOk(ic)
	return ic.VM.GasConsumed(), ok, nil
}

func isVerifyOk(ic *interop.Context) bool {
	defer ic.Finalize()
	if err := ic.VM.Run(); err != nil {
		return false
	}
	if ic.VM.Estack().Len() != 1 {
		return false
	}
	ok, err := ic.VM.Estack().Pop().Item().TryBool()
	return err == nil && ok
}

func getFailedResponse(id uint64) *transaction.OracleResponse {
	return &transaction.OracleResponse{
		ID:   id,
		Code: transaction.Error,
	}
}
