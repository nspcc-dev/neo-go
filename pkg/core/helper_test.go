package core

import (
	"crypto/sha256"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

func newBlock(index uint32, txs ...*transaction.Transaction) *Block {
	b := &Block{
		BlockBase: BlockBase{
			Version:       0,
			PrevHash:      sha256.Sum256([]byte("a")),
			MerkleRoot:    sha256.Sum256([]byte("b")),
			Timestamp:     uint32(time.Now().UTC().Unix()),
			Index:         index,
			ConsensusData: 1111,
			NextConsensus: util.Uint160{},
			Script: &transaction.Witness{
				VerificationScript: []byte{0x0},
				InvocationScript:   []byte{0x1},
			},
		},
		Transactions: txs,
	}
	hash, err := b.createHash()
	if err != nil {
		panic(err)
	}
	b.hash = hash
	return b
}

func newTX(t transaction.TXType) *transaction.Transaction {
	return &transaction.Transaction{
		Type: t,
	}
}
