package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
)

var (
	ecdsaSecp256r1VerifyID        = interopnames.ToID([]byte(interopnames.NeoCryptoVerifyWithECDsaSecp256r1))
	ecdsaSecp256k1VerifyID        = interopnames.ToID([]byte(interopnames.NeoCryptoVerifyWithECDsaSecp256k1))
	ecdsaSecp256r1CheckMultisigID = interopnames.ToID([]byte(interopnames.NeoCryptoCheckMultisigWithECDsaSecp256r1))
	ecdsaSecp256k1CheckMultisigID = interopnames.ToID([]byte(interopnames.NeoCryptoCheckMultisigWithECDsaSecp256k1))
)

var cryptoInterops = []interop.Function{
	{ID: ecdsaSecp256r1VerifyID, Func: ECDSASecp256r1Verify},
	{ID: ecdsaSecp256k1VerifyID, Func: ECDSASecp256k1Verify},
	{ID: ecdsaSecp256r1CheckMultisigID, Func: ECDSASecp256r1CheckMultisig},
	{ID: ecdsaSecp256k1CheckMultisigID, Func: ECDSASecp256k1CheckMultisig},
}

func init() {
	interop.Sort(cryptoInterops)
}

// Register adds crypto interops to ic.
func Register(ic *interop.Context) {
	ic.Functions = append(ic.Functions, cryptoInterops)
}
