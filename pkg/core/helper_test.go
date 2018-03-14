package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func getBlockData(i int) (map[string]interface{}, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("test_data/block_%d.json", i))
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, err
}
