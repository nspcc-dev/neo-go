package consensus

import (
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/nspcc-dev/dbft/payload"
)

// prepareRequest represents dBFT prepareRequest message.
type prepareRequest struct {
	timestamp         uint32
	nonce             uint64
	transactionHashes []util.Uint256
	minerTx           transaction.Transaction
	nextConsensus     util.Uint160
}

var _ payload.PrepareRequest = (*prepareRequest)(nil)

// EncodeBinary implements io.Serializable interface.
func (p *prepareRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(p.timestamp)
	w.WriteU64LE(p.nonce)
	w.WriteBytes(p.nextConsensus[:])
	w.WriteArray(p.transactionHashes)
	p.minerTx.EncodeBinary(w)
}

// DecodeBinary implements io.Serializable interface.
func (p *prepareRequest) DecodeBinary(r *io.BinReader) {
	p.timestamp = r.ReadU32LE()
	p.nonce = r.ReadU64LE()
	r.ReadBytes(p.nextConsensus[:])
	r.ReadArray(&p.transactionHashes)
	p.minerTx.DecodeBinary(r)
}

// Timestamp implements payload.PrepareRequest interface.
func (p *prepareRequest) Timestamp() uint32 { return p.timestamp }

// SetTimestamp implements payload.PrepareRequest interface.
func (p *prepareRequest) SetTimestamp(ts uint32) { p.timestamp = ts }

// Nonce implements payload.PrepareRequest interface.
func (p *prepareRequest) Nonce() uint64 { return p.nonce }

// SetNonce implements payload.PrepareRequest interface.
func (p *prepareRequest) SetNonce(nonce uint64) { p.nonce = nonce }

// TransactionHashes implements payload.PrepareRequest interface.
func (p *prepareRequest) TransactionHashes() []util.Uint256 { return p.transactionHashes }

// SetTransactionHashes implements payload.PrepareRequest interface.
func (p *prepareRequest) SetTransactionHashes(hs []util.Uint256) { p.transactionHashes = hs }

// NextConsensus implements payload.PrepareRequest interface.
func (p *prepareRequest) NextConsensus() util.Uint160 { return p.nextConsensus }

// SetNextConsensus implements payload.PrepareRequest interface.
func (p *prepareRequest) SetNextConsensus(nc util.Uint160) { p.nextConsensus = nc }
