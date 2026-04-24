package crypto

import (
	"crypto/elliptic"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// ECDSASecp256r1CheckMultisig checks multiple ECDSA signatures at once using
// Secp256r1 elliptic curve.
func ECDSASecp256r1CheckMultisig(ic *interop.Context) error {
	pkeys, err := ic.VM.Estack().PopSigElements()
	if err != nil {
		return fmt.Errorf("wrong key parameters: %w", err)
	}
	sigs, err := ic.VM.Estack().PopSigElements()
	if err != nil {
		return fmt.Errorf("wrong signature parameters: %w", err)
	}
	if err := ic.VM.AddPicoGas(ic.BaseExecFee() * fee.ECDSAVerifyPrice * int64(len(pkeys))); err != nil {
		return err
	}
	// It's ok to have more keys than there are signatures (it would
	// just mean that some keys didn't sign), but not the other way around.
	if len(pkeys) < len(sigs) {
		return errors.New("more signatures than there are keys")
	}
	if ic.IsHardforkEnabled(config.HFGorgon) {
		for i, s := range sigs {
			if len(s) != keys.SignatureLen {
				return fmt.Errorf("wrong signature #%d length: expected %d, got %d", i, keys.SignatureLen, len(s))
			}
		}
	}
	sigok := vm.CheckMultisigPar(elliptic.P256(), hash.NetSha256(ic.Network, ic.Container).BytesBE(), pkeys, sigs)
	ic.VM.Estack().PushItem(stackitem.Bool(sigok))
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
	if ic.IsHardforkEnabled(config.HFGorgon) && len(signature) != keys.SignatureLen {
		return fmt.Errorf("wrong signature length: expected %d, got %d", keys.SignatureLen, len(signature))
	}
	res := pkey.VerifyHashable(signature, ic.Network, ic.Container)
	ic.VM.Estack().PushItem(stackitem.Bool(res))
	return nil
}
