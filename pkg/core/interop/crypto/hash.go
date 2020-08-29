package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Sha256 returns sha256 hash of the data.
func Sha256(ic *interop.Context) error {
	h, err := getMessageHash(ic, ic.VM.Estack().Pop().Item())
	if err != nil {
		return err
	}
	ic.VM.Estack().PushVal(h.BytesBE())
	return nil
}

// RipeMD160 returns RipeMD160 hash of the data.
func RipeMD160(ic *interop.Context) error {
	var msg []byte

	item := ic.VM.Estack().Pop().Item()
	switch val := item.(type) {
	case *stackitem.Interop:
		msg = val.Value().(crypto.Verifiable).GetSignedPart()
	case stackitem.Null:
		msg = ic.Container.GetSignedPart()
	default:
		var err error
		if msg, err = val.TryBytes(); err != nil {
			return err
		}
	}
	h := hash.RipeMD160(msg).BytesBE()
	ic.VM.Estack().PushVal(h)
	return nil
}
