package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
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

	b.createHash()

	return b
}

func makeBlocks(n int) []*Block {
	blocks := make([]*Block, n)
	for i := 0; i < n; i++ {
		blocks[i] = newBlock(uint32(i+1), newTX(transaction.MinerType))
	}
	return blocks
}

func newTX(t transaction.TXType) *transaction.Transaction {
	return &transaction.Transaction{
		Type: t,
	}
}

func getDecodedBlock(t *testing.T, i int) *Block {
	data, err := getBlockData(i)
	if err != nil {
		t.Fatal(err)
	}

	b, err := hex.DecodeString(data["raw"].(string))
	if err != nil {
		t.Fatal(err)
	}

	block := &Block{}
	if err := block.DecodeBinary(bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}

	return block
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
