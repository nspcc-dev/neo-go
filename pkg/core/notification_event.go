package core

import (
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/pkg/errors"
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

// putAppExecResultIntoStore puts given application execution result into the
// given store.
func putAppExecResultIntoStore(s storage.Store, aer *AppExecResult) error {
	buf := io.NewBufBinWriter()
	aer.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	key := storage.AppendPrefix(storage.STNotification, aer.TxHash.BytesBE())
	return s.Put(key, buf.Bytes())
}

// getAppExecResultFromStore gets application execution result from the
// given store.
func getAppExecResultFromStore(s storage.Store, hash util.Uint256) (*AppExecResult, error) {
	aer := &AppExecResult{}
	key := storage.AppendPrefix(storage.STNotification, hash.BytesBE())
	if b, err := s.Get(key); err == nil {
		r := io.NewBinReaderFromBuf(b)
		aer.DecodeBinary(r)
		if r.Err != nil {
			return nil, errors.Wrap(r.Err, "decoding failure:")
		}
	} else {
		return nil, err
	}
	return aer, nil
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
	w.WriteArray(aer.Events)
}

// DecodeBinary implements the Serializable interface.
func (aer *AppExecResult) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(aer.TxHash[:])
	r.ReadArray(&aer.Events)
}
