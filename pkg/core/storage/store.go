package storage

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/btree"
)

// KeyPrefix constants.
const (
	DataExecutable KeyPrefix = 0x01
	DataMPT        KeyPrefix = 0x03
	STAccount      KeyPrefix = 0x40
	STContractID   KeyPrefix = 0x51
	STStorage      KeyPrefix = 0x70
	// STTempStorage is used to store contract storage items during state sync process
	// in order not to mess up the previous state which has its own items stored by
	// STStorage prefix. Once state exchange process is completed, all items with
	// STStorage prefix will be replaced with STTempStorage-prefixed ones.
	STTempStorage                  KeyPrefix = 0x71
	STNEP11Transfers               KeyPrefix = 0x72
	STNEP17Transfers               KeyPrefix = 0x73
	STTokenTransferInfo            KeyPrefix = 0x74
	IXHeaderHashList               KeyPrefix = 0x80
	SYSCurrentBlock                KeyPrefix = 0xc0
	SYSCurrentHeader               KeyPrefix = 0xc1
	SYSStateSyncCurrentBlockHeight KeyPrefix = 0xc2
	SYSStateSyncPoint              KeyPrefix = 0xc3
	SYSStateJumpStage              KeyPrefix = 0xc4
	SYSCleanStorage                KeyPrefix = 0xc5
	SYSVersion                     KeyPrefix = 0xf0
)

// Executable subtypes.
const (
	ExecBlock       byte = 1
	ExecTransaction byte = 2
)

const (
	// MaxStorageKeyLen is the maximum length of a key for storage items.
	MaxStorageKeyLen = 64
	// MaxStorageValueLen is the maximum length of a value for storage items.
	// It is set to be the maximum value for uint16.
	MaxStorageValueLen = 65535
)

// Operation represents a single KV operation (add/del/change) performed
// in the DB.
type Operation struct {
	// State can be Added, Changed or Deleted.
	State string `json:"state"`
	Key   []byte `json:"key"`
	Value []byte `json:"value,omitempty"`
}

// SeekRange represents options for Store.Seek operation.
type SeekRange struct {
	// Prefix denotes the Seek's lookup key.
	// Empty Prefix means seeking through all keys in the DB starting from
	// the Start if specified.
	Prefix []byte
	// Start denotes value appended to the Prefix to start Seek from.
	// Seeking starting from some key includes this key to the result;
	// if no matching key was found then next suitable key is picked up.
	// Start may be empty. Empty Start means seeking through all keys in
	// the DB with matching Prefix.
	// Empty Prefix and empty Start can be combined, which means seeking
	// through all keys in the DB.
	Start []byte
	// Backwards denotes whether Seek direction should be reversed, i.e.
	// whether seeking should be performed in a descending way.
	// Backwards can be safely combined with Prefix and Start.
	Backwards bool
}

// ErrKeyNotFound is an error returned by Store implementations
// when a certain key is not found.
var ErrKeyNotFound = errors.New("key not found")

type (
	// Store is anything that can persist and retrieve the blockchain.
	// information.
	Store interface {
		Batch() Batch
		Delete(k []byte) error
		Get([]byte) ([]byte, error)
		Put(k, v []byte) error
		PutBatch(Batch) error
		// PutChangeSet allows to push prepared changeset to the Store.
		PutChangeSet(puts *btree.BTree) error
		// Seek can guarantee that provided key (k) and value (v) are the only valid until the next call to f.
		// Seek continues iteration until false is returned from f.
		// Key and value slices should not be modified.
		// Seek can guarantee that key-value items are sorted by key in ascending way.
		Seek(rng SeekRange, f func(k, v []byte) bool)
		Close() error
	}

	// Batch represents an abstraction on top of batch operations.
	// Each Store implementation is responsible of casting a Batch
	// to its appropriate type. Batches can only be used in a single
	// thread.
	Batch interface {
		Delete(k []byte)
		Put(k, v []byte)
	}

	// KeyPrefix is a constant byte added as a prefix for each key
	// stored.
	KeyPrefix uint8
)

// Bytes returns the bytes representation of KeyPrefix.
func (k KeyPrefix) Bytes() []byte {
	return []byte{byte(k)}
}

// AppendPrefix appends byteslice b to the given KeyPrefix.
// AppendKeyPrefix(SYSVersion, []byte{0x00, 0x01}).
func AppendPrefix(k KeyPrefix, b []byte) []byte {
	dest := make([]byte, len(b)+1)
	dest[0] = byte(k)
	copy(dest[1:], b)
	return dest
}

// AppendPrefixInt append int n to the given KeyPrefix.
//   AppendPrefixInt(SYSCurrentHeader, 10001)
func AppendPrefixInt(k KeyPrefix, n int) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(n))
	return AppendPrefix(k, b)
}

// NewStore creates storage with preselected in configuration database type.
func NewStore(cfg DBConfiguration) (Store, error) {
	var store Store
	var err error
	switch cfg.Type {
	case "leveldb":
		store, err = NewLevelDBStore(cfg.LevelDBOptions)
	case "inmemory":
		store = NewMemoryStore()
	case "boltdb":
		store, err = NewBoltDBStore(cfg.BoltDBOptions)
	default:
		return nil, fmt.Errorf("unknown storage: %s", cfg.Type)
	}
	return store, err
}

// BatchToOperations converts a batch of changes into array of Operations.
func BatchToOperations(batch *MemBatch) []Operation {
	size := len(batch.Put) + len(batch.Deleted)
	ops := make([]Operation, 0, size)
	for i := range batch.Put {
		key := batch.Put[i].Key
		if len(key) == 0 || key[0] != byte(STStorage) && key[0] != byte(STTempStorage) {
			continue
		}

		op := "Added"
		if batch.Put[i].Exists {
			op = "Changed"
		}

		ops = append(ops, Operation{
			State: op,
			Key:   key[1:],
			Value: batch.Put[i].Value,
		})
	}

	for i := range batch.Deleted {
		key := batch.Deleted[i].Key
		if len(key) == 0 || !batch.Deleted[i].Exists ||
			key[0] != byte(STStorage) && key[0] != byte(STTempStorage) {
			continue
		}

		ops = append(ops, Operation{
			State: "Deleted",
			Key:   key[1:],
		})
	}
	return ops
}
