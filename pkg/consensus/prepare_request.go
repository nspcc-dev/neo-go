package consensus

import (
	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// prepareRequest represents dBFT prepareRequest message.
type prepareRequest struct {
	// TODO: extend prepare request verification code that hsould check that version is properly set
	version           uint32
	prevHash          util.Uint256
	timestamp         uint64
	nonce             uint64
	transactionHashes []util.Uint256
	stateRoot         util.Uint256
}

var _ dbft.PrepareRequest[util.Uint256] = (*prepareRequest)(nil)

// EncodeBinary implements the io.Serializable interface.
func (p *prepareRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(p.version)
	w.WriteBytes(p.prevHash[:])
	w.WriteU64LE(p.timestamp)
	w.WriteU64LE(p.nonce)
	if p.version == block.VersionFaun {
		w.WriteBytes(p.stateRoot[:])
	}
	w.WriteVarUint(uint64(len(p.transactionHashes)))
	for i := range p.transactionHashes {
		w.WriteBytes(p.transactionHashes[i][:])
	}
}

// DecodeBinary implements the io.Serializable interface.
func (p *prepareRequest) DecodeBinary(r *io.BinReader) {
	p.version = r.ReadU32LE()
	r.ReadBytes(p.prevHash[:])
	p.timestamp = r.ReadU64LE()
	p.nonce = r.ReadU64LE()
	if p.version == block.VersionFaun {
		r.ReadBytes(p.stateRoot[:])
	}
	r.ReadArray(&p.transactionHashes, block.MaxTransactionsPerBlock)
}

// Timestamp implements the payload.PrepareRequest interface.
func (p *prepareRequest) Timestamp() uint64 { return p.timestamp * nsInMs }

// Nonce implements the payload.PrepareRequest interface.
func (p *prepareRequest) Nonce() uint64 { return p.nonce }

// TransactionHashes implements the payload.PrepareRequest interface.
func (p *prepareRequest) TransactionHashes() []util.Uint256 { return p.transactionHashes }
