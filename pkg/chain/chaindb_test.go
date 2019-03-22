package chain

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

var s = rand.NewSource(time.Now().UnixNano())
var r = rand.New(s)

func TestLastHeader(t *testing.T) {
	_, cdb, hdrs := saveRandomHeaders(t)

	// Select last header from list of headers
	lastHeader := hdrs[len(hdrs)-1]
	// GetLastHeader from the database
	hdr, err := cdb.GetLastHeader()
	assert.Nil(t, err)
	assert.Equal(t, hdr.Index, lastHeader.Index)

	// Clean up
	os.RemoveAll(database.DbDir)
}

func TestSaveHeader(t *testing.T) {
	// save headers then fetch a random element

	db, _, hdrs := saveRandomHeaders(t)

	headerTable := database.NewTable(db, HEADER)
	// check that each header was saved
	for _, hdr := range hdrs {
		index := make([]byte, 4)
		binary.BigEndian.PutUint32(index, hdr.Index)
		ok, err := headerTable.Has(index)
		assert.Nil(t, err)
		assert.True(t, ok)
	}

	// Clean up
	os.RemoveAll(database.DbDir)
}

func TestSaveBlock(t *testing.T) {

	// Init databases
	db, err := database.New("temp.test")
	assert.Nil(t, err)

	cdb := &Chaindb{db}

	// Construct block0 and block1
	block0, block1 := twoBlocksLinked(t)

	// Save genesis header
	err = cdb.saveHeaders([]*payload.BlockBase{&block0.BlockBase})
	assert.Nil(t, err)

	// Save genesis block
	err = cdb.saveBlock(block0, true)
	assert.Nil(t, err)

	// Test genesis block saved
	testBlockWasSaved(t, cdb, block0)

	// Save block1 header
	err = cdb.saveHeaders([]*payload.BlockBase{&block1.BlockBase})
	assert.Nil(t, err)

	// Save block1
	err = cdb.saveBlock(block1, false)
	assert.Nil(t, err)

	// Test block1 was saved
	testBlockWasSaved(t, cdb, block1)

	// Clean up
	os.RemoveAll(database.DbDir)
}

func testBlockWasSaved(t *testing.T, cdb *Chaindb, block payload.Block) {
	// Fetch last block from database
	lastBlock, err := cdb.GetLastBlock()
	assert.Nil(t, err)

	// Get byte representation of last block from database
	byts, err := lastBlock.Bytes()
	assert.Nil(t, err)

	// Get byte representation of block that we saved
	blockBytes, err := block.Bytes()
	assert.Nil(t, err)

	// Should be equal
	assert.True(t, bytes.Equal(byts, blockBytes))
}

func randomHeaders(t *testing.T) []*payload.BlockBase {
	assert := assert.New(t)
	hdrsMsg, err := payload.NewHeadersMessage()
	assert.Nil(err)

	for i := 0; i < 2000; i++ {
		err = hdrsMsg.AddHeader(randomBlockBase(t))
		assert.Nil(err)
	}

	return hdrsMsg.Headers
}

func randomBlockBase(t *testing.T) *payload.BlockBase {

	base := &payload.BlockBase{
		Version:       r.Uint32(),
		PrevHash:      randUint256(t),
		MerkleRoot:    randUint256(t),
		Timestamp:     r.Uint32(),
		Index:         r.Uint32(),
		ConsensusData: r.Uint64(),
		NextConsensus: randUint160(t),
		Witness: transaction.Witness{
			InvocationScript:   []byte{0, 1, 2, 34, 56},
			VerificationScript: []byte{0, 12, 3, 45, 66},
		},
		Hash: randUint256(t),
	}
	return base
}

func randomTxs(t *testing.T) []transaction.Transactioner {

	var txs []transaction.Transactioner
	for i := 0; i < 10; i++ {
		tx := transaction.NewContract(0)
		tx.AddInput(transaction.NewInput(randUint256(t), uint16(r.Int())))
		tx.AddOutput(transaction.NewOutput(randUint256(t), r.Int63(), randUint160(t)))
		txs = append(txs, tx)
	}
	return txs
}

func saveRandomHeaders(t *testing.T) (database.Database, *Chaindb, []*payload.BlockBase) {
	db, err := database.New("temp.test")
	assert.Nil(t, err)

	cdb := &Chaindb{db}

	hdrs := randomHeaders(t)

	err = cdb.saveHeaders(hdrs)
	assert.Nil(t, err)
	return db, cdb, hdrs
}

func randUint256(t *testing.T) util.Uint256 {
	slice := make([]byte, 32)
	_, err := r.Read(slice)
	u, err := util.Uint256DecodeBytes(slice)
	assert.Nil(t, err)
	return u
}
func randUint160(t *testing.T) util.Uint160 {
	slice := make([]byte, 20)
	_, err := r.Read(slice)
	u, err := util.Uint160DecodeBytes(slice)
	assert.Nil(t, err)
	return u
}

// twoBlocksLinked will return two blocks, the second block spends from the utxos in the first
func twoBlocksLinked(t *testing.T) (payload.Block, payload.Block) {
	genesisBase := randomBlockBase(t)
	genesisTxs := randomTxs(t)
	genesisBlock := payload.Block{BlockBase: *genesisBase, Txs: genesisTxs}

	var txs []transaction.Transactioner

	// Form transactions that spend from the genesis block
	for _, tx := range genesisTxs {
		txHash, err := tx.ID()
		assert.Nil(t, err)
		newTx := transaction.NewContract(0)
		newTx.AddInput(transaction.NewInput(txHash, 0))
		newTx.AddOutput(transaction.NewOutput(randUint256(t), r.Int63(), randUint160(t)))
		txs = append(txs, newTx)
	}

	nextBase := randomBlockBase(t)
	nextBlock := payload.Block{BlockBase: *nextBase, Txs: txs}

	return genesisBlock, nextBlock
}
