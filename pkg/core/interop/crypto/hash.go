package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
)

// Sha256 returns sha256 hash of the data.
func Sha256(ic *interop.Context) error {
	msg := getMessage(ic, ic.VM.Estack().Pop().Item())
	h := hash.Sha256(msg).BytesBE()
	ic.VM.Estack().PushVal(h)
	return nil
}

// RipeMD160 returns RipeMD160 hash of the data.
func RipeMD160(ic *interop.Context) error {
	msg := getMessage(ic, ic.VM.Estack().Pop().Item())
	h := hash.RipeMD160(msg).BytesBE()
	ic.VM.Estack().PushVal(h)
	return nil
}
