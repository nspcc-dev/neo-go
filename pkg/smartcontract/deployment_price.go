package smartcontract

import "github.com/nspcc-dev/neo-go/pkg/util"

// GetDeploymentPrice returns contract deployment price based on its properties.
func GetDeploymentPrice(props PropertyState) util.Fixed8 {
	fee := int64(100)

	if props&HasStorage != 0 {
		fee += 400
	}

	if props&HasDynamicInvoke != 0 {
		fee += 500
	}

	return util.Fixed8FromInt64(fee)
}
