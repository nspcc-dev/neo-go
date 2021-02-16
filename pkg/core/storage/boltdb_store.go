package storage

import (
	"bytes"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/syndtr/goleveldb/leveldb/util"
	"go.etcd.io/bbolt"
)

// BoltDBOptions configuration for boltdb.
type BoltDBOptions struct {
	FilePath string `yaml:"FilePath"`
}

// BoltDBStore it is the storage implementation for storing and retrieving
// blockchain data.
type BoltDBStore struct {
	db *bbolt.DB
}

// NewBoltDBStore returns a new ready to use BoltDB storage with created bucket.
func NewBoltDBStore(cfg BoltDBOptions) (*BoltDBStore, error) {
	var opts *bbolt.Options       // should be exposed via BoltDBOptions if anything needed
	fileMode := os.FileMode(0600) // should be exposed via BoltDBOptions if anything needed
	fileName := cfg.FilePath
	if err := io.MakeDirForFile(fileName, "BoltDB"); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(fileName, fileMode, opts)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		prefixes := []KeyPrefix{
			DataBlock,
			DataTransaction,
			DataMPT,
			STAccount,
			STNotification,
			STContractID,
			STStorage,
			STNEP17Transfers,
			STNEP17Balances,
			IXHeaderHashList,
			SYSCurrentBlock,
			SYSCurrentHeader,
			SYSVersion,
		}
		for _, p := range prefixes {
			_, err = tx.CreateBucketIfNotExists([]byte{byte(p)})
			if err != nil {
				return err
			}
		}
		//key := make([]byte, 5)
		//key[0] = byte(STStorage)
		//for _, id := range []int32{-3, -4} {
		//	binary.LittleEndian.PutUint32(key[1:], uint32(id))
		//	_, err := tx.CreateBucketIfNotExists(key)
		//	if err != nil {
		//		return err
		//	}
		//}
		return nil
	})

	return &BoltDBStore{db: db}, err
}

func split(key []byte) ([]byte, []byte) {
	switch KeyPrefix(key[0]) {
	case DataMPT:
		return key[:1], key
	default:
		return key[:1], key
	}
}

// Put implements the Store interface.
func (s *BoltDBStore) Put(key, value []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		k1, k2 := split(key)
		b := tx.Bucket(k1)
		err := b.Put(k2, value)
		return err
	})
}

// Get implements the Store interface.
func (s *BoltDBStore) Get(key []byte) (val []byte, err error) {
	err = s.db.View(func(tx *bbolt.Tx) error {
		k1, k2 := split(key)
		b := tx.Bucket(k1)
		val = b.Get(k2)
		// Value from Get is only valid for the lifetime of transaction, #1482
		if val != nil {
			var valcopy = make([]byte, len(val))
			copy(valcopy, val)
			val = valcopy
		}
		return nil
	})
	if val == nil {
		err = ErrKeyNotFound
	}
	return
}

// Delete implements the Store interface.
func (s *BoltDBStore) Delete(key []byte) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		k1, k2 := split(key)
		b := tx.Bucket(k1)
		return b.Delete(k2)
	})
}

// PutBatch implements the Store interface.
func (s *BoltDBStore) PutBatch(batch Batch) error {
	return s.db.Batch(func(tx *bbolt.Tx) error {
		for k, v := range batch.(*MemoryBatch).mem {
			k1, k2 := split([]byte(k))
			err := tx.Bucket(k1).Put(k2, v)
			if err != nil {
				return err
			}
		}
		for k := range batch.(*MemoryBatch).del {
			k1, k2 := split([]byte(k))
			err := tx.Bucket(k1).Delete(k2)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Seek implements the Store interface.
func (s *BoltDBStore) Seek(key []byte, f func(k, v []byte)) {
	err := s.db.View(func(tx *bbolt.Tx) error {
		k1, k2 := split([]byte(key))
		c := tx.Bucket(k1).Cursor()
		prefix := util.BytesPrefix(k2)
		for k, v := c.Seek(prefix.Start); k != nil && bytes.Compare(k, prefix.Limit) <= 0; k, v = c.Next() {
			f(k, v)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}

// Batch implements the Batch interface and returns a boltdb
// compatible Batch.
func (s *BoltDBStore) Batch() Batch {
	return newMemoryBatch()
}

// Close releases all db resources.
func (s *BoltDBStore) Close() error {
	return s.db.Close()
}
