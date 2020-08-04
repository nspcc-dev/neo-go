package state

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// NotificationEvent is a tuple of scripthash that emitted the Item as a
// notification and that item itself.
type NotificationEvent struct {
	ScriptHash util.Uint160
	Name       string
	Item       *stackitem.Array
}

// AppExecResult represent the result of the script execution, gathering together
// all resulting notifications, state, stack and other metadata.
type AppExecResult struct {
	TxHash      util.Uint256
	Trigger     trigger.Type
	VMState     vm.State
	GasConsumed int64
	Stack       []stackitem.Item
	Events      []NotificationEvent
}

// EncodeBinary implements the Serializable interface.
func (ne *NotificationEvent) EncodeBinary(w *io.BinWriter) {
	ne.ScriptHash.EncodeBinary(w)
	w.WriteString(ne.Name)
	stackitem.EncodeBinaryStackItem(ne.Item, w)
}

// DecodeBinary implements the Serializable interface.
func (ne *NotificationEvent) DecodeBinary(r *io.BinReader) {
	ne.ScriptHash.DecodeBinary(r)
	ne.Name = r.ReadString()
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err != nil {
		return
	}
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		r.Err = errors.New("Array or Struct expected")
		return
	}
	ne.Item = stackitem.NewArray(arr)
}

// EncodeBinary implements the Serializable interface.
func (aer *AppExecResult) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(aer.TxHash[:])
	w.WriteB(byte(aer.Trigger))
	w.WriteB(byte(aer.VMState))
	w.WriteU64LE(uint64(aer.GasConsumed))
	stackitem.EncodeBinaryStackItem(stackitem.NewArray(aer.Stack), w)
	w.WriteArray(aer.Events)
}

// DecodeBinary implements the Serializable interface.
func (aer *AppExecResult) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(aer.TxHash[:])
	aer.Trigger = trigger.Type(r.ReadB())
	aer.VMState = vm.State(r.ReadB())
	aer.GasConsumed = int64(r.ReadU64LE())
	item := stackitem.DecodeBinaryStackItem(r)
	if r.Err == nil {
		arr, ok := item.Value().([]stackitem.Item)
		if !ok {
			r.Err = errors.New("array expected")
			return
		}
		aer.Stack = arr
	}
	r.ReadArray(&aer.Events)
}
