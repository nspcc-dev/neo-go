package testdata

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

// Verify is a verification contract method.
// It returns true iff it is signed by NTh9TnZTstvAePEYWDGLLxidBikJE24uTo (id-0 private key from testchain).
func Verify() bool {
	tx := runtime.GetScriptContainer()
	addr := util.FromAddress("NTh9TnZTstvAePEYWDGLLxidBikJE24uTo")
	return util.Equals(string(tx.Sender), string(addr))
}
