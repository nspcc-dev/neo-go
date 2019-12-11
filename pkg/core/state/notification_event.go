package state

import (
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
)

// NotificationEvent is a tuple of scripthash that emitted the StackItem as a
// notification and that item itself.
type NotificationEvent struct {
	ScriptHash util.Uint160
	Item       vm.StackItem
}

// AppExecResult represent the result of the script execution, gathering together
// all resulting notifications, state, stack and other metadata.
type AppExecResult struct {
	TxHash      util.Uint256
	Trigger     byte
	VMState     string
	GasConsumed util.Fixed8
	Stack       string // JSON
	Events      []NotificationEvent
}

// EncodeBinary implements the Serializable interface.
func (ne *NotificationEvent) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(ne.ScriptHash[:])
	vm.EncodeBinaryStackItem(ne.Item, w)
}

// DecodeBinary implements the Serializable interface.
func (ne *NotificationEvent) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(ne.ScriptHash[:])
	ne.Item = vm.DecodeBinaryStackItem(r)
}

// EncodeBinary implements the Serializable interface.
func (aer *AppExecResult) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(aer.TxHash[:])
	w.WriteLE(aer.Trigger)
	w.WriteString(aer.VMState)
	w.WriteLE(aer.GasConsumed)
	w.WriteString(aer.Stack)
	w.WriteArray(aer.Events)
}

// DecodeBinary implements the Serializable interface.
func (aer *AppExecResult) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(aer.TxHash[:])
	r.ReadLE(&aer.Trigger)
	aer.VMState = r.ReadString()
	r.ReadLE(&aer.GasConsumed)
	aer.Stack = r.ReadString()
	r.ReadArray(&aer.Events)
}
