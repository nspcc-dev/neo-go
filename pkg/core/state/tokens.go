package state

import (
	"bytes"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/config/limits"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// TokenTransferBatchSize is the maximum number of entries for TokenTransferLog.
const TokenTransferBatchSize = 128

// TokenTransferLog is a serialized log of token transfers.
type TokenTransferLog struct {
	Raw []byte
	buf *bytes.Buffer
	iow *io.BinWriter
}

// NEP17Transfer represents a single NEP-17 Transfer event.
type NEP17Transfer struct {
	// Asset is a NEP-17 contract ID.
	Asset int32
	// Counterparty is the address of the sender/receiver (the other side of the transfer).
	Counterparty util.Uint160
	// Amount is the amount of tokens transferred.
	// It is negative when tokens are sent and positive if they are received.
	Amount *big.Int
	// Block is a number of block when the event occurred.
	Block uint32
	// Timestamp is the timestamp of the block where transfer occurred.
	Timestamp uint64
	// Tx is a hash the transaction.
	Tx util.Uint256
}

// NEP11Transfer represents a single NEP-11 Transfer event.
type NEP11Transfer struct {
	NEP17Transfer

	// ID is a NEP-11 token ID.
	ID []byte
}

// TokenTransferInfo stores a map of the contract IDs to the balance's last updated
// block trackers along with the information about NEP-17 and NEP-11 transfer batch.
type TokenTransferInfo struct {
	LastUpdated map[int32]uint32
	// NextNEP11Batch stores the index of the next NEP-11 transfer batch.
	NextNEP11Batch uint32
	// NextNEP17Batch stores the index of the next NEP-17 transfer batch.
	NextNEP17Batch uint32
	// NextNEP11NewestTimestamp stores the block timestamp of the first NEP-11 transfer in raw.
	NextNEP11NewestTimestamp uint64
	// NextNEP17NewestTimestamp stores the block timestamp of the first NEP-17 transfer in raw.
	NextNEP17NewestTimestamp uint64
	// NewNEP11Batch is true if batch with the `NextNEP11Batch` index should be created.
	NewNEP11Batch bool
	// NewNEP17Batch is true if batch with the `NextNEP17Batch` index should be created.
	NewNEP17Batch bool
}

// NewTokenTransferInfo returns new TokenTransferInfo.
func NewTokenTransferInfo() *TokenTransferInfo {
	return &TokenTransferInfo{
		NewNEP11Batch: true,
		NewNEP17Batch: true,
		LastUpdated:   make(map[int32]uint32),
	}
}

// DecodeBinary implements the io.Serializable interface.
func (bs *TokenTransferInfo) DecodeBinary(r *io.BinReader) {
	bs.NextNEP11Batch = r.ReadU32LE()
	bs.NextNEP17Batch = r.ReadU32LE()
	bs.NextNEP11NewestTimestamp = r.ReadU64LE()
	bs.NextNEP17NewestTimestamp = r.ReadU64LE()
	bs.NewNEP11Batch = r.ReadBool()
	bs.NewNEP17Batch = r.ReadBool()
	lenBalances := r.ReadVarUint()
	m := make(map[int32]uint32, lenBalances)
	for range lenBalances {
		key := int32(r.ReadU32LE())
		m[key] = r.ReadU32LE()
	}
	bs.LastUpdated = m
}

// EncodeBinary implements the io.Serializable interface.
func (bs *TokenTransferInfo) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(bs.NextNEP11Batch)
	w.WriteU32LE(bs.NextNEP17Batch)
	w.WriteU64LE(bs.NextNEP11NewestTimestamp)
	w.WriteU64LE(bs.NextNEP17NewestTimestamp)
	w.WriteBool(bs.NewNEP11Batch)
	w.WriteBool(bs.NewNEP17Batch)
	w.WriteVarUint(uint64(len(bs.LastUpdated)))
	for k, v := range bs.LastUpdated {
		w.WriteU32LE(uint32(k))
		w.WriteU32LE(v)
	}
}

// Append appends a single transfer to a log.
func (lg *TokenTransferLog) Append(tr io.Serializable) error {
	// The first entry, set up counter.
	if len(lg.Raw) == 0 {
		lg.Raw = append(lg.Raw, 0)
	}

	if lg.buf == nil {
		lg.buf = bytes.NewBuffer(lg.Raw)
	}
	if lg.iow == nil {
		lg.iow = io.NewBinWriterFromIO(lg.buf)
	}

	tr.EncodeBinary(lg.iow)
	if lg.iow.Err != nil {
		return lg.iow.Err
	}
	lg.Raw = lg.buf.Bytes()
	lg.Raw[0]++
	return nil
}

// Reset resets the state of the log, clearing all entries, but keeping existing
// buffer for future writes.
func (lg *TokenTransferLog) Reset() {
	lg.Raw = lg.Raw[:0]
	lg.buf = nil
	lg.iow = nil
}

// ForEachNEP11 iterates over a transfer log returning on the first error.
func (lg *TokenTransferLog) ForEachNEP11(f func(*NEP11Transfer) (bool, error)) (bool, error) {
	if lg == nil || len(lg.Raw) == 0 {
		return true, nil
	}
	transfers := make([]NEP11Transfer, lg.Size())
	r := io.NewBinReaderFromBuf(lg.Raw[1:])
	for i := range transfers {
		transfers[i].DecodeBinary(r)
	}
	if r.Err != nil {
		return false, r.Err
	}
	for i := len(transfers) - 1; i >= 0; i-- {
		cont, err := f(&transfers[i])
		if err != nil || !cont {
			return false, err
		}
	}
	return true, nil
}

// ForEachNEP17 iterates over a transfer log returning on the first error.
func (lg *TokenTransferLog) ForEachNEP17(f func(*NEP17Transfer) (bool, error)) (bool, error) {
	if lg == nil || len(lg.Raw) == 0 {
		return true, nil
	}
	transfers := make([]NEP17Transfer, lg.Size())
	r := io.NewBinReaderFromBuf(lg.Raw[1:])
	for i := range transfers {
		transfers[i].DecodeBinary(r)
	}
	if r.Err != nil {
		return false, r.Err
	}
	for i := len(transfers) - 1; i >= 0; i-- {
		cont, err := f(&transfers[i])
		if err != nil || !cont {
			return false, err
		}
	}
	return true, nil
}

// Size returns the amount of the transfer written in the log.
func (lg *TokenTransferLog) Size() int {
	if len(lg.Raw) == 0 {
		return 0
	}
	return int(lg.Raw[0])
}

// EncodeBinary implements the io.Serializable interface.
func (t *NEP17Transfer) EncodeBinary(w *io.BinWriter) {
	var buf [bigint.MaxBytesLen]byte

	w.WriteU32LE(uint32(t.Asset))
	w.WriteBytes(t.Tx[:])
	w.WriteBytes(t.Counterparty[:])
	w.WriteU32LE(t.Block)
	w.WriteU64LE(t.Timestamp)
	amount := bigint.ToPreallocatedBytes(t.Amount, buf[:])
	w.WriteVarBytes(amount)
}

// DecodeBinary implements the io.Serializable interface.
func (t *NEP17Transfer) DecodeBinary(r *io.BinReader) {
	t.Asset = int32(r.ReadU32LE())
	r.ReadBytes(t.Tx[:])
	r.ReadBytes(t.Counterparty[:])
	t.Block = r.ReadU32LE()
	t.Timestamp = r.ReadU64LE()
	amount := r.ReadVarBytes(bigint.MaxBytesLen)
	t.Amount = bigint.FromBytes(amount)
}

// EncodeBinary implements the io.Serializable interface.
func (t *NEP11Transfer) EncodeBinary(w *io.BinWriter) {
	t.NEP17Transfer.EncodeBinary(w)
	w.WriteVarBytes(t.ID)
}

// DecodeBinary implements the io.Serializable interface.
func (t *NEP11Transfer) DecodeBinary(r *io.BinReader) {
	t.NEP17Transfer.DecodeBinary(r)
	t.ID = r.ReadVarBytes(limits.MaxStorageKeyLen)
}
