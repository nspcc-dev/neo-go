package result

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
)

// Invoke represents code invocation result and is used by several RPC calls
// that invoke functions, scripts and generic bytecode.
type Invoke struct {
	State       string                    `json:"state"`
	GasConsumed int64                     `json:"gasconsumed,string"`
	Script      string                    `json:"script"`
	Stack       []smartcontract.Parameter `json:"stack"`
}
