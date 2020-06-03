package state

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// NotificationEvent is a tuple of scripthash that emitted the Item as a
// notification and that item itself.
type NotificationEvent struct {
	ScriptHash util.Uint160
	Item       stackitem.Item
}

// AppExecResult represent the result of the script execution, gathering together
// all resulting notifications, state, stack and other metadata.
type AppExecResult struct {
	TxHash      util.Uint256
	Trigger     trigger.Type
	VMState     string
	GasConsumed util.Fixed8
	Stack       []smartcontract.Parameter
	Events      []NotificationEvent
}

// EncodeBinary implements the Serializable interface.
func (ne *NotificationEvent) EncodeBinary(w *io.BinWriter) {
	ne.ScriptHash.EncodeBinary(w)
	stackitem.EncodeBinaryStackItem(ne.Item, w)
}

// DecodeBinary implements the Serializable interface.
func (ne *NotificationEvent) DecodeBinary(r *io.BinReader) {
	ne.ScriptHash.DecodeBinary(r)
	ne.Item = stackitem.DecodeBinaryStackItem(r)
}

// EncodeBinary implements the Serializable interface.
func (aer *AppExecResult) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(aer.TxHash[:])
	w.WriteB(byte(aer.Trigger))
	w.WriteString(aer.VMState)
	aer.GasConsumed.EncodeBinary(w)
	w.WriteArray(aer.Stack)
	w.WriteArray(aer.Events)
}

// DecodeBinary implements the Serializable interface.
func (aer *AppExecResult) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(aer.TxHash[:])
	aer.Trigger = trigger.Type(r.ReadB())
	aer.VMState = r.ReadString()
	aer.GasConsumed.DecodeBinary(r)
	r.ReadArray(&aer.Stack)
	r.ReadArray(&aer.Events)
}
