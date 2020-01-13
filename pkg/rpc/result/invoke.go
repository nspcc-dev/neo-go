package result

import (
	"github.com/CityOfZion/neo-go/pkg/vm"
)

// Invoke represents code invocation result and is used by several RPC calls
// that invoke functions, scripts and generic bytecode.
type Invoke struct {
	State       string `json:"state"`
	GasConsumed string `json:"gas_consumed"`
	Script      string `json:"script"`
	Stack       *vm.Stack
}
