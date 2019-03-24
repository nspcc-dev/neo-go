package chain

import (
	"bufio"
	"bytes"
	"encoding/binary"

	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

var (
	// TX is the prefix used when inserting a tx into the db
	TX = []byte("TX")
	// HEADER is the prefix used when inserting a header into the db
	HEADER = []byte("HE")
	// LATESTHEADER is the prefix used when inserting the latests header into the db
	LATESTHEADER = []byte("LH")
	// UTXO is the prefix used when inserting a utxo into the db
	UTXO = []byte("UT")
	// LATESTBLOCK is the prefix used when inserting the latest block into the db
	LATESTBLOCK = []byte("LB")
	// BLOCKHASHTX is the prefix used when linking a blockhash to a given tx
	BLOCKHASHTX = []byte("BT")
	// BLOCKHASHHEIGHT is the prefix used when linking a blockhash to it's height
	// This is linked both ways
	BLOCKHASHHEIGHT = []byte("BH")
	// SCRIPTHASHUTXO is the prefix used when linking a utxo to a scripthash
	// This is linked both ways
	SCRIPTHASHUTXO = []byte("SU")
)

// Chaindb is a wrapper around the db interface which adds an extra block chain specific layer on top.
type Chaindb struct {
	db database.Database
}

// This should not be exported for other callers.
// It is safe-guarded by the chain's verification logic
func (c *Chaindb) saveBlock(blk payload.Block, genesis bool) error {

	latestBlockTable := database.NewTable(c.db, LATESTBLOCK)
	hashHeightTable := database.NewTable(c.db, BLOCKHASHHEIGHT)

	// Save Txs and link to block hash
	err := c.saveTXs(blk.Txs, blk.Hash.Bytes(), genesis)
	if err != nil {
		return err
	}

	// LINK block height to hash - Both ways
	// This allows us to fetch a block using it's hash or it's height
	// Given the height, we will search the table to get the hash
	// We can then fetch all transactions in the tx table, which match that block hash
	height := uint32ToBytes(blk.Index)
	err = hashHeightTable.Put(height, blk.Hash.Bytes())
	if err != nil {
		return err
	}

	err = hashHeightTable.Put(blk.Hash.Bytes(), height)
	if err != nil {
		return err
	}

	// Add block as latest block
	// This also acts a Commit() for the block.
	// If an error occured, then this will be set to the previous block
	// This is useful because if the node suddently shut down while saving and the database was not corrupted
	// Then the node will see the latestBlock as the last saved block, and re-download the faulty block
	// Note: We check for the latest block on startup
	return latestBlockTable.Put([]byte(""), blk.Hash.Bytes())
}

// Saves a tx and links each tx to the block it was found in
// This should never be exported. Only way to add a tx, is through it's block
func (c *Chaindb) saveTXs(txs []transaction.Transactioner, blockHash []byte, genesis bool) error {

	for txIndex, tx := range txs {
		err := c.saveTx(tx, uint32(txIndex), blockHash, genesis)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Chaindb) saveTx(tx transaction.Transactioner, txIndex uint32, blockHash []byte, genesis bool) error {

	txTable := database.NewTable(c.db, TX)
	blockTxTable := database.NewTable(c.db, BLOCKHASHTX)

	// Save the whole tx using it's hash a key
	// In order to find a tx in this table, we need to know it's hash
	txHash, err := tx.ID()
	if err != nil {
		return err
	}
	err = txTable.Put(txHash.Bytes(), tx.BaseTx().Bytes())
	if err != nil {
		return err
	}

	// LINK TXhash to block
	// This allows us to fetch a tx by just knowing what block it was in
	// This is useful for when we want to re-construct a block from it's hash
	// In order to ge the tx, we must do a prefix search on blockHash
	// This will return a set of txHashes.
	//We can then use these hashes to search the txtable for the tx's we need
	key := bytesConcat(blockHash, uint32ToBytes(txIndex))
	err = blockTxTable.Put(key, txHash.Bytes())
	if err != nil {
		return err
	}

	// Save all of the utxos in a transaction
	// We do this additional save so that we can form a utxo database
	// and know when a transaction is a double spend.
	utxos := tx.BaseTx().UTXOs()
	for utxoIndex, utxo := range utxos {
		err := c.saveUTXO(utxo, uint16(utxoIndex), txHash.Bytes(), blockHash)
		if err != nil {
			return err
		}
	}

	// Do not check for spent utxos on the genesis block
	if genesis {
		return nil
	}

	// Remove all spent utxos
	// We do this so that once an output has been spent
	// It will be removed from the utxo database and cannot be spent again
	// If the output was never in the utxo database, this function will return an error
	txos := tx.BaseTx().TXOs()
	for _, txo := range txos {
		err := c.removeUTXO(txo)
		if err != nil {
			return err
		}
	}
	return nil
}

// saveUTxo will save a utxo and link it to it's transaction and block
func (c *Chaindb) saveUTXO(utxo *transaction.Output, utxoIndex uint16, txHash, blockHash []byte) error {

	utxoTable := database.NewTable(c.db, UTXO)
	scripthashUTXOTable := database.NewTable(c.db, SCRIPTHASHUTXO)

	// This is quite messy, we should (if possible) find a way to pass a Writer and Reader interface
	// Encode utxo into a buffer
	buf := new(bytes.Buffer)
	bw := &util.BinWriter{W: buf}
	if utxo.Encode(bw); bw.Err != nil {
		return bw.Err
	}

	// Save UTXO
	// In order to find a utxo in the utxoTable
	// One must know the txHash that the utxo was in
	key := bytesConcat(txHash, uint16ToBytes(utxoIndex))
	if err := utxoTable.Put(key, buf.Bytes()); err != nil {
		return err
	}

	// LINK utxo to scripthash
	// This allows us to find a utxo with the scriptHash
	// Since the key starts with scriptHash, we can look for the scriptHash prefix
	// and find all utxos for a given scriptHash.
	// Additionally, we can search for all utxos for a certain user in a certain block with scriptHash+blockHash
	// But this may not be of use to us. However, note that we cannot have just the scriptHash with the utxoIndex
	// as this may not be unique. If Kim/Dautt agree, we can change blockHash to blockHeight, which allows us
	// To get all utxos above a certain blockHeight. Question is; Would this be useful?
	newKey := bytesConcat(utxo.ScriptHash.Bytes(), blockHash, uint16ToBytes(utxoIndex))
	if err := scripthashUTXOTable.Put(newKey, key); err != nil {
		return err
	}
	if err := scripthashUTXOTable.Put(key, newKey); err != nil {
		return err
	}
	return nil
}

// Remove
func (c *Chaindb) removeUTXO(txo *transaction.Input) error {

	utxoTable := database.NewTable(c.db, UTXO)
	scripthashUTXOTable := database.NewTable(c.db, SCRIPTHASHUTXO)

	// Remove spent utxos from utxo database
	key := bytesConcat(txo.PrevHash.Bytes(), uint16ToBytes(txo.PrevIndex))
	err := utxoTable.Delete(key)
	if err != nil {
		return err
	}

	// Remove utxos from scripthash table
	otherKey, err := scripthashUTXOTable.Get(key)
	if err != nil {
		return err
	}
	if err := scripthashUTXOTable.Delete(otherKey); err != nil {
		return err
	}
	if err := scripthashUTXOTable.Delete(key); err != nil {
		return err
	}

	return nil
}

// saveHeaders will save a set of headers into the database
func (c *Chaindb) saveHeaders(headers []*payload.BlockBase) error {

	for _, hdr := range headers {
		err := c.saveHeader(hdr)
		if err != nil {
			return err
		}
	}
	return nil
}

// saveHeader saves a header into the database and updates the latest header
// The headers are saved with their `blockheights` as Key
// If we want to search for a header, we need to know it's index
// Alternatively, we can search the hashHeightTable with the block index to get the hash
// If the block has been saved.
// The reason why headers are saved with their index as Key, is so that we can
// increment the key to find out what block we should fetch next during the initial
// block download, when we are saving thousands of headers
func (c *Chaindb) saveHeader(hdr *payload.BlockBase) error {

	headerTable := database.NewTable(c.db, HEADER)
	latestHeaderTable := database.NewTable(c.db, LATESTHEADER)

	index := uint32ToBytes(hdr.Index)

	byt, err := hdr.Bytes()
	if err != nil {
		return err
	}

	err = headerTable.Put(index, byt)
	if err != nil {
		return err
	}

	// Update latest header
	return latestHeaderTable.Put([]byte(""), index)
}

// GetHeaderFromHeight will get a header given it's block height
func (c *Chaindb) GetHeaderFromHeight(index []byte) (*payload.BlockBase, error) {
	headerTable := database.NewTable(c.db, HEADER)
	hdrBytes, err := headerTable.Get(index)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(hdrBytes)

	blockBase := &payload.BlockBase{}
	err = blockBase.Decode(reader)
	if err != nil {
		return nil, err
	}
	return blockBase, nil
}

// GetLastHeader will get the header which was saved last in the database
func (c *Chaindb) GetLastHeader() (*payload.BlockBase, error) {

	latestHeaderTable := database.NewTable(c.db, LATESTHEADER)
	index, err := latestHeaderTable.Get([]byte(""))
	if err != nil {
		return nil, err
	}
	return c.GetHeaderFromHeight(index)
}

// GetBlockFromHash will return a block given it's hash
func (c *Chaindb) GetBlockFromHash(blockHash []byte) (*payload.Block, error) {

	blockTxTable := database.NewTable(c.db, BLOCKHASHTX)

	// To get a block we need to fetch:
	// The transactions (1)
	// The header (2)

	// Reconstruct block by fetching it's txs (1)
	var txs []transaction.Transactioner

	// Get all Txhashes for this block
	txHashes, err := blockTxTable.Prefix(blockHash)
	if err != nil {
		return nil, err
	}

	// Get all Tx's given their hash
	txTable := database.NewTable(c.db, TX)
	for _, txHash := range txHashes {

		// Fetch tx by it's hash
		txBytes, err := txTable.Get(txHash)
		if err != nil {
			return nil, err
		}
		reader := bufio.NewReader(bytes.NewReader(txBytes))

		tx, err := transaction.FromReader(reader)
		if err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}

	// Now fetch the header (2)
	// We have the block hash, but headers are stored with their `Height` as key.
	// We first search the `BlockHashHeight` table to get the height.
	//Then we search the headers table with the height
	hashHeightTable := database.NewTable(c.db, BLOCKHASHHEIGHT)
	height, err := hashHeightTable.Get(blockHash)
	if err != nil {
		return nil, err
	}
	hdr, err := c.GetHeaderFromHeight(height)
	if err != nil {
		return nil, err
	}

	// Construct block
	block := &payload.Block{
		BlockBase: *hdr,
		Txs:       txs,
	}
	return block, nil
}

// GetLastBlock will return the last block that has been saved
func (c *Chaindb) GetLastBlock() (*payload.Block, error) {

	latestBlockTable := database.NewTable(c.db, LATESTBLOCK)
	blockHash, err := latestBlockTable.Get([]byte(""))
	if err != nil {
		return nil, err
	}
	return c.GetBlockFromHash(blockHash)
}

func uint16ToBytes(x uint16) []byte {
	index := make([]byte, 2)
	binary.BigEndian.PutUint16(index, x)
	return index
}

func uint32ToBytes(x uint32) []byte {
	index := make([]byte, 4)
	binary.BigEndian.PutUint32(index, x)
	return index
}

func bytesConcat(args ...[]byte) []byte {
	var res []byte
	for _, arg := range args {
		res = append(res, arg...)
	}
	return res
}
