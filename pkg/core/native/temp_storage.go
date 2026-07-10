package native

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

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
	// maxCleanupBatchSize is the maximum number of temporary key-value entries
	// that can be removed per a single PostPersist.
	maxCleanupBatchSize = 10_000
)

// TempStorage represents TemporaryStorage native contract.
type TempStorage struct {
	interop.ContractMD
	Policy IPolicy
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
		manifest.NewParameter("validTill", smartcontract.IntegerType))

	md := NewMethodAndPrice(s.put, 1<<15, callflag.WriteStates)
	s.AddMethod(md, desc)

	desc = NewDescriptor("get", smartcontract.ByteArrayType,
		manifest.NewParameter("key", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.get, 1<<15, callflag.ReadStates)
	s.AddMethod(md, desc)

	desc = NewDescriptor("get", smartcontract.ByteArrayType,
		manifest.NewParameter("hash", smartcontract.Hash160Type),
		manifest.NewParameter("key", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.getByHash, 1<<15, callflag.ReadStates)
	s.AddMethod(md, desc)

	desc = NewDescriptor("getExpiration", smartcontract.IntegerType,
		manifest.NewParameter("key", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.getExpiration, 1<<15, callflag.ReadStates)
	s.AddMethod(md, desc)

	desc = NewDescriptor("getExpiration", smartcontract.IntegerType,
		manifest.NewParameter("hash", smartcontract.Hash160Type),
		manifest.NewParameter("key", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.getExpirationByHash, 1<<15, callflag.ReadStates)
	s.AddMethod(md, desc)

	desc = NewDescriptor("delete", smartcontract.VoidType,
		manifest.NewParameter("key", smartcontract.ByteArrayType))
	md = NewMethodAndPrice(s.delete, 1<<15, callflag.WriteStates)
	s.AddMethod(md, desc)

	desc = NewDescriptor("find", smartcontract.InteropInterfaceType,
		manifest.NewParameter("prefix", smartcontract.ByteArrayType),
		manifest.NewParameter("opts", smartcontract.IntegerType))
	md = NewMethodAndPrice(s.find, 1<<15, callflag.ReadStates)
	s.AddMethod(md, desc)

	desc = NewDescriptor("find", smartcontract.InteropInterfaceType,
		manifest.NewParameter("hash", smartcontract.Hash160Type),
		manifest.NewParameter("prefix", smartcontract.ByteArrayType),
		manifest.NewParameter("opts", smartcontract.IntegerType))
	md = NewMethodAndPrice(s.findByHash, 1<<15, callflag.ReadStates)
	s.AddMethod(md, desc)

	desc = NewDescriptor("renew", smartcontract.VoidType,
		manifest.NewParameter("key", smartcontract.ByteArrayType),
		manifest.NewParameter("validTill", smartcontract.IntegerType))
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
	// Remove top maxCleanupBatchSize oldest expired entries, the newer expired
	// ones will also be removed eventually.
	var (
		i     int
		start = make([]byte, 8)
	)
	ic.DAO.Seek(s.ID, storage.SeekRange{
		Prefix: []byte{prefixTTL},
		Start:  start,
	}, func(k, _ []byte) bool {
		if binary.BigEndian.Uint64(k[:8]) >= ic.Block.Timestamp {
			return false
		}

		ic.DAO.DeleteStorageItem(s.ID, append([]byte{prefixTTL}, k...))
		ic.DAO.DeleteStorageItem(s.ID, append([]byte{prefixTempStorage}, k[8:]...))

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
	var f = config.HFHuyao
	return &f
}

func (s *TempStorage) put(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	value := toLimitedBytesGeneric(args[1], limits.MaxStorageValueLen)
	validTill := toUint64(args[2])
	s.checkValidTill(ic, validTill)
	/*
		// TODO: @roman-khimov, pricing?
		if err := ic.VM.AddPicoGas(int64(sizeInc) * ttl * fee); err != nil {
			panic(fmt.Errorf("failed to charge temporary storage fee: %w", err))
		}
	*/
	contract, err := ic.GetContract(ic.VM.GetCallingScriptHash())
	if err != nil {
		panic(fmt.Errorf("failed to get calling contract: %w", err))
	}
	recordKey := makeTempRecordKey(contract.ID, key)
	s.putRecord(ic.DAO, recordKey, value, uint64(validTill))
	return stackitem.Null{}
}

func (s *TempStorage) checkValidTill(ic *interop.Context, validTill uint64) {
	maxValidTill := ic.Block.Timestamp + uint64(s.Policy.GetTemporaryStorageMaxTTLInternal(ic.DAO))
	if validTill > maxValidTill {
		panic(fmt.Errorf("validTill exceeds max limit: %d vs %d", validTill, maxValidTill))
	}
	minValidTill := ic.Block.Timestamp + uint64(2*s.Policy.GetMillisecondsPerBlockInternal(ic.DAO)) // TempStorage is active after Policy's getMSPerBlock activation.
	if validTill < minValidTill {
		panic(fmt.Errorf("item is valid for less than 2*msPerBlock: %d vs %d", validTill, minValidTill))
	}
}

func (s *TempStorage) get(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	res, _ := s.getInternal(ic, ic.VM.GetCallingScriptHash(), key)
	return res
}

func (s *TempStorage) getByHash(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	contractHash := toUint160(args[0])
	key := toLimitedBytesGeneric(args[1], limits.MaxStorageKeyLen)
	res, _ := s.getInternal(ic, contractHash, key)
	return res
}

func (s *TempStorage) getExpiration(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	key := toLimitedBytesGeneric(args[0], limits.MaxStorageKeyLen)
	_, exp := s.getInternal(ic, ic.VM.GetCallingScriptHash(), key)
	return stackitem.Make(exp)
}

func (s *TempStorage) getExpirationByHash(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	contractHash := toUint160(args[0])
	key := toLimitedBytesGeneric(args[1], limits.MaxStorageKeyLen)
	_, exp := s.getInternal(ic, contractHash, key)
	return stackitem.Make(exp)
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
	validTill := toUint64(args[1])
	s.checkValidTill(ic, validTill)
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
	if validTill < oldValidTill {
		panic(fmt.Errorf("new expiration point should be newer than the old one: %d vs %d", validTill, oldValidTill))
	}
	/*
		// TODO: @roman-khimov, pricing?
		if err := ic.VM.AddPicoGas(int64(sizeInc) * ttl * fee); err != nil {
			panic(fmt.Errorf("failed to charge temporary storage fee: %w", err))
		}
	*/

	ic.DAO.DeleteStorageItem(s.ID, makeValidTillKey(old[:8], recordKey))
	s.putRecord(ic.DAO, recordKey, old[8:], validTill)
	return stackitem.Null{}
}

func makeTempRecordKey(contractID int32, key []byte) []byte {
	buf := make([]byte, 5+len(key))
	buf[0] = byte(prefixTempStorage)
	binary.LittleEndian.PutUint32(buf[1:], uint32(contractID))
	copy(buf[5:], key)
	return buf
}

// makeValidTillKey returns a key used to store record expiration timestamp. It
// accepts 8 BE bytes of expiration timestamp as the first argument.
func makeValidTillKey(validTill []byte, recordKey []byte) []byte {
	buf := make([]byte, 8+len(recordKey))
	buf[0] = prefixTTL
	copy(buf[1:], validTill) // use BigEndian for lexicographic sorting during DB Seek.
	copy(buf[9:], recordKey[1:])
	return buf
}

func (s *TempStorage) putRecord(dao *dao.Simple, recordKey, value []byte, validTill uint64) {
	validTillB := make([]byte, 8)
	binary.BigEndian.PutUint64(validTillB, validTill)
	dao.PutStorageItem(s.ID, recordKey, append(validTillB, value...))
	dao.PutStorageItem(s.ID, makeValidTillKey(validTillB[:8], recordKey), state.StorageItem{})
}
