package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Sha256 returns sha256 hash of the data.
func Sha256(ic *interop.Context, v *vm.VM) error {
	msg := getMessage(ic, v.Estack().Pop().Item())
	h := hash.Sha256(msg).BytesBE()
	v.Estack().PushVal(h)
	return nil
}

// RipeMD160 returns RipeMD160 hash of the data.
func RipeMD160(ic *interop.Context, v *vm.VM) error {
	msg := getMessage(ic, v.Estack().Pop().Item())
	h := hash.RipeMD160(msg).BytesBE()
	v.Estack().PushVal(h)
	return nil
}
