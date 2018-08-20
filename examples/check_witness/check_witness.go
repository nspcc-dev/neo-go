package check_witness_contract

import (
	"github.com/CityOfZion/neo-go-sc/interop/runtime"
	"github.com/CityOfZion/neo-go-sc/interop/util"
)

// Check if the invoker of the contract is the specified owner.

var owner = util.FromAddress("Aej1fe4mUgou48Zzup5j8sPrE3973cJ5oz")

func Main() bool {
	if runtime.CheckWitness(owner) {
		return true
	}
	return false
}
