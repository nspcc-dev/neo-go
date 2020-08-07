package state

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// TransferSize is a size of a marshaled Transfer struct in bytes.
const TransferSize = 2 + 8 + 4 + 4 + util.Uint256Size

// Transfer represents a single  Transfer event.
type Transfer struct {
	// IsGoverning is true iff transfer is for neo token.
	IsGoverning bool
	// IsSent is true iff UTXO used in the input.
	IsSent bool
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
	w.WriteBytes(t.Tx[:])
	w.WriteU32LE(t.Block)
	w.WriteU32LE(t.Timestamp)
	w.WriteU64LE(uint64(t.Amount))
	w.WriteBool(t.IsGoverning)
	w.WriteBool(t.IsSent)
}

// DecodeBinary implements io.Serializable interface.
func (t *Transfer) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(t.Tx[:])
	t.Block = r.ReadU32LE()
	t.Timestamp = r.ReadU32LE()
	t.Amount = int64(r.ReadU64LE())
	t.IsGoverning = r.ReadBool()
	t.IsSent = r.ReadBool()
}
