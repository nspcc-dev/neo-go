package state

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// NEP5Tracker contains info about a single account in a NEP5 contract.
type NEP5Tracker struct {
	// Balance is the current balance of the account.
	Balance int64
	// LastUpdatedBlock is a number of block when last `transfer` to or from the
	// account occured.
	LastUpdatedBlock uint32
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
