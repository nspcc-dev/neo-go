package consensus

import (
	"github.com/nspcc-dev/dbft/block"
	"github.com/nspcc-dev/dbft/crypto"
	coreb "github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// neoBlock is a wrapper of core.Block which implements
// methods necessary for dBFT library.
type neoBlock struct {
	coreb.Block

	signature []byte
}

var _ block.Block = (*neoBlock)(nil)

// Sign implements block.Block interface.
func (n *neoBlock) Sign(key crypto.PrivateKey) error {
	data := n.Base.GetSignedPart()
	sig, err := key.Sign(data[:])
	if err != nil {
		return err
	}

	n.signature = sig

	return nil
}

// Verify implements block.Block interface.
func (n *neoBlock) Verify(key crypto.PublicKey, sign []byte) error {
	data := n.Base.GetSignedPart()
	return key.Verify(data, sign)
}

// Transactions implements block.Block interface.
func (n *neoBlock) Transactions() []block.Transaction {
	txes := make([]block.Transaction, len(n.Block.Transactions))
	for i, tx := range n.Block.Transactions {
		txes[i] = tx
	}

	return txes
}

// SetTransactions implements block.Block interface.
func (n *neoBlock) SetTransactions(txes []block.Transaction) {
	n.Block.Transactions = make([]*transaction.Transaction, len(txes))
	for i, tx := range txes {
		n.Block.Transactions[i] = tx.(*transaction.Transaction)
	}
}

// Version implements block.Block interface.
func (n *neoBlock) Version() uint32 { return n.Block.Version }

// SetVersion implements block.Block interface.
func (n *neoBlock) SetVersion(v uint32) { n.Block.Version = v }

// PrevHash implements block.Block interface.
func (n *neoBlock) PrevHash() util.Uint256 { return n.Block.PrevHash }

// SetPrevHash implements block.Block interface.
func (n *neoBlock) SetPrevHash(h util.Uint256) { n.Block.PrevHash = h }

// MerkleRoot implements block.Block interface.
func (n *neoBlock) MerkleRoot() util.Uint256 { return n.Block.MerkleRoot }

// SetMerkleRoot implements block.Block interface.
func (n *neoBlock) SetMerkleRoot(r util.Uint256) { n.Block.MerkleRoot = r }

// Timestamp implements block.Block interface.
func (n *neoBlock) Timestamp() uint64 { return n.Block.Timestamp * 1000000 }

// SetTimestamp implements block.Block interface.
func (n *neoBlock) SetTimestamp(ts uint64) { n.Block.Timestamp = ts / 1000000 }

// Index implements block.Block interface.
func (n *neoBlock) Index() uint32 { return n.Block.Index }

// SetIndex implements block.Block interface.
func (n *neoBlock) SetIndex(i uint32) { n.Block.Index = i }

// ConsensusData implements block.Block interface.
func (n *neoBlock) ConsensusData() uint64 { return n.Block.ConsensusData }

// SetConsensusData implements block.Block interface.
func (n *neoBlock) SetConsensusData(nonce uint64) { n.Block.ConsensusData = nonce }

// NextConsensus implements block.Block interface.
func (n *neoBlock) NextConsensus() util.Uint160 { return n.Block.NextConsensus }

// SetNextConsensus implements block.Block interface.
func (n *neoBlock) SetNextConsensus(h util.Uint160) { n.Block.NextConsensus = h }

// Signature implements block.Block interface.
func (n *neoBlock) Signature() []byte { return n.signature }
