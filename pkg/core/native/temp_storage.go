package native

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/limits"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeids"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// prefixTempStorage is used to store key-value pairs in the temporary storage.
	prefixTempStorage byte = 0x01
	// prefixTTL is used to store the timestamp (in milliseconds) when a particular
	// temporary key-value pair can be reached for the last time.
	prefixTTL byte = 0x02

	// maxTTL is the maximum allowed TTL window in milliseconds.
	maxTTL = uint32(7 * 24 * time.Hour / time.Millisecond) // TODO: 1. Define value; 2. Move to Policy and make dynamic?
	// maxCleanupBatchSize is the maximum number of temporary key-value entries
	// that can be removed per a single PostPersist.
	maxCleanupBatchSize = 1000 // TODO: define value.
)

// TempStorage represents TemporaryStorage native contract.
type TempStorage struct {
	interop.ContractMD
	policy IPolicy
}

var _ interop.Contract = (*TempStorage)(nil)

func newTempStorage() *TempStorage {
	s := &TempStorage{
		ContractMD: *interop.NewContractMD(nativenames.TemporaryStorage, nativeids.TemporaryStorage),
	}
	defer s.BuildHFSpecificMD(s.ActiveIn())

	desc := NewDescriptor("put", smartcontract.VoidType,
		manifest.NewParameter("key", smartcontract.ByteArrayType),
		manifest.NewParameter("value", smartcontract.ByteArrayType),
		manifest.NewParameter("ttl", smartcontract.IntegerType)) // TODO: @core needs to agree, in blocks or in milliseconds ?

	// TODO: @roman-khimov, these calls supposed to cost cheaper (or at least equal to) the price of standard
	// System.Storage.Put. System.Storage.Put costs 1<<15 (and additional fee is charged for the size of stored data).
	// However, the TempStorage contract call will always be surrounded by System.Contract.Call SYSCALL (1<<15) and requires
	// additional PUSH of TempStorage contract hash and call flags which is already greater than 1<<15.
	//
	// We may consider adding a set of `System.Storage.Temporary.*` interops that will put/get data directly to/from
	// TempStorage contract. And the TempStorage contract itself won't have any methods except OnPersist/PostPersist and
	// will only perform expiration entries management in PostPersist.
	md := NewMethodAndPrice(s.put, 1<<15, callflag.WriteStates)
	s.AddMethod(md, desc)

	desc = NewDescriptor("get", smartcontract.ByteArrayType,
		manifest.NewParameter("key", smartcontract.ByteArrayType))
	// TODO: ditto for the rest of prices.
	md = NewMethodAndPrice(s.get, 1<<15, callflag.ReadStates)

	desc = NewDescriptor("get", smartcontract.ByteArrayType,
		manifest.NewParameter("hash", smartcontract.Hash160Type),
		manifest.NewParameter("key", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.getByHash, 1<<15, callflag.ReadStates)

	// TODO: @roman-khimov, do we need this API?
	desc = NewDescriptor("getExpiration", smartcontract.ArrayType,
		manifest.NewParameter("key", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.getExpiration, 1<<15, callflag.ReadStates)

	desc = NewDescriptor("getExpiration", smartcontract.ByteArrayType,
		manifest.NewParameter("hash", smartcontract.Hash160Type),
		manifest.NewParameter("key", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.getExpirationByHash, 1<<15, callflag.ReadStates)

	desc = NewDescriptor("delete", smartcontract.ByteArrayType,
		manifest.NewParameter("key", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.delete, 1<<15, callflag.WriteStates)

	desc = NewDescriptor("find", smartcontract.InteropInterfaceType,
		manifest.NewParameter("prefix", smartcontract.ByteArrayType),
		manifest.NewParameter("opts", smartcontract.IntegerType))
	md = NewMethodAndPrice(s.find, 1<<15, callflag.ReadStates)

	desc = NewDescriptor("find", smartcontract.InteropInterfaceType,
		manifest.NewParameter("hash", smartcontract.Hash160Type),
		manifest.NewParameter("prefix", smartcontract.ByteArrayType),
		manifest.NewParameter("opts", smartcontract.IntegerType))
	md = NewMethodAndPrice(s.findByHash, 1<<15, callflag.ReadStates)

	desc = NewDescriptor("renew", smartcontract.VoidType,
		manifest.NewParameter("key", smartcontract.ByteArrayType),
		manifest.NewParameter("ttl", smartcontract.IntegerType))
	md = NewMethodAndPrice(s.renew, 1<<15, callflag.WriteStates)
	s.AddMethod(md, desc)

	return s
}

// OnPersist implements the [interop.Contract] interface.
func (s *TempStorage) OnPersist(ic *interop.Context) error {
	return nil
}

// PostPersist implements the [interop.Contract] interface.
func (s *TempStorage) PostPersist(ic *interop.Context) error {
	// Remove top maxCleanupBatchSize most recent expired entries, but the older
	// ones will also be removed eventually.
	var (
		i     int
		start = make([]byte, 8)
	)
	binary.BigEndian.PutUint64(start, ic.Block.Timestamp)
	ic.DAO.Seek(s.ID, storage.SeekRange{
		Prefix:    []byte{prefixTTL},
		Start:     start,
		Backwards: true,
	}, func(k, _ []byte) bool {
		key := append([]byte{prefixTTL}, k...)
		ic.DAO.DeleteStorageItem(s.ID, key)

		key = key[:0]
		key[0] = prefixTempStorage
		copy(key[1:], k[9:])
		ic.DAO.DeleteStorageItem(s.ID, key)

		i++
		return i < maxCleanupBatchSize
	})
	return nil
}

// Metadata implements the [interop.Contract] interface.
func (s *TempStorage) Metadata() *interop.ContractMD {
	return &s.ContractMD
}

// Initialize implements the [interop.Contract] interface.
func (s *TempStorage) Initialize(ic *interop.Context, hf *config.Hardfork, newMD *interop.HFSpecificContractMD) error {
	return nil
}

// InitializeCache implements the [interop.Contract] interface.
func (s *TempStorage) InitializeCache(_ interop.IsHardforkEnabled, blockHeight uint32, d *dao.Simple) error {
	return nil
}

// ActiveIn implements the [interop.Contract] interface.
func (s *TempStorage) ActiveIn() *config.Hardfork {
	var f = config.HFGorgon // TODO: move to H.
	return &f
}

func (s *TempStorage) put(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	value := toLimitedBytesGeneric(args[1], limits.MaxStorageValueLen)
	ttl := toUint32(args[2])
	if ttl > maxTTL {
		panic(fmt.Errorf("ttl exceeds max limit: %d vs %d", ttl, maxTTL))
	}
	if msPerBlock := s.policy.GetMillisecondsPerBlockInternal(ic.DAO); ttl < 2*msPerBlock { // TempStorage is active after Policy's getMSPerBlock activation.
		panic(fmt.Errorf("ttl is less than 2*msPerBlock: %d vs %d", ttl, msPerBlock))
	}
	/*
		// TODO: @roman-khimov, pricing?
		if err := ic.VM.AddPicoGas(int64(sizeInc) * ttl * ic.BaseTempStorageFee()); err != nil {
			panic(fmt.Errorf("failed to charge temporary storage fee: %w", err))
		}
	*/
	contract, err := ic.GetContract(ic.VM.GetCallingScriptHash())
	if err != nil {
		panic(fmt.Errorf("failed to get calling contract: %w", err))
	}
	recordKey := makeTempRecordKey(contract.ID, key)
	validTill := ic.Block.Timestamp + uint64(ttl)
	s.putRecord(ic.DAO, recordKey, value, validTill)
	return stackitem.Null{}
}

func (s *TempStorage) get(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	res, _ := s.getInternal(ic, ic.VM.GetCallingScriptHash(), key)
	return res
}

func (s *TempStorage) getByHash(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	contractHash := toUint160(args[1])
	res, _ := s.getInternal(ic, contractHash, key)
	return res
}

func (s *TempStorage) getExpiration(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	res, exp := s.getInternal(ic, ic.VM.GetCallingScriptHash(), key)
	return stackitem.NewArray([]stackitem.Item{ // TODO: in case of missing entry return Array{Null, 0} or just Null?
		res,
		stackitem.Make(exp),
	})
}

func (s *TempStorage) getExpirationByHash(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	contractHash := toUint160(args[1])
	res, exp := s.getInternal(ic, contractHash, key)
	return stackitem.NewArray([]stackitem.Item{
		res,
		stackitem.Make(exp),
	})
}

func (s *TempStorage) getInternal(ic *interop.Context, h util.Uint160, key []byte) (stackitem.Item, uint64) {
	contract, err := ic.GetContract(h)
	if err != nil {
		panic(fmt.Errorf("failed to get contract %s: %w", h.StringLE(), err))
	}
	si := ic.DAO.GetStorageItem(s.ID, makeTempRecordKey(contract.ID, key))
	if si == nil {
		return stackitem.Null{}, 0
	}
	validTill, ok := isTraceable(ic, si)
	if !ok {
		return stackitem.Null{}, 0
	}
	return stackitem.NewByteArray(si[8:]), validTill
}

func isTraceable(ic *interop.Context, si state.StorageItem) (uint64, bool) {
	validTill := binary.BigEndian.Uint64(si[:8])
	if validTill < ic.Block.Timestamp {
		return 0, false
	}
	return validTill, true
}

func (s *TempStorage) delete(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	contract, err := ic.GetContract(ic.VM.GetCallingScriptHash())
	if err != nil {
		panic(fmt.Errorf("failed to get calling contract: %w", err))
	}
	recordKey := makeTempRecordKey(contract.ID, key)
	si := ic.DAO.GetStorageItem(s.ID, recordKey)
	if si == nil {
		return stackitem.Null{}
	}
	validTill := si[:8]
	ic.DAO.DeleteStorageItem(s.ID, recordKey)
	ic.DAO.DeleteStorageItem(s.ID, makeValidTillKey(validTill, recordKey))
	return stackitem.Null{}
}

func (s *TempStorage) find(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	prefix := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	opts := toInt64(args[1])
	return s.findInternal(ic, ic.VM.GetCallingScriptHash(), prefix, opts)
}

func (s *TempStorage) findByHash(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	h := toHash160(args[0])
	prefix := toLimitedBytesGeneric(args[1], limits.MaxStorageKeyLen)
	opts := toInt64(args[2])
	return s.findInternal(ic, h, prefix, opts)
}

func (s *TempStorage) findInternal(ic *interop.Context, h util.Uint160, prefix []byte, opts int64) stackitem.Item {
	bkwrds, err := istorage.ValidateFindOptions(opts)
	if err != nil {
		panic(fmt.Errorf("invalid options: %w", err))
	}
	contract, err := ic.GetContract(h)
	if err != nil {
		panic(fmt.Errorf("failed to get contract %s: %w", h.StringLE(), err))
	}
	pref := makeTempRecordKey(contract.ID, prefix)
	ctx, cancel := context.WithCancel(context.Background())

	keep := func(kv storage.KeyValue) (storage.KeyValue, bool) {
		_, ok := isTraceable(ic, kv.Value)
		return storage.KeyValue{
			Key:   kv.Key,
			Value: kv.Value[8:],
		}, ok
	}
	seekres := ic.DAO.SeekAsync(ctx, s.ID, storage.SeekRange{Prefix: pref, Backwards: bkwrds})
	filteredRes := make(chan storage.KeyValue)
	go func() {
	loop:
		for kv := range seekres {
			if kv, ok := keep(kv); ok {
				select {
				case <-ctx.Done():
					break loop
				case filteredRes <- kv:
				}
			}
		}
		close(filteredRes)
	}()

	item := istorage.NewIterator(filteredRes, pref, opts)
	ic.RegisterCancelFunc(func() {
		cancel()
		for range filteredRes { //nolint:revive //empty-block
		}
	})
	return stackitem.NewInterop(item)
}

func (s *TempStorage) renew(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	ttl := toUint32(args[1])
	if ttl > maxTTL {
		panic(fmt.Errorf("ttl exceeds max limit: %d vs %d", ttl, maxTTL))
	}
	// TODO: validation depends on the ttl meaning.
	if msPerBlock := s.policy.GetMillisecondsPerBlockInternal(ic.DAO); ttl < 2*msPerBlock { // TempStorage is active after Policy's getMSPerBlock activation.
		panic(fmt.Errorf("ttl is less than 2*msPerBlock: %d vs %d", ttl, msPerBlock))
	}
	contract, err := ic.GetContract(ic.VM.GetCallingScriptHash())
	if err != nil {
		panic(fmt.Errorf("failed to get calling contract: %w", err))
	}
	recordKey := makeTempRecordKey(contract.ID, key)
	old := ic.DAO.GetStorageItem(s.ID, recordKey)
	if old == nil {
		panic(fmt.Errorf("failed to get old record %s", hex.EncodeToString(recordKey)))
	}
	oldValidTill, ok := isTraceable(ic, old)
	if !ok {
		panic(fmt.Errorf("failed to get old record %s: already expired", hex.EncodeToString(recordKey)))
	}
	validTill := ic.Block.Timestamp + uint64(ttl) // TODO: @roman-khimov, do we want an absolute value instead of delta?
	if validTill < oldValidTill {
		panic(fmt.Errorf("new expiration point should be newer than the old one: %d vs %d", validTill, oldValidTill))
	}
	/*
		// TODO: @roman-khimov, pricing?
		if err := ic.VM.AddPicoGas(int64(sizeInc) * ttl * ic.BaseTempStorageFee()); err != nil {
			panic(fmt.Errorf("failed to charge temporary storage fee: %w", err))
		}
	*/

	ic.DAO.DeleteStorageItem(s.ID, makeValidTillKey(old[:8], recordKey))
	s.putRecord(ic.DAO, recordKey, old[8:], validTill)
	return stackitem.Null{}
}

func makeTempRecordKey(contractID int32, key []byte) []byte {
	buf := make([]byte, 5+len(key)) // TODO: allocate a single buffer for two keys (cap +8), reuse it
	buf[0] = byte(prefixTempStorage)
	binary.LittleEndian.PutUint32(buf[1:], uint32(contractID))
	copy(buf[5:], key)
	return buf
}

// makeValidTillKey returns a key used to store record expiration timestamp. It
// accepts 8 BE bytes of expiration timestamp as the first argument.
func makeValidTillKey(validTill []byte, recordKey []byte) []byte {
	buf := make([]byte, 8+len(recordKey))
	recordKey[0] = prefixTTL
	copy(buf[1:], validTill) // use BigEndian for lexicographic sorting during DB Seek.
	copy(buf[9:], recordKey[1:])
	return buf
}

func (s *TempStorage) putRecord(dao *dao.Simple, recordKey, value []byte, validTill uint64) {
	validTillB := make([]byte, 8)
	binary.BigEndian.PutUint64(validTillB, validTill)
	dao.PutStorageItem(s.ID, recordKey, append(validTillB, value...)) // TODO: @roman-khimov, do we need a separate structure instead of raw bytes?
	dao.PutStorageItem(s.ID, makeValidTillKey(validTillB[:8], recordKey), state.StorageItem{})
}
