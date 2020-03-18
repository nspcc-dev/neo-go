package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm"
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

func getMessage(_ *interop.Context, item vm.StackItem) []byte {
	msg, err := item.TryBytes()
	if err != nil {
		panic(err)
	}
	return msg
}
