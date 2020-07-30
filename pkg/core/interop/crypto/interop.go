package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

var (
	ecdsaSecp256r1VerifyID        = emit.InteropNameToID([]byte("Neo.Crypto.VerifyWithECDsaSecp256r1"))
	ecdsaSecp256k1VerifyID        = emit.InteropNameToID([]byte("Neo.Crypto.VerifyWithECDsaSecp256k1"))
	ecdsaSecp256r1CheckMultisigID = emit.InteropNameToID([]byte("Neo.Crypto.CheckMultisigWithECDsaSecp256r1"))
	ecdsaSecp256k1CheckMultisigID = emit.InteropNameToID([]byte("Neo.Crypto.CheckMultisigWithECDsaSecp256k1"))
	sha256ID                      = emit.InteropNameToID([]byte("Neo.Crypto.SHA256"))
	ripemd160ID                   = emit.InteropNameToID([]byte("Neo.Crypto.RIPEMD160"))
)

var cryptoInterops = []interop.Function{
	{ID: ecdsaSecp256r1VerifyID, Func: ECDSASecp256r1Verify},
	{ID: ecdsaSecp256k1VerifyID, Func: ECDSASecp256k1Verify},
	{ID: ecdsaSecp256r1CheckMultisigID, Func: ECDSASecp256r1CheckMultisig},
	{ID: ecdsaSecp256k1CheckMultisigID, Func: ECDSASecp256k1CheckMultisig},
	{ID: sha256ID, Func: Sha256},
	{ID: ripemd160ID, Func: RipeMD160},
}

func init() {
	interop.Sort(cryptoInterops)
}

// Register adds crypto interops to ic.
func Register(ic *interop.Context) {
	ic.Functions = append(ic.Functions, cryptoInterops)
}
