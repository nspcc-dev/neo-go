package state

import (
	"encoding/json"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// ContractInvocation contains method call information.
// The Arguments field will be nil if serialization of the arguments exceeds the predefined limit
// of [stackitem.MaxSerialized] (for security reasons). In that case Truncated will be set to true.
type ContractInvocation struct {
	Hash   util.Uint160 `json:"contract"`
	Method string       `json:"method"`
	// Arguments are the arguments as passed to the `args` parameter of System.Contract.Call
	// for use in the RPC Server and RPC Client.
	Arguments *stackitem.Array `json:"arguments"`
	// argumentsBytes is the serialized arguments used at the interop level.
	argumentsBytes []byte
	ArgumentsCount uint32 `json:"argumentscount"`
	Truncated      bool   `json:"truncated"`
}

// contractInvocationAux is an auxiliary struct for ContractInvocation JSON marshalling.
type contractInvocationAux struct {
	Hash           util.Uint160    `json:"hash"`
	Method         string          `json:"method"`
	Arguments      json.RawMessage `json:"arguments,omitempty"`
	ArgumentsCount uint32          `json:"argumentscount"`
	Truncated      bool            `json:"truncated"`
}

// NewContractInvocation returns a new ContractInvocation.
func NewContractInvocation(hash util.Uint160, method string, argBytes []byte, argCnt uint32) *ContractInvocation {
	return &ContractInvocation{
		Hash:           hash,
		Method:         method,
		argumentsBytes: argBytes,
		ArgumentsCount: argCnt,
		Truncated:      argBytes == nil,
	}
}

// DecodeBinary implements the Serializable interface.
func (ci *ContractInvocation) DecodeBinary(r *io.BinReader) {
	ci.Hash.DecodeBinary(r)
	ci.Method = r.ReadString()
	ci.ArgumentsCount = r.ReadU32LE()
	ci.Truncated = r.ReadBool()
	if !ci.Truncated {
		ci.argumentsBytes = r.ReadVarBytes()
	}
}

// EncodeBinary implements the Serializable interface.
func (ci *ContractInvocation) EncodeBinary(w *io.BinWriter) {
	ci.EncodeBinaryWithContext(w, stackitem.NewSerializationContext())
}

// EncodeBinaryWithContext is the same as EncodeBinary, but allows to efficiently reuse
// stack item serialization context.
func (ci *ContractInvocation) EncodeBinaryWithContext(w *io.BinWriter, sc *stackitem.SerializationContext) {
	ci.Hash.EncodeBinary(w)
	w.WriteString(ci.Method)
	w.WriteU32LE(ci.ArgumentsCount)
	w.WriteBool(ci.Truncated)
	if !ci.Truncated {
		w.WriteVarBytes(ci.argumentsBytes)
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (ci ContractInvocation) MarshalJSON() ([]byte, error) {
	var item []byte
	if ci.Arguments == nil && ci.argumentsBytes != nil {
		si, err := stackitem.Deserialize(ci.argumentsBytes)
		if err != nil {
			return nil, err
		}
		item, err = stackitem.ToJSONWithTypes(si.(*stackitem.Array))
		if err != nil {
			item = nil
		}
	}
	return json.Marshal(contractInvocationAux{
		Hash:           ci.Hash,
		Method:         ci.Method,
		Arguments:      item,
		ArgumentsCount: ci.ArgumentsCount,
		Truncated:      ci.Truncated,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (ci *ContractInvocation) UnmarshalJSON(data []byte) error {
	aux := new(contractInvocationAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	var args *stackitem.Array
	if aux.Arguments != nil {
		arguments, err := stackitem.FromJSONWithTypes(aux.Arguments)
		if err != nil {
			return err
		}
		if t := arguments.Type(); t != stackitem.ArrayT {
			return fmt.Errorf("failed to convert invocation state of type %s to array", t.String())
		}
		args = arguments.(*stackitem.Array)
	}
	ci.Method = aux.Method
	ci.Hash = aux.Hash
	ci.ArgumentsCount = aux.ArgumentsCount
	ci.Truncated = aux.Truncated
	ci.Arguments = args
	return nil
}
