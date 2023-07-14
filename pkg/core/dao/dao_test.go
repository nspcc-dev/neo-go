package dao

import (
	"encoding/binary"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestPutGetAndDecode(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), false, false)
	serializable := &TestSerializable{field: random.String(4)}
	hash := []byte{1}
	require.NoError(t, dao.putWithBuffer(serializable, hash, io.NewBufBinWriter()))

	gotAndDecoded := &TestSerializable{}
	err := dao.GetAndDecode(gotAndDecoded, hash)
	require.NoError(t, err)
}

// TestSerializable structure used in testing.
type TestSerializable struct {
	field string
}

func (t *TestSerializable) EncodeBinary(writer *io.BinWriter) {
	writer.WriteString(t.field)
}

func (t *TestSerializable) DecodeBinary(reader *io.BinReader) {
	t.field = reader.ReadString()
}

func TestPutGetStorageItem(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), false, false)
	id := int32(random.Int(0, 1024))
	key := []byte{0}
	storageItem := state.StorageItem{}
	dao.PutStorageItem(id, key, storageItem)
	gotStorageItem := dao.GetStorageItem(id, key)
	require.Equal(t, storageItem, gotStorageItem)
}

func TestDeleteStorageItem(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), false, false)
	id := int32(random.Int(0, 1024))
	key := []byte{0}
	storageItem := state.StorageItem{}
	dao.PutStorageItem(id, key, storageItem)
	dao.DeleteStorageItem(id, key)
	gotStorageItem := dao.GetStorageItem(id, key)
	require.Nil(t, gotStorageItem)
}

func TestGetBlock_NotExists(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), false, false)
	hash := random.Uint256()
	block, err := dao.GetBlock(hash)
	require.Error(t, err)
	require.Nil(t, block)
}

func TestPutGetBlock(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), false, false)
	b := &block.Block{
		Header: block.Header{
			Script: transaction.Witness{
				VerificationScript: []byte{byte(opcode.PUSH1)},
				InvocationScript:   []byte{byte(opcode.NOP)},
			},
		},
	}
	hash := b.Hash()
	appExecResult1 := &state.AppExecResult{
		Container: hash,
		Execution: state.Execution{
			Trigger: trigger.OnPersist,
			Events:  []state.NotificationEvent{},
			Stack:   []stackitem.Item{},
		},
	}
	appExecResult2 := &state.AppExecResult{
		Container: hash,
		Execution: state.Execution{
			Trigger: trigger.PostPersist,
			Events:  []state.NotificationEvent{},
			Stack:   []stackitem.Item{},
		},
	}
	err := dao.StoreAsBlock(b, appExecResult1, appExecResult2)
	require.NoError(t, err)
	gotBlock, err := dao.GetBlock(hash)
	require.NoError(t, err)
	require.NotNil(t, gotBlock)
	gotAppExecResult, err := dao.GetAppExecResults(hash, trigger.All)
	require.NoError(t, err)
	require.Equal(t, 2, len(gotAppExecResult))
	require.Equal(t, *appExecResult1, gotAppExecResult[0])
	require.Equal(t, *appExecResult2, gotAppExecResult[1])
}

func TestGetVersion_NoVersion(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), false, false)
	version, err := dao.GetVersion()
	require.Error(t, err)
	require.Equal(t, "", version.Value)
}

func TestGetVersion(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), false, false)
	expected := Version{
		StoragePrefix:     0x42,
		P2PSigExtensions:  true,
		StateRootInHeader: true,
		Value:             "testVersion",
	}
	dao.PutVersion(expected)
	actual, err := dao.GetVersion()
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	t.Run("invalid", func(t *testing.T) {
		dao := NewSimple(storage.NewMemoryStore(), false, false)
		dao.Store.Put([]byte{byte(storage.SYSVersion)}, []byte("0.1.2\x00x"))

		_, err := dao.GetVersion()
		require.Error(t, err)
	})
	t.Run("old format", func(t *testing.T) {
		dao := NewSimple(storage.NewMemoryStore(), false, false)
		dao.Store.Put([]byte{byte(storage.SYSVersion)}, []byte("0.1.2"))

		version, err := dao.GetVersion()
		require.NoError(t, err)
		require.Equal(t, "0.1.2", version.Value)
	})
}

func TestGetCurrentHeaderHeight_NoHeader(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), false, false)
	height, err := dao.GetCurrentBlockHeight()
	require.Error(t, err)
	require.Equal(t, uint32(0), height)
}

func TestGetCurrentHeaderHeight_Store(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), false, false)
	b := &block.Block{
		Header: block.Header{
			Script: transaction.Witness{
				VerificationScript: []byte{byte(opcode.PUSH1)},
				InvocationScript:   []byte{byte(opcode.NOP)},
			},
		},
	}
	dao.StoreAsCurrentBlock(b)
	height, err := dao.GetCurrentBlockHeight()
	require.NoError(t, err)
	require.Equal(t, uint32(0), height)
}

func TestStoreAsTransaction(t *testing.T) {
	t.Run("P2PSigExtensions off", func(t *testing.T) {
		dao := NewSimple(storage.NewMemoryStore(), false, false)
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 1)
		tx.Signers = append(tx.Signers, transaction.Signer{})
		tx.Scripts = append(tx.Scripts, transaction.Witness{})
		hash := tx.Hash()
		aer := &state.AppExecResult{
			Container: hash,
			Execution: state.Execution{
				Trigger: trigger.Application,
				Events:  []state.NotificationEvent{},
				Stack:   []stackitem.Item{},
			},
		}
		err := dao.StoreAsTransaction(tx, 0, aer)
		require.NoError(t, err)
		err = dao.HasTransaction(hash, nil)
		require.ErrorIs(t, err, ErrAlreadyExists)
		gotAppExecResult, err := dao.GetAppExecResults(hash, trigger.All)
		require.NoError(t, err)
		require.Equal(t, 1, len(gotAppExecResult))
		require.Equal(t, *aer, gotAppExecResult[0])
	})

	t.Run("P2PSigExtensions on", func(t *testing.T) {
		dao := NewSimple(storage.NewMemoryStore(), false, true)
		conflictsH := util.Uint256{1, 2, 3}
		signer1 := util.Uint160{1, 2, 3}
		signer2 := util.Uint160{4, 5, 6}
		signer3 := util.Uint160{7, 8, 9}
		signerMalicious := util.Uint160{10, 11, 12}
		tx1 := transaction.New([]byte{byte(opcode.PUSH1)}, 1)
		tx1.Signers = append(tx1.Signers, transaction.Signer{Account: signer1}, transaction.Signer{Account: signer2})
		tx1.Scripts = append(tx1.Scripts, transaction.Witness{}, transaction.Witness{})
		tx1.Attributes = []transaction.Attribute{
			{
				Type:  transaction.ConflictsT,
				Value: &transaction.Conflicts{Hash: conflictsH},
			},
		}
		hash1 := tx1.Hash()
		tx2 := transaction.New([]byte{byte(opcode.PUSH1)}, 1)
		tx2.Signers = append(tx2.Signers, transaction.Signer{Account: signer3})
		tx2.Scripts = append(tx2.Scripts, transaction.Witness{})
		tx2.Attributes = []transaction.Attribute{
			{
				Type:  transaction.ConflictsT,
				Value: &transaction.Conflicts{Hash: conflictsH},
			},
		}
		hash2 := tx2.Hash()
		aer1 := &state.AppExecResult{
			Container: hash1,
			Execution: state.Execution{
				Trigger: trigger.Application,
				Events:  []state.NotificationEvent{},
				Stack:   []stackitem.Item{},
			},
		}
		err := dao.StoreAsTransaction(tx1, 0, aer1)
		require.NoError(t, err)
		aer2 := &state.AppExecResult{
			Container: hash2,
			Execution: state.Execution{
				Trigger: trigger.Application,
				Events:  []state.NotificationEvent{},
				Stack:   []stackitem.Item{},
			},
		}
		err = dao.StoreAsTransaction(tx2, 0, aer2)
		require.NoError(t, err)
		err = dao.HasTransaction(hash1, nil)
		require.ErrorIs(t, err, ErrAlreadyExists)
		err = dao.HasTransaction(hash2, nil)
		require.ErrorIs(t, err, ErrAlreadyExists)

		// Conflicts: unimportant payer.
		err = dao.HasTransaction(conflictsH, nil)
		require.ErrorIs(t, err, ErrHasConflicts)

		// Conflicts: payer is important, conflict isn't malicious, test signer #1.
		err = dao.HasTransaction(conflictsH, map[util.Uint160]struct{}{signer1: {}})
		require.ErrorIs(t, err, ErrHasConflicts)

		// Conflicts: payer is important, conflict isn't malicious, test signer #2.
		err = dao.HasTransaction(conflictsH, map[util.Uint160]struct{}{signer2: {}})
		require.ErrorIs(t, err, ErrHasConflicts)

		// Conflicts: payer is important, conflict isn't malicious, test signer #3.
		err = dao.HasTransaction(conflictsH, map[util.Uint160]struct{}{signer3: {}})
		require.ErrorIs(t, err, ErrHasConflicts)

		// Conflicts: payer is important, conflict is malicious.
		err = dao.HasTransaction(conflictsH, map[util.Uint160]struct{}{signerMalicious: {}})
		require.NoError(t, err)

		gotAppExecResult, err := dao.GetAppExecResults(hash1, trigger.All)
		require.NoError(t, err)
		require.Equal(t, 1, len(gotAppExecResult))
		require.Equal(t, *aer1, gotAppExecResult[0])

		gotAppExecResult, err = dao.GetAppExecResults(hash2, trigger.All)
		require.NoError(t, err)
		require.Equal(t, 1, len(gotAppExecResult))
		require.Equal(t, *aer2, gotAppExecResult[0])
	})
}

func BenchmarkStoreAsTransaction(b *testing.B) {
	dao := NewSimple(storage.NewMemoryStore(), false, true)
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 1)
	tx.Attributes = []transaction.Attribute{
		{
			Type: transaction.ConflictsT,
			Value: &transaction.Conflicts{
				Hash: util.Uint256{1, 2, 3},
			},
		},
		{
			Type: transaction.ConflictsT,
			Value: &transaction.Conflicts{
				Hash: util.Uint256{4, 5, 6},
			},
		},
		{
			Type: transaction.ConflictsT,
			Value: &transaction.Conflicts{
				Hash: util.Uint256{7, 8, 9},
			},
		},
	}
	_ = tx.Hash()
	aer := &state.AppExecResult{
		Container: tx.Hash(),
		Execution: state.Execution{
			Trigger: trigger.Application,
			Events:  []state.NotificationEvent{},
			Stack:   []stackitem.Item{},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		err := dao.StoreAsTransaction(tx, 1, aer)
		if err != nil {
			b.FailNow()
		}
	}
}

func TestMakeStorageItemKey(t *testing.T) {
	var id int32 = 5

	dao := NewSimple(storage.NewMemoryStore(), true, false)

	expected := []byte{byte(storage.STStorage), 0, 0, 0, 0, 1, 2, 3}
	binary.LittleEndian.PutUint32(expected[1:5], uint32(id))
	actual := dao.makeStorageItemKey(id, []byte{1, 2, 3})
	require.Equal(t, expected, actual)

	expected = expected[0:5]
	actual = dao.makeStorageItemKey(id, nil)
	require.Equal(t, expected, actual)

	expected = []byte{byte(storage.STTempStorage), 0, 0, 0, 0, 1, 2, 3}
	binary.LittleEndian.PutUint32(expected[1:5], uint32(id))
	dao.Version.StoragePrefix = storage.STTempStorage
	actual = dao.makeStorageItemKey(id, []byte{1, 2, 3})
	require.Equal(t, expected, actual)
}

func TestPutGetStateSyncPoint(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), true, false)

	// empty store
	_, err := dao.GetStateSyncPoint()
	require.Error(t, err)

	// non-empty store
	var expected uint32 = 5
	dao.PutStateSyncPoint(expected)
	actual, err := dao.GetStateSyncPoint()
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestPutGetStateSyncCurrentBlockHeight(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), true, false)

	// empty store
	_, err := dao.GetStateSyncCurrentBlockHeight()
	require.Error(t, err)

	// non-empty store
	var expected uint32 = 5
	dao.PutStateSyncCurrentBlockHeight(expected)
	actual, err := dao.GetStateSyncCurrentBlockHeight()
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
