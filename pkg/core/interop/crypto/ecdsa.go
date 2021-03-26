package crypto

import (
	"crypto/elliptic"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// ECDSASecp256r1CheckMultisig checks multiple ECDSA signatures at once using
// Secp256r1 elliptic curve.
func ECDSASecp256r1CheckMultisig(ic *interop.Context) error {
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
	sigok := vm.CheckMultisigPar(ic.VM, elliptic.P256(), hash.NetSha256(ic.Network, ic.Container).BytesBE(), pkeys, sigs)
	ic.VM.Estack().PushVal(sigok)
	return nil
}

// ECDSASecp256r1CheckSig checks ECDSA signature using Secp256r1 elliptic curve.
func ECDSASecp256r1CheckSig(ic *interop.Context) error {
	keyb := ic.VM.Estack().Pop().Bytes()
	signature := ic.VM.Estack().Pop().Bytes()
	pkey, err := keys.NewPublicKeyFromBytes(keyb, elliptic.P256())
	if err != nil {
		return err
	}
	res := pkey.VerifyHashable(signature, ic.Network, ic.Container)
	ic.VM.Estack().PushVal(res)
	return nil
}
