package rpcclient

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// InvokeAndPackIteratorResults creates a script containing System.Contract.Call
// of the specified contract with the specified arguments. It assumes that the
// specified operation will return iterator. The script traverses the resulting
// iterator, packs all its values into array and pushes the resulting array on
// stack. Constructed script is invoked via `invokescript` JSON-RPC API using
// the provided signers. The result of the script invocation contains single array
// stackitem on stack if invocation HALTed. InvokeAndPackIteratorResults can be
// used to interact with JSON-RPC server where iterator sessions are disabled to
// retrieve iterator values via single `invokescript` JSON-RPC call. It returns
// maxIteratorResultItems items at max which is set to
// config.DefaultMaxIteratorResultItems by default.
//
// Deprecated: please use more convenient and powerful invoker.Invoker interface with
// CallAndExpandIterator method. This method will be removed in future versions.
func (c *Client) InvokeAndPackIteratorResults(contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer, maxIteratorResultItems ...int) (*result.Invoke, error) {
	max := config.DefaultMaxIteratorResultItems
	if len(maxIteratorResultItems) != 0 {
		max = maxIteratorResultItems[0]
	}
	values, err := smartcontract.ExpandParameterToEmitable(smartcontract.Parameter{
		Type:  smartcontract.ArrayType,
		Value: params,
	})
	if err != nil {
		return nil, fmt.Errorf("expanding params to emitable: %w", err)
	}
	bytes, err := smartcontract.CreateCallAndUnwrapIteratorScript(contract, operation, max, values.([]interface{})...)
	if err != nil {
		return nil, fmt.Errorf("failed to create iterator unwrapper script: %w", err)
	}
	return c.InvokeScript(bytes, signers)
}
