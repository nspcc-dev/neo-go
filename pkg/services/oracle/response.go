package oracle

import (
	"encoding/hex"
	"errors"
	gio "io"

	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
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

// AddResponse processes oracle response from node pub.
// sig is response transaction signature.
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
					zap.String("pub", hex.EncodeToString(pub.Bytes())))
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
		o.getOnTransaction()(readyTx)
	}
}

// ErrResponseTooLarge is returned when response exceeds max allowed size.
var ErrResponseTooLarge = errors.New("too big response")

func readResponse(rc gio.ReadCloser, limit int) ([]byte, error) {
	defer rc.Close()

	buf := make([]byte, limit+1)
	n, err := gio.ReadFull(rc, buf)
	if err == gio.ErrUnexpectedEOF && n <= limit {
		return buf[:n], nil
	}
	if err == nil || n > limit {
		return nil, ErrResponseTooLarge
	}
	return nil, err
}

// CreateResponseTx creates unsigned oracle response transaction.
func (o *Oracle) CreateResponseTx(gasForResponse int64, height uint32, resp *transaction.OracleResponse) (*transaction.Transaction, error) {
	tx := transaction.New(o.oracleResponse, 0)
	tx.Nonce = uint32(resp.ID)
	tx.ValidUntilBlock = height + transaction.MaxValidUntilBlockIncrement
	tx.Attributes = []transaction.Attribute{{
		Type:  transaction.OracleResponseT,
		Value: resp,
	}}

	oracleSignContract := o.getOracleSignContract()
	tx.Signers = []transaction.Signer{
		{
			Account: o.oracleHash,
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

	gasConsumed, ok := o.testVerify(tx)
	if !ok {
		return nil, errors.New("can't verify transaction")
	}
	tx.NetworkFee += gasConsumed

	netFee, sizeDelta := fee.Calculate(o.Chain.GetPolicer().GetBaseExecFee(), tx.Scripts[1].VerificationScript)
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

func (o *Oracle) testVerify(tx *transaction.Transaction) (int64, bool) {
	v := o.Chain.GetTestVM(trigger.Verification, tx, nil)
	v.GasLimit = o.Chain.GetPolicer().GetMaxVerificationGAS()
	v.LoadScriptWithHash(o.oracleScript, o.oracleHash, callflag.ReadOnly)
	v.Jump(v.Context(), o.verifyOffset)

	ok := isVerifyOk(v)
	return v.GasConsumed(), ok
}

func isVerifyOk(v *vm.VM) bool {
	if err := v.Run(); err != nil {
		return false
	}
	if v.Estack().Len() != 1 {
		return false
	}
	ok, err := v.Estack().Pop().Item().TryBool()
	return err == nil && ok
}

func getFailedResponse(id uint64) *transaction.OracleResponse {
	return &transaction.OracleResponse{
		ID:   id,
		Code: transaction.Error,
	}
}
