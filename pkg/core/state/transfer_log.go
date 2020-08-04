package state

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// TransferSize is a size of a marshaled Transfer struct in bytes.
const TransferSize = util.Uint160Size*2 + 8 + 4 + 4 + util.Uint256Size*2

// Transfer represents a single  Transfer event.
type Transfer struct {
	// AssetID is the asset related to transfer.
	AssetID util.Uint256
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

// EncodeBinary implements io.Serializable interface.
// Note: change TransferSize constant when changing this function.
func (t *Transfer) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(t.AssetID[:])
	w.WriteBytes(t.Tx[:])
	w.WriteBytes(t.From[:])
	w.WriteBytes(t.To[:])
	w.WriteU32LE(t.Block)
	w.WriteU32LE(t.Timestamp)
	w.WriteU64LE(uint64(t.Amount))
}

// DecodeBinary implements io.Serializable interface.
func (t *Transfer) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(t.AssetID[:])
	r.ReadBytes(t.Tx[:])
	r.ReadBytes(t.From[:])
	r.ReadBytes(t.To[:])
	t.Block = r.ReadU32LE()
	t.Timestamp = r.ReadU32LE()
	t.Amount = int64(r.ReadU64LE())
}
