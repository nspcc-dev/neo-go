package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// KeyPrefix constants.
const (
	DataBlock       KeyPrefix = 0x01
	DataTransaction KeyPrefix = 0x02
	DataMPT         KeyPrefix = 0x03
	STAccount       KeyPrefix = 0x40
	STNotification  KeyPrefix = 0x4d
	STContractID    KeyPrefix = 0x51
	STStorage       KeyPrefix = 0x70
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
	SYSVersion                     KeyPrefix = 0xf0
)

const (
	// MaxStorageKeyLen is the maximum length of a key for storage items.
	MaxStorageKeyLen = 64
	// MaxStorageValueLen is the maximum length of a value for storage items.
	// It is set to be the maximum value for uint16.
	MaxStorageValueLen = 65535
)

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
		PutChangeSet(puts map[string][]byte, dels map[string]bool) error
		// Seek can guarantee that provided key (k) and value (v) are the only valid until the next call to f.
		// Key and value slices should not be modified. Seek can guarantee that key-value items are sorted by
		// key in ascending way.
		Seek(k []byte, f func(k, v []byte))
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
