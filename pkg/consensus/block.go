package consensus

import (
	"errors"

	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	coreb "github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// neoBlock is a wrapper of a core.Block which implements
// methods necessary for dBFT library.
type neoBlock struct {
	coreb.Block

	network   netmode.Magic
	signature []byte
}

var _ dbft.Block[util.Uint256] = (*neoBlock)(nil)

// Sign implements the block.Block interface.
func (n *neoBlock) Sign(key dbft.PrivateKey) error {
	k := key.(*privateKey)
	sig := k.PrivateKey.SignHashable(uint32(n.network), &n.Block)
	n.signature = sig
	return nil
}

// Verify implements the block.Block interface.
func (n *neoBlock) Verify(key dbft.PublicKey, sign []byte) error {
	k := key.(*keys.PublicKey)
	if k.VerifyHashable(sign, uint32(n.network), &n.Block) {
		return nil
	}
	return errors.New("verification failed")
}

// Transactions implements the block.Block interface.
func (n *neoBlock) Transactions() []dbft.Transaction[util.Uint256] {
	txes := make([]dbft.Transaction[util.Uint256], len(n.Block.Transactions))
	for i, tx := range n.Block.Transactions {
		txes[i] = tx
	}

	return txes
}

// SetTransactions implements the block.Block interface.
func (n *neoBlock) SetTransactions(txes []dbft.Transaction[util.Uint256]) {
	n.Block.Transactions = make([]*transaction.Transaction, len(txes))
	for i, tx := range txes {
		n.Block.Transactions[i] = tx.(*transaction.Transaction)
	}
}

// PrevHash implements the block.Block interface.
func (n *neoBlock) PrevHash() util.Uint256 { return n.Block.PrevHash }

// MerkleRoot implements the block.Block interface.
func (n *neoBlock) MerkleRoot() util.Uint256 { return n.Block.MerkleRoot }

// Timestamp implements the block.Block interface.
func (n *neoBlock) Timestamp() uint64 { return n.Block.Timestamp * nsInMs }

// Index implements the block.Block interface.
func (n *neoBlock) Index() uint32 { return n.Block.Index }

// Signature implements the block.Block interface.
func (n *neoBlock) Signature() []byte { return n.signature }
