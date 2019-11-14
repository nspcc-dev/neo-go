package consensus

import (
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// prepareRequest represents dBFT PrepareRequest message.
type prepareRequest struct {
	Timestamp         uint32
	Nonce             uint64
	TransactionHashes []util.Uint256
	MinerTransaction  transaction.Transaction
	NextConsensus     util.Uint160
}

// EncodeBinary implements io.Serializable interface.
func (p *prepareRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteLE(p.Timestamp)
	w.WriteLE(p.Nonce)
	w.WriteBE(p.NextConsensus[:])
	w.WriteArray(p.TransactionHashes)
	p.MinerTransaction.EncodeBinary(w)
}

// DecodeBinary implements io.Serializable interface.
func (p *prepareRequest) DecodeBinary(r *io.BinReader) {
	r.ReadLE(&p.Timestamp)
	r.ReadLE(&p.Nonce)
	r.ReadBE(p.NextConsensus[:])
	r.ReadArray(&p.TransactionHashes)
	p.MinerTransaction.DecodeBinary(r)
}
