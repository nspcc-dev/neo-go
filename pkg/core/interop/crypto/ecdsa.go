package crypto

import (
	"crypto/elliptic"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// ECDSASecp256r1CheckMultisig checks multiple ECDSA signatures at once using
// Secp256r1 elliptic curve.
func ECDSASecp256r1CheckMultisig(ic *interop.Context) error {
	hashToCheck, err := getMessageHash(ic, ic.VM.Estack().Pop().Item())
	if err != nil {
		return err
	}
	pkeys, err := ic.VM.Estack().PopSigElements()
	if err != nil {
		return fmt.Errorf("wrong parameters: %w", err)
	}
	if !ic.VM.AddGas(ic.BaseExecFee() * fee.ECDSAVerifyPrice * int64(len(pkeys))) {
		return errors.New("gas limit exceeded")
	}
	sigs, err := ic.VM.Estack().PopSigElements()
	if err != nil {
		return fmt.Errorf("wrong parameters: %w", err)
	}
	// It's ok to have more keys than there are signatures (it would
	// just mean that some keys didn't sign), but not the other way around.
	if len(pkeys) < len(sigs) {
		return errors.New("more signatures than there are keys")
	}
	sigok := vm.CheckMultisigPar(ic.VM, elliptic.P256(), hashToCheck.BytesBE(), pkeys, sigs)
	ic.VM.Estack().PushVal(sigok)
	return nil
}

func getMessageHash(ic *interop.Context, item stackitem.Item) (util.Uint256, error) {
	var msg []byte
	switch val := item.(type) {
	case *stackitem.Interop:
		return val.Value().(crypto.Verifiable).GetSignedHash(), nil
	case stackitem.Null:
		return ic.Container.GetSignedHash(), nil
	default:
		var err error
		if msg, err = val.TryBytes(); err != nil {
			return util.Uint256{}, err
		}
	}
	return hash.Sha256(msg), nil
}

// ECDSASecp256r1CheckSig checks ECDSA signature using Secp256r1 elliptic curve.
func ECDSASecp256r1CheckSig(ic *interop.Context) error {
	hashToCheck := ic.Container.GetSignedHash()
	keyb := ic.VM.Estack().Pop().Bytes()
	signature := ic.VM.Estack().Pop().Bytes()
	pkey, err := keys.NewPublicKeyFromBytes(keyb, elliptic.P256())
	if err != nil {
		return err
	}
	res := pkey.Verify(signature, hashToCheck.BytesBE())
	ic.VM.Estack().PushVal(res)
	return nil
}
