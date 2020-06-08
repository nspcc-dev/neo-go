package crypto

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// ECDSAVerify checks ECDSA signature.
func ECDSAVerify(ic *interop.Context, v *vm.VM) error {
	msg := getMessage(ic, v.Estack().Pop().Item())
	hashToCheck := hash.Sha256(msg).BytesBE()
	keyb := v.Estack().Pop().Bytes()
	signature := v.Estack().Pop().Bytes()
	pkey, err := keys.NewPublicKeyFromBytes(keyb)
	if err != nil {
		return err
	}
	res := pkey.Verify(signature, hashToCheck)
	v.Estack().PushVal(res)
	return nil
}

// ECDSACheckMultisig checks multiple ECDSA signatures at once.
func ECDSACheckMultisig(ic *interop.Context, v *vm.VM) error {
	msg := getMessage(ic, v.Estack().Pop().Item())
	hashToCheck := hash.Sha256(msg).BytesBE()
	pkeys, err := v.Estack().PopSigElements()
	if err != nil {
		return fmt.Errorf("wrong parameters: %s", err.Error())
	}
	sigs, err := v.Estack().PopSigElements()
	if err != nil {
		return fmt.Errorf("wrong parameters: %s", err.Error())
	}
	// It's ok to have more keys than there are signatures (it would
	// just mean that some keys didn't sign), but not the other way around.
	if len(pkeys) < len(sigs) {
		return errors.New("more signatures than there are keys")
	}
	sigok := vm.CheckMultisigPar(v, hashToCheck, pkeys, sigs)
	v.Estack().PushVal(sigok)
	return nil
}

func getMessage(ic *interop.Context, item stackitem.Item) []byte {
	var msg []byte
	switch val := item.(type) {
	case *stackitem.Interop:
		msg = val.Value().(crypto.Verifiable).GetSignedPart()
	case stackitem.Null:
		msg = ic.Container.GetSignedPart()
	default:
		var err error
		if msg, err = val.TryBytes(); err != nil {
			return nil
		}
	}
	return msg
}
