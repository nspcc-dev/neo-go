package consensus

import (
	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// prepareRequest represents dBFT prepareRequest message.
type prepareRequest struct {
	version           uint32
	prevHash          util.Uint256
	timestamp         uint64
	nonce             uint64
	transactionHashes []util.Uint256
	stateRootEnabled  bool
	stateRoot         util.Uint256
}

var _ dbft.PrepareRequest[util.Uint256] = (*prepareRequest)(nil)

// EncodeBinary implements the io.Serializable interface.
func (p *prepareRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(p.version)
	w.WriteBytes(p.prevHash[:])
	w.WriteU64LE(p.timestamp)
	w.WriteU64LE(p.nonce)
	w.WriteVarUint(uint64(len(p.transactionHashes)))
	for i := range p.transactionHashes {
		w.WriteBytes(p.transactionHashes[i][:])
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
	r.ReadArray(&p.transactionHashes, block.MaxTransactionsPerBlock)
	if p.stateRootEnabled {
		r.ReadBytes(p.stateRoot[:])
	}
}

// Version implements the payload.PrepareRequest interface.
func (p prepareRequest) Version() uint32 {
	return p.version
}

// SetVersion implements the payload.PrepareRequest interface.
func (p *prepareRequest) SetVersion(v uint32) {
	p.version = v
}

// PrevHash implements the payload.PrepareRequest interface.
func (p prepareRequest) PrevHash() util.Uint256 {
	return p.prevHash
}

// SetPrevHash implements the payload.PrepareRequest interface.
func (p *prepareRequest) SetPrevHash(h util.Uint256) {
	p.prevHash = h
}

// Timestamp implements the payload.PrepareRequest interface.
func (p *prepareRequest) Timestamp() uint64 { return p.timestamp * nsInMs }

// SetTimestamp implements the payload.PrepareRequest interface.
func (p *prepareRequest) SetTimestamp(ts uint64) { p.timestamp = ts / nsInMs }

// Nonce implements the payload.PrepareRequest interface.
func (p *prepareRequest) Nonce() uint64 { return p.nonce }

// SetNonce implements the payload.PrepareRequest interface.
func (p *prepareRequest) SetNonce(nonce uint64) { p.nonce = nonce }

// TransactionHashes implements the payload.PrepareRequest interface.
func (p *prepareRequest) TransactionHashes() []util.Uint256 { return p.transactionHashes }

// SetTransactionHashes implements the payload.PrepareRequest interface.
func (p *prepareRequest) SetTransactionHashes(hs []util.Uint256) { p.transactionHashes = hs }

// NextConsensus implements the payload.PrepareRequest interface.
func (p *prepareRequest) NextConsensus() util.Uint160 { return util.Uint160{} }

// SetNextConsensus implements the payload.PrepareRequest interface.
func (p *prepareRequest) SetNextConsensus(_ util.Uint160) {}
