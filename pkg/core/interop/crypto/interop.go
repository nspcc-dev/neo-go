package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
)

var (
	ecdsaSecp256r1CheckMultisigID = interopnames.ToID([]byte(interopnames.NeoCryptoCheckMultisigWithECDsaSecp256r1))
	neoCryptoCheckSigID           = interopnames.ToID([]byte(interopnames.NeoCryptoCheckSig))
)

var cryptoInterops = []interop.Function{
	{ID: ecdsaSecp256r1CheckMultisigID, Func: ECDSASecp256r1CheckMultisig},
	{ID: neoCryptoCheckSigID, Func: ECDSASecp256r1CheckSig},
}

func init() {
	interop.Sort(cryptoInterops)
}

// Register adds crypto interops to ic.
func Register(ic *interop.Context) {
	ic.Functions = append(ic.Functions, cryptoInterops)
}
