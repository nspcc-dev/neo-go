package state

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NEP5Tracker contains info about a single account in a NEP5 contract.
type NEP5Tracker struct {
	// Balance is the current balance of the account.
	Balance int64
	// LastUpdatedBlock is a number of block when last `transfer` to or from the
	// account occured.
	LastUpdatedBlock uint32
}

// NEP5TransferLog is a log of NEP5 token transfers for the specific command.
type NEP5TransferLog struct {
	Raw []byte
}

// NEP5TransferSize is a size of a marshaled NEP5Transfer struct in bytes.
const NEP5TransferSize = util.Uint160Size*3 + 8 + 4 + 4 + util.Uint256Size

// NEP5Transfer represents a single NEP5 Transfer event.
type NEP5Transfer struct {
	// Asset is a NEP5 contract hash.
	Asset util.Uint160
	// Address is the address of the sender.
	From util.Uint160
	// To is the address of the receiver.
	To util.Uint160
	// Amount is the amount of tokens transferred.
	// It is negative when tokens are sent and positive if they are received.
	Amount int64
	// Block is a number of block when the event occured.
	Block uint32
	// Timestamp is the timestamp of the block where transfer occured.
	Timestamp uint32
	// Tx is a hash the transaction.
	Tx util.Uint256
}

// Append appends single transfer to a log.
func (lg *NEP5TransferLog) Append(tr *NEP5Transfer) error {
	w := io.NewBufBinWriter()
	tr.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		return w.Err
	}
	lg.Raw = append(lg.Raw, w.Bytes()...)
	return nil
}

// EncodeBinary implements io.Serializable interface.
func (t *NEP5Tracker) EncodeBinary(w *io.BinWriter) {
	w.WriteU64LE(uint64(t.Balance))
	w.WriteU32LE(t.LastUpdatedBlock)
}

// DecodeBinary implements io.Serializable interface.
func (t *NEP5Tracker) DecodeBinary(r *io.BinReader) {
	t.Balance = int64(r.ReadU64LE())
	t.LastUpdatedBlock = r.ReadU32LE()
}

// EncodeBinary implements io.Serializable interface.
// Note: change NEP5TransferSize constant when changing this function.
func (t *NEP5Transfer) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(t.Asset[:])
	w.WriteBytes(t.Tx[:])
	w.WriteBytes(t.From[:])
	w.WriteBytes(t.To[:])
	w.WriteU32LE(t.Block)
	w.WriteU32LE(t.Timestamp)
	w.WriteU64LE(uint64(t.Amount))
}

// DecodeBinary implements io.Serializable interface.
func (t *NEP5Transfer) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(t.Asset[:])
	r.ReadBytes(t.Tx[:])
	r.ReadBytes(t.From[:])
	r.ReadBytes(t.To[:])
	t.Block = r.ReadU32LE()
	t.Timestamp = r.ReadU32LE()
	t.Amount = int64(r.ReadU64LE())
}
