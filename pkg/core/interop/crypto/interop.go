package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
)

var (
	neoCryptoCheckMultisigID = interopnames.ToID([]byte(interopnames.SystemCryptoCheckMultisig))
	neoCryptoCheckSigID      = interopnames.ToID([]byte(interopnames.SystemCryptoCheckSig))
)

// Interops represents sorted crypto-related interop functions.
var Interops = []interop.Function{
	{ID: neoCryptoCheckMultisigID, Func: ECDSASecp256r1CheckMultisig},
	{ID: neoCryptoCheckSigID, Func: ECDSASecp256r1CheckSig},
}

func init() {
	interop.Sort(Interops)
}
