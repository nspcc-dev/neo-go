package storage

import (
	"errors"
	"fmt"
	"slices"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dboper"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// KeyPrefix constants.
const (
	DataExecutable KeyPrefix = 0x01
	// DataMPT is used for MPT node entries identified by Uint256.
	DataMPT KeyPrefix = 0x03
	// DataMPTAux is used to store additional MPT data like height-root
	// mappings and local/validated heights.
	DataMPTAux KeyPrefix = 0x04
	STStorage  KeyPrefix = 0x70
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
	// SYSStateChangeStage is used to store the phase of a state changing process
	// which is one of the state jump or state reset. Its value is one byte containing
	// state reset / state jump stages bits (first seven bits are reserved for that)
	// and the last bit reserved for the state reset process marker (set to 1 on
	// unfinished state reset and to 0 on unfinished state jump).
	SYSStateChangeStage KeyPrefix = 0xc4
	// SYSStateSyncCheckpoint is used to store checkpoint of state synchronization
	// process performed by `statesync.Module`.
	SYSStateSyncCheckpoint KeyPrefix = 0xc5
	SYSVersion             KeyPrefix = 0xf0
)

// Executable subtypes.
const (
	ExecBlock       byte = 1
	ExecTransaction byte = 2
)

// SeekRange represents options for Store.Seek operation.
type SeekRange struct {
	// Prefix denotes the Seek's lookup key.
	// If used directly with MemCachedStore's or MemoryStore's Seek, SeekAsync or
	// SeekGC empty Prefix is not supported due to internal MemoryStore cache
	// architecture; otherwise empty prefix can be used safely.
	Prefix []byte
	// Start denotes value appended to the Prefix to start Seek from.
	// Seeking starting from some key includes this key to the result;
	// if no matching key was found then next suitable key is picked up.
	// Start may be empty. Empty Start means seeking through all keys in
	// the DB with matching Prefix.
	// Empty Prefix and empty Start can be combined, which means seeking
	// through all keys in the DB, but see the Prefix's comment.
	Start []byte
	// Backwards denotes whether Seek direction should be reversed, i.e.
	// whether seeking should be performed in a descending way.
	// Backwards can be safely combined with Prefix and Start.
	Backwards bool
	// SearchDepth is the depth of Seek operation, denotes the number of cached
	// DAO layers to perform search. Use 1 to fetch the latest changes from upper
	// in-memory layer of cached DAO. Default 0 value denotes searching through
	// the whole set of cached layers.
	SearchDepth int
}

// ErrKeyNotFound is an error returned by Store implementations
// when a certain key is not found.
var ErrKeyNotFound = errors.New("key not found")

type (
	// Store is the underlying KV backend for the blockchain data, it's
	// not intended to be used directly, you wrap it with some memory cache
	// layer most of the time.
	Store interface {
		Get([]byte) ([]byte, error)
		// PutChangeSet allows to push prepared changeset to the Store.
		PutChangeSet(puts map[string][]byte, stor map[string][]byte) error
		// Seek can guarantee that provided key (k) and value (v) are the only valid until the next call to f.
		// Seek continues iteration until false is returned from f.
		// Key and value slices should not be modified.
		// Seek can guarantee that key-value items are sorted by key in ascending way.
		Seek(rng SeekRange, f func(k, v []byte) bool)
		// SeekGC is similar to Seek, but the function should return true if current
		// KV pair should be kept and false if it's to be deleted; there is no way to
		// do an early exit here. SeekGC only works with the current Store, it won't
		// go down to layers below and it takes a full write lock, so use it carefully.
		SeekGC(rng SeekRange, keep func(k, v []byte) bool) error
		Close() error
	}

	// KeyPrefix is a constant byte added as a prefix for each key
	// stored.
	KeyPrefix uint8
)

func seekRangeToPrefixes(sr SeekRange) *util.Range {
	var (
		rang  *util.Range
		start = slices.Concat(sr.Prefix, sr.Start)
	)

	if !sr.Backwards {
		rang = util.BytesPrefix(sr.Prefix)
		rang.Start = start
	} else {
		rang = util.BytesPrefix(start)
		rang.Start = sr.Prefix
	}
	return rang
}

// NewStore creates storage with preselected in configuration database type.
func NewStore(cfg dbconfig.DBConfiguration) (Store, error) {
	var store Store
	var err error
	switch cfg.Type {
	case dbconfig.LevelDB:
		store, err = NewLevelDBStore(cfg.LevelDBOptions)
	case dbconfig.InMemoryDB:
		store = NewMemoryStore()
	case dbconfig.BoltDB:
		store, err = NewBoltDBStore(cfg.BoltDBOptions)
	default:
		return nil, fmt.Errorf("unknown storage: %s", cfg.Type)
	}
	return store, err
}

// BatchToOperations converts a batch of changes into array of dboper.Operation.
func BatchToOperations(batch *MemBatch) []dboper.Operation {
	size := len(batch.Put) + len(batch.Deleted)
	ops := make([]dboper.Operation, 0, size)
	for i := range batch.Put {
		key := batch.Put[i].Key
		if len(key) == 0 || key[0] != byte(STStorage) && key[0] != byte(STTempStorage) {
			continue
		}

		op := "Added"
		if batch.Put[i].Exists {
			op = "Changed"
		}

		ops = append(ops, dboper.Operation{
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

		ops = append(ops, dboper.Operation{
			State: "Deleted",
			Key:   key[1:],
		})
	}
	return ops
}
