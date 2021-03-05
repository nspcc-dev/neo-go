/*
Package crypto provides an interface to cryptographic syscalls.
*/
package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// ECDSASecp256r1CheckMultisig checks multiple ECDSA signatures at once. It uses
// `Neo.Crypto.CheckMultisigWithECDsaSecp256r1` syscall.
func ECDSASecp256r1CheckMultisig(msg []byte, pubs []interop.PublicKey, sigs []interop.Signature) bool {
	return neogointernal.Syscall3("Neo.Crypto.CheckMultisigWithECDsaSecp256r1", msg, pubs, sigs).(bool)
}

// ECDSASecp256k1CheckMultisig checks multiple ECDSA signatures at once. It uses
// `Neo.Crypto.CheckMultisigWithECDsaSecp256k1` syscall.
func ECDSASecp256k1CheckMultisig(msg []byte, pubs []interop.PublicKey, sigs []interop.Signature) bool {
	return neogointernal.Syscall3("Neo.Crypto.CheckMultisigWithECDsaSecp256k1", msg, pubs, sigs).(bool)
}

// CheckSig checks that sig is correct script-container's signature for a given pub
// (serialized public key). It uses `Neo.Crypto.CheckSig` syscall.
func CheckSig(pub interop.PublicKey, sig interop.Signature) bool {
	return neogointernal.Syscall2("Neo.Crypto.CheckSig", pub, sig).(bool)
}
