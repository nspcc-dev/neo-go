package management

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents Management contract hash.
const Hash = "\xfd\xa3\xfa\x43\x46\xea\x53\x2a\x25\x8f\xc4\x97\xdd\xad\xdb\x64\x37\xc9\xfd\xff"

// Deploy represents `deploy` method of Management native contract.
func Deploy(script, manifest []byte) *Contract {
	return contract.Call(interop.Hash160(Hash), "deploy",
		contract.States|contract.AllowNotify, script, manifest).(*Contract)
}

// DeployWithData represents `deploy` method of Management native contract.
func DeployWithData(script, manifest []byte, data interface{}) *Contract {
	return contract.Call(interop.Hash160(Hash), "deploy",
		contract.States|contract.AllowNotify, script, manifest, data).(*Contract)
}

// Destroy represents `destroy` method of Management native contract.
func Destroy() {
	contract.Call(interop.Hash160(Hash), "destroy", contract.States|contract.AllowNotify)
}

// GetContract represents `getContract` method of Management native contract.
func GetContract(addr interop.Hash160) *Contract {
	return contract.Call(interop.Hash160(Hash), "getContract", contract.ReadStates, addr).(*Contract)
}

// GetMinimumDeploymentFee represents `getMinimumDeploymentFee` method of Management native contract.
func GetMinimumDeploymentFee() int {
	return contract.Call(interop.Hash160(Hash), "getMinimumDeploymentFee", contract.ReadStates).(int)
}

// SetMinimumDeploymentFee represents `setMinimumDeploymentFee` method of Management native contract.
func SetMinimumDeploymentFee(value int) {
	contract.Call(interop.Hash160(Hash), "setMinimumDeploymentFee", contract.States, value)
}

// Update represents `update` method of Management native contract.
func Update(script, manifest []byte) {
	contract.Call(interop.Hash160(Hash), "update",
		contract.States|contract.AllowNotify, script, manifest)
}

// UpdateWithData represents `update` method of Management native contract.
func UpdateWithData(script, manifest []byte, data interface{}) {
	contract.Call(interop.Hash160(Hash), "update",
		contract.States|contract.AllowNotify, script, manifest, data)
}
