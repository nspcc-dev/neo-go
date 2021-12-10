/*
Package management provides interface to ContractManagement native contract.
It allows to get/deploy/update contracts as well as get/set deployment fee.
*/
package management

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Hash represents Management contract hash.
const Hash = "\xfd\xa3\xfa\x43\x46\xea\x53\x2a\x25\x8f\xc4\x97\xdd\xad\xdb\x64\x37\xc9\xfd\xff"

// Deploy represents `deploy` method of Management native contract.
func Deploy(script, manifest []byte) *Contract {
	return neogointernal.CallWithToken(Hash, "deploy",
		int(contract.States|contract.AllowNotify), script, manifest).(*Contract)
}

// DeployWithData represents `deploy` method of Management native contract.
func DeployWithData(script, manifest []byte, data interface{}) *Contract {
	return neogointernal.CallWithToken(Hash, "deploy",
		int(contract.States|contract.AllowNotify), script, manifest, data).(*Contract)
}

// Destroy represents `destroy` method of Management native contract.
func Destroy() {
	neogointernal.CallWithTokenNoRet(Hash, "destroy", int(contract.States|contract.AllowNotify))
}

// GetContract represents `getContract` method of Management native contract.
func GetContract(addr interop.Hash160) *Contract {
	return neogointernal.CallWithToken(Hash, "getContract", int(contract.ReadStates), addr).(*Contract)
}

// GetMinimumDeploymentFee represents `getMinimumDeploymentFee` method of Management native contract.
func GetMinimumDeploymentFee() int {
	return neogointernal.CallWithToken(Hash, "getMinimumDeploymentFee", int(contract.ReadStates)).(int)
}

// SetMinimumDeploymentFee represents `setMinimumDeploymentFee` method of Management native contract.
func SetMinimumDeploymentFee(value int) {
	neogointernal.CallWithTokenNoRet(Hash, "setMinimumDeploymentFee", int(contract.States), value)
}

// Update represents `update` method of Management native contract.
func Update(script, manifest []byte) {
	neogointernal.CallWithTokenNoRet(Hash, "update",
		int(contract.States|contract.AllowNotify), script, manifest)
}

// UpdateWithData represents `update` method of Management native contract.
func UpdateWithData(script, manifest []byte, data interface{}) {
	neogointernal.CallWithTokenNoRet(Hash, "update",
		int(contract.States|contract.AllowNotify), script, manifest, data)
}
