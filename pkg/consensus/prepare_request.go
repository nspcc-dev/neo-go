package consensus

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

var errTransactionMismatch = errors.New("decoded transaction hash does not match the proposed one")

// prepareRequest represents dBFT prepareRequest message.
type prepareRequest struct {
	version          uint32
	prevHash         util.Uint256
	timestamp        uint64
	nonce            uint64
	stateRootEnabled bool
	stateRoot        util.Uint256

	transactionHashes []util.Uint256

	transactionsData    []byte
	transactionsOffsets []uint64

	transactions []*transaction.Transaction
}

var _ dbft.PrepareRequest[util.Uint256] = (*prepareRequest)(nil)

// EncodeBinary implements the io.Serializable interface.
func (p *prepareRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(p.version)
	w.WriteBytes(p.prevHash[:])
	w.WriteU64LE(p.timestamp)
	w.WriteU64LE(p.nonce)

	if p.transactionHashes == nil {
		hashes := make([]util.Uint256, len(p.transactions))
		offsets := make([]uint64, len(p.transactions))
		data := io.NewBufBinWriter()
		for i, tx := range p.transactions {
			hashes[i] = tx.Hash()
			offsets[i] = uint64(data.Len())
			tx.EncodeBinary(data.BinWriter)
		}
		if data.Err != nil {
			w.Err = data.Err
			return
		}
		p.transactionHashes = hashes
		p.transactionsOffsets = offsets
		p.transactionsData = data.Bytes()
	}

	w.WriteVarUint(uint64(len(p.transactionHashes)))
	for i, h := range p.transactionHashes {
		w.WriteBytes(h.BytesBE())
		w.WriteVarUint(p.transactionsOffsets[i])
	}
	w.WriteVarBytes(p.transactionsData)

	if p.stateRootEnabled {
		w.WriteBytes(p.stateRoot[:])
	}
}

// DecodeBinary implements the io.Serializable interface.
func (p *prepareRequest) DecodeBinary(r *io.BinReader) {
	p.version = r.ReadU32LE()
	r.ReadBytes(p.prevHash[:])
	p.timestamp = r.ReadU64LE()
	p.nonce = r.ReadU64LE()

	n := r.ReadVarUint()
	if n > uint64(block.MaxTransactionsPerBlock) {
		r.Err = fmt.Errorf("%w: %d", errInvalidTransactionsCount, n)
		return
	}

	hashes := make([]util.Uint256, n)
	offsets := make([]uint64, n)
	for i := range hashes {
		r.ReadBytes(hashes[i][:])
		offsets[i] = r.ReadVarUint()
	}

	data := r.ReadVarBytes()

	dataLen := uint64(len(data))
	for i, off := range offsets {
		if off > dataLen {
			r.Err = fmt.Errorf("invalid transaction offset at index %d", i)
			return
		}
	}

	p.transactionHashes = hashes
	p.transactionsData = data
	p.transactionsOffsets = offsets

	if p.stateRootEnabled {
		r.ReadBytes(p.stateRoot[:])
	}
}

// Timestamp implements the payload.PrepareRequest interface.
func (p *prepareRequest) Timestamp() uint64 { return p.timestamp * nsInMs }

// Nonce implements the payload.PrepareRequest interface.
func (p *prepareRequest) Nonce() uint64 { return p.nonce }

// TransactionHashes implements the payload.PrepareRequest interface.
func (p *prepareRequest) TransactionHashes() []util.Uint256 {
	return p.transactionHashes
}
