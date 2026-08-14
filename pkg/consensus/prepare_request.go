package consensus

import (
	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// prepareRequest represents dBFT prepareRequest message.
type prepareRequest struct {
	version          uint32
	prevHash         util.Uint256
	timestamp        uint64
	nonce            uint64
	transactions     []*transaction.Transaction
	stateRootEnabled bool
	stateRoot        util.Uint256
}

var _ dbft.PrepareRequest[util.Uint256] = (*prepareRequest)(nil)

// EncodeBinary implements the io.Serializable interface.
func (p *prepareRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(p.version)
	w.WriteBytes(p.prevHash[:])
	w.WriteU64LE(p.timestamp)
	w.WriteU64LE(p.nonce)
	w.WriteArray(p.transactions)
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
	r.ReadArray(&p.transactions, block.MaxTransactionsPerBlock)
	if p.stateRootEnabled {
		r.ReadBytes(p.stateRoot[:])
	}
}

// Timestamp implements the payload.PrepareRequest interface.
func (p *prepareRequest) Timestamp() uint64 { return p.timestamp * nsInMs }

// Nonce implements the payload.PrepareRequest interface.
func (p *prepareRequest) Nonce() uint64 { return p.nonce }

// TransactionHashes implements the payload.PrepareRequest interface.
func (p *prepareRequest) TransactionHashes() []util.Uint256 { return nil }

// Transactions implements the payload.PrepareRequest interface.
func (p *prepareRequest) Transactions() []dbft.Transaction[util.Uint256] {
	// TODO: avoid conversion (and allocation), return directly p.transactions
	txx := make([]dbft.Transaction[util.Uint256], len(p.transactions))
	for i, tx := range p.transactions {
		txx[i] = tx
	}
	return txx
}
