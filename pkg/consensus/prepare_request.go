package consensus

import (
	"github.com/nspcc-dev/dbft/payload"
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

var _ payload.PrepareRequest = (*prepareRequest)(nil)

// EncodeBinary implements io.Serializable interface.
func (p *prepareRequest) EncodeBinary(w io.BinaryWriter) {
	w.WriteU32LE(p.version)
	w.WriteBytes(p.prevHash[:])
	w.WriteU64LE(p.timestamp)
	w.WriteU64LE(p.nonce)
	w.WriteArray(p.transactionHashes)
	if p.stateRootEnabled {
		w.WriteBytes(p.stateRoot[:])
	}
}

// DecodeBinary implements io.Serializable interface.
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

// Version implements payload.PrepareRequest interface.
func (p prepareRequest) Version() uint32 {
	return p.version
}

// SetVersion implements payload.PrepareRequest interface.
func (p *prepareRequest) SetVersion(v uint32) {
	p.version = v
}

// PrevHash implements payload.PrepareRequest interface.
func (p prepareRequest) PrevHash() util.Uint256 {
	return p.prevHash
}

// SetPrevHash implements payload.PrepareRequest interface.
func (p *prepareRequest) SetPrevHash(h util.Uint256) {
	p.prevHash = h
}

// Timestamp implements payload.PrepareRequest interface.
func (p *prepareRequest) Timestamp() uint64 { return p.timestamp * nsInMs }

// SetTimestamp implements payload.PrepareRequest interface.
func (p *prepareRequest) SetTimestamp(ts uint64) { p.timestamp = ts / nsInMs }

// Nonce implements payload.PrepareRequest interface.
func (p *prepareRequest) Nonce() uint64 { return p.nonce }

// SetNonce implements payload.PrepareRequest interface.
func (p *prepareRequest) SetNonce(nonce uint64) { p.nonce = nonce }

// TransactionHashes implements payload.PrepareRequest interface.
func (p *prepareRequest) TransactionHashes() []util.Uint256 { return p.transactionHashes }

// SetTransactionHashes implements payload.PrepareRequest interface.
func (p *prepareRequest) SetTransactionHashes(hs []util.Uint256) { p.transactionHashes = hs }

// NextConsensus implements payload.PrepareRequest interface.
func (p *prepareRequest) NextConsensus() util.Uint160 { return util.Uint160{} }

// SetNextConsensus implements payload.PrepareRequest interface.
func (p *prepareRequest) SetNextConsensus(_ util.Uint160) {}
