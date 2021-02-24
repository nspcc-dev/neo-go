/*
Package crypto provides an interface to cryptographic syscalls.
*/
package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// SHA256 computes SHA256 hash of b. It uses `Neo.Crypto.SHA256` syscall.
func SHA256(b []byte) interop.Hash256 {
	return neogointernal.Syscall1("Neo.Crypto.SHA256", b).(interop.Hash256)
}

// RIPEMD160 computes RIPEMD160 hash of b. It uses `Neo.Crypto.RIPEMD160` syscall.
func RIPEMD160(b []byte) interop.Hash160 {
	return neogointernal.Syscall1("Neo.Crypto.RIPEMD160", b).(interop.Hash160)
}

// ECDsaSecp256r1Verify checks that sig is correct msg's signature for a given pub
// (serialized public key). It uses `Neo.Crypto.VerifyWithECDsaSecp256r1` syscall.
func ECDsaSecp256r1Verify(msg []byte, pub interop.PublicKey, sig interop.Signature) bool {
	return neogointernal.Syscall3("Neo.Crypto.VerifyWithECDsaSecp256r1", msg, pub, sig).(bool)
}

// ECDsaSecp256k1Verify checks that sig is correct msg's signature for a given pub
// (serialized public key). It uses `Neo.Crypto.VerifyWithECDsaSecp256k1` syscall.
func ECDsaSecp256k1Verify(msg []byte, pub interop.PublicKey, sig interop.Signature) bool {
	return neogointernal.Syscall3("Neo.Crypto.VerifyWithECDsaSecp256k1", msg, pub, sig).(bool)
}

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
