/*
Package crypto provides an interface to cryptographic syscalls.
*/
package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// CheckMultisig checks that a script container (transaction) is signed by multiple
// ECDSA keys at once. It uses `System.Crypto.CheckMultisig` syscall.
func CheckMultisig(pubs []interop.PublicKey, sigs []interop.Signature) bool {
	return neogointernal.Syscall2("System.Crypto.CheckMultisig", pubs, sigs).(bool)
}

// CheckSig checks that sig is a correct signature of the script container
// (transaction) for the given pub (serialized public key). It uses
// `System.Crypto.CheckSig` syscall.
func CheckSig(pub interop.PublicKey, sig interop.Signature) bool {
	return neogointernal.Syscall2("System.Crypto.CheckSig", pub, sig).(bool)
}
