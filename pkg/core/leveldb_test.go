package core

import (
	"os"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	path = "test_chain"
)

func TestPersistBlock(t *testing.T) {
}

func newBlockchain() *Blockchain {
	startHash, _ := util.Uint256DecodeString("a")
	opts := &opt.Options{}
	store, _ := NewLevelDBStore(path, opts)
	chain := NewBlockchain(
		store,
		startHash,
	)
	return chain
}

func tearDown() error {
	return os.RemoveAll(path)
}
