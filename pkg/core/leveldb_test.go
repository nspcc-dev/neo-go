package core

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	path = "test_chain"
)

func TestStoreAsCurrentBlock(t *testing.T) {
	bc := newTestLevelDBChain(t)
	defer tearDown()

	batch := new(leveldb.Batch)
	block := getDecodedBlock(t, 1)
	storeAsCurrentBlock(batch, block)
	bc.PutBatch(batch)

	currBlock, err := bc.Get(preSYSCurrentBlock.bytes())
	if err != nil {
		t.Fatal(err)
	}
	t.Log(currBlock)
	hash, err := util.Uint256DecodeBytes(currBlock[:32])
	if err != nil {
		t.Fatal(err)
	}
	index := binary.LittleEndian.Uint32(currBlock[32:36])

	assert.Equal(t, block.Hash(), hash)
	assert.Equal(t, block.Index, index)
}

func newTestLevelDBChain(t *testing.T) *Blockchain {
	startHash, _ := util.Uint256DecodeString("a")
	opts := &opt.Options{}
	store, err := NewLevelDBStore(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := NewBlockchain(store, startHash)
	if err != nil {
		t.Fatal(err)
	}
	return chain
}

func tearDown() error {
	return os.RemoveAll(path)
}
