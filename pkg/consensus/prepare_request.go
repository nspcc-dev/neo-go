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
	version   uint32
	prevHash  util.Uint256
	timestamp uint64
	nonce     uint64

	// extended denotes whether prepareRequest holds the whole set of transactions
	// instead of the hashes only.
	extended bool
	// transactionHashes is used for pre-Huyao behaviour.
	transactionHashes []util.Uint256
	// transactions is used for post-Huyao behaviour.
	transactions []*transaction.Transaction

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
	if p.extended {
		w.WriteArray(p.transactions)
	} else {
		w.WriteVarUint(uint64(len(p.transactionHashes)))
		for i := range p.transactionHashes {
			w.WriteBytes(p.transactionHashes[i][:])
		}
	}
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
	if p.extended {
		r.ReadArray(&p.transactions, block.MaxTransactionsPerBlock)
	} else {
		r.ReadArray(&p.transactionHashes, block.MaxTransactionsPerBlock)
	}
	if p.stateRootEnabled {
		r.ReadBytes(p.stateRoot[:])
	}
}

// Timestamp implements the payload.PrepareRequest interface.
func (p *prepareRequest) Timestamp() uint64 { return p.timestamp * nsInMs }

// Nonce implements the payload.PrepareRequest interface.
func (p *prepareRequest) Nonce() uint64 { return p.nonce }

// TransactionHashes implements the payload.PrepareRequest interface.
func (p *prepareRequest) TransactionHashes() []util.Uint256 { return p.transactionHashes }

// Transactions implements the payload.PrepareRequest interface.
func (p *prepareRequest) Transactions() []dbft.Transaction[util.Uint256] {
	txx := make([]dbft.Transaction[util.Uint256], len(p.transactions))
	for i, tx := range p.transactions {
		txx[i] = tx
	}
	return txx
}

func (p *prepareRequest) txHashes() []util.Uint256 {
	if !p.extended {
		return p.transactionHashes
	}
	hashes := make([]util.Uint256, len(p.transactions))
	for i, tx := range p.transactions {
		hashes[i] = tx.Hash()
	}
	return hashes
}
