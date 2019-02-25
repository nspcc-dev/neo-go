package database

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

type LDB struct {
	db   *leveldb.DB
	path string
}

type Database interface {
	Has(key []byte) (bool, error)
	Put(key []byte, value []byte) error
	Get(key []byte) ([]byte, error)
	Delete(key []byte) error
	Close() error
}

var (
	// TX, HEADER AND UTXO are the prefixes for the db
	TX           = []byte("TX")
	HEADER       = []byte("HEADER")
	LATESTHEADER = []byte("LH")
	UTXO         = []byte("UTXO")
)

func New(path string) *LDB {
	db, err := leveldb.OpenFile(path, nil)

	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(path, nil)
	}

	if err != nil {
		return nil
	}

	return &LDB{
		db,
		path,
	}
}

func (l *LDB) Has(key []byte) (bool, error) {
	return l.db.Has(key, nil)
}

func (l *LDB) Put(key []byte, value []byte) error {
	return l.db.Put(key, value, nil)
}
func (l *LDB) Get(key []byte) ([]byte, error) {
	return l.db.Get(key, nil)
}
func (l *LDB) Delete(key []byte) error {
	return l.db.Delete(key, nil)
}
func (l *LDB) Close() error {
	return l.db.Close()
}

func (l *LDB) AddHeader(header *payload.BlockBase) error {

	table := NewTable(l, HEADER)

	byt, err := header.Bytes()
	if err != nil {
		fmt.Println("Could not Get bytes from decoded BlockBase")
		return nil
	}

	fmt.Println("Adding Header, This should be batched!!!!")

	// This is the main mapping
	//Key: HEADER+BLOCKHASH Value: contents of blockhash
	key := header.Hash.Bytes()
	err = table.Put(key, byt)
	if err != nil {
		fmt.Println("Error trying to add the original mapping into the DB for Header. Mapping is [Header]+[Hash]")
		return err
	}

	// This is the secondary mapping
	// Key: HEADER + BLOCKHEIGHT Value: blockhash

	bh := uint32ToBytes(header.Index)
	key = []byte(bh)
	err = table.Put(key, header.Hash.Bytes())
	if err != nil {
		return err
	}
	// This is the third mapping
	// WARNING: This assumes that headers are adding in order.
	return table.Put(LATESTHEADER, header.Hash.Bytes())
}

func (l *LDB) AddTransactions(blockhash util.Uint256, txs []transaction.Transactioner) error {

	// SHOULD BE DONE IN BATCH!!!!
	for i, tx := range txs {
		buf := new(bytes.Buffer)
		fmt.Println(tx.ID())
		tx.Encode(buf)
		txByt := buf.Bytes()
		txhash, err := tx.ID()
		if err != nil {
			fmt.Println("Error adding transaction with bytes", txByt)
			return err
		}
		// This is the original mapping
		// Key: [TX] + TXHASH
		key := append(TX, txhash.Bytes()...)
		l.Put(key, txByt)

		// This is the index
		// Key: [TX] + BLOCKHASH + I <- i is the incrementer from the for loop
		//Value : TXHASH
		key = append(TX, blockhash.Bytes()...)
		key = append(key, uint32ToBytes(uint32(i))...)

		err = l.Put(key, txhash.Bytes())
		if err != nil {
			fmt.Println("Error could not add tx index into db")
			return err
		}
	}
	return nil
}

// BigEndian
func uint32ToBytes(h uint32) []byte {
	a := make([]byte, 4)
	binary.BigEndian.PutUint32(a, h)
	return a
}
