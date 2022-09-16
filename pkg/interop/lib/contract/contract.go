package contract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
)

// CallWithVersion is a utility function that executes the previously deployed
// blockchain contract with the specified version (update counter) and hash
// (20 bytes in BE form) using the provided arguments and call flags. It fails
// if the contract has version mismatch. It returns whatever this contract
// returns. This function uses `System.Contract.Call` syscall.
func CallWithVersion(scriptHash interop.Hash160, version int, method string, f contract.CallFlag, args ...interface{}) interface{} {
	cs := management.GetContract(scriptHash)
	if cs == nil {
		panic("unknown contract")
	}
	if cs.UpdateCounter != version {
		panic("contract version mismatch")
	}
	return contract.Call(scriptHash, method, f, args...)
}
