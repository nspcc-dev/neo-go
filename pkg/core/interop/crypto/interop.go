package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
)

var (
	neoCryptoCheckMultisigID = interopnames.ToID([]byte(interopnames.SystemCryptoCheckMultisig))
	neoCryptoCheckSigID      = interopnames.ToID([]byte(interopnames.SystemCryptoCheckSig))
)

var cryptoInterops = []interop.Function{
	{ID: neoCryptoCheckMultisigID, Func: ECDSASecp256r1CheckMultisig},
	{ID: neoCryptoCheckSigID, Func: ECDSASecp256r1CheckSig},
}

func init() {
	interop.Sort(cryptoInterops)
}

// Register adds crypto interops to ic.
func Register(ic *interop.Context) {
	ic.Functions = append(ic.Functions, cryptoInterops)
}
