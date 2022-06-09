package contract

import (
	"crypto/elliptic"
	"errors"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// CreateMultisigAccount calculates multisig contract scripthash for a
// given m and a set of public keys.
func CreateMultisigAccount(ic *interop.Context) error {
	m := ic.VM.Estack().Pop().BigInt()
	mu64 := m.Uint64()
	if !m.IsUint64() || mu64 > math.MaxInt32 {
		return errors.New("m must be positive and fit int32")
	}
	arr := ic.VM.Estack().Pop().Array()
	pubs := make(keys.PublicKeys, len(arr))
	for i, pk := range arr {
		p, err := keys.NewPublicKeyFromBytes(pk.Value().([]byte), elliptic.P256())
		if err != nil {
			return err
		}
		pubs[i] = p
	}
	var invokeFee int64
	if ic.IsHardforkEnabled(config.HFAspidochelone) {
		invokeFee = fee.ECDSAVerifyPrice * int64(len(pubs))
	} else {
		invokeFee = 1 << 8
	}
	invokeFee *= ic.BaseExecFee()
	if !ic.VM.AddGas(invokeFee) {
		return errors.New("gas limit exceeded")
	}
	script, err := smartcontract.CreateMultiSigRedeemScript(int(mu64), pubs)
	if err != nil {
		return err
	}
	ic.VM.Estack().PushItem(stackitem.NewByteArray(hash.Hash160(script).BytesBE()))
	return nil
}

// CreateStandardAccount calculates contract scripthash for a given public key.
func CreateStandardAccount(ic *interop.Context) error {
	h := ic.VM.Estack().Pop().Bytes()
	p, err := keys.NewPublicKeyFromBytes(h, elliptic.P256())
	if err != nil {
		return err
	}
	var invokeFee int64
	if ic.IsHardforkEnabled(config.HFAspidochelone) {
		invokeFee = fee.ECDSAVerifyPrice
	} else {
		invokeFee = 1 << 8
	}
	invokeFee *= ic.BaseExecFee()
	if !ic.VM.AddGas(invokeFee) {
		return errors.New("gas limit exceeded")
	}
	ic.VM.Estack().PushItem(stackitem.NewByteArray(p.GetScriptHash().BytesBE()))
	return nil
}
