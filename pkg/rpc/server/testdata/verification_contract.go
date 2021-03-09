package testdata

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

// Verify is a verification contract method.
// It returns true iff it is signed by NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc (id-0 private key from testchain).
func Verify() bool {
	tx := runtime.GetScriptContainer()
	addr := util.FromAddress("NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc")
	return util.Equals(string(tx.Sender), string(addr))
}
