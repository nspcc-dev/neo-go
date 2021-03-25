package dao

import (
	"encoding/binary"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestPutGetAndDecode(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	serializable := &TestSerializable{field: random.String(4)}
	hash := []byte{1}
	err := dao.Put(serializable, hash)
	require.NoError(t, err)

	gotAndDecoded := &TestSerializable{}
	err = dao.GetAndDecode(gotAndDecoded, hash)
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

func TestPutGetAppExecResult(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	hash := random.Uint256()
	appExecResult := &state.AppExecResult{
		Container: hash,
		Execution: state.Execution{
			Trigger: trigger.Application,
			Events:  []state.NotificationEvent{},
			Stack:   []stackitem.Item{},
		},
	}
	err := dao.AppendAppExecResult(appExecResult, nil)
	require.NoError(t, err)
	gotAppExecResult, err := dao.GetAppExecResults(hash, trigger.All)
	require.NoError(t, err)
	require.Equal(t, []state.AppExecResult{*appExecResult}, gotAppExecResult)
}

func TestPutGetStorageItem(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	id := int32(random.Int(0, 1024))
	key := []byte{0}
	storageItem := state.StorageItem{}
	err := dao.PutStorageItem(id, key, storageItem)
	require.NoError(t, err)
	gotStorageItem := dao.GetStorageItem(id, key)
	require.Equal(t, storageItem, gotStorageItem)
}

func TestDeleteStorageItem(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	id := int32(random.Int(0, 1024))
	key := []byte{0}
	storageItem := state.StorageItem{}
	err := dao.PutStorageItem(id, key, storageItem)
	require.NoError(t, err)
	err = dao.DeleteStorageItem(id, key)
	require.NoError(t, err)
	gotStorageItem := dao.GetStorageItem(id, key)
	require.Nil(t, gotStorageItem)
}

func TestGetBlock_NotExists(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	hash := random.Uint256()
	block, err := dao.GetBlock(hash)
	require.Error(t, err)
	require.Nil(t, block)
}

func TestPutGetBlock(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	b := &block.Block{
		Header: block.Header{
			Script: transaction.Witness{
				VerificationScript: []byte{byte(opcode.PUSH1)},
				InvocationScript:   []byte{byte(opcode.NOP)},
			},
		},
	}
	hash := b.Hash()
	err := dao.StoreAsBlock(b, nil)
	require.NoError(t, err)
	gotBlock, err := dao.GetBlock(hash)
	require.NoError(t, err)
	require.NotNil(t, gotBlock)
}

func TestGetVersion_NoVersion(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	version, err := dao.GetVersion()
	require.Error(t, err)
	require.Equal(t, "", version)
}

func TestGetVersion(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	err := dao.PutVersion("testVersion")
	require.NoError(t, err)
	version, err := dao.GetVersion()
	require.NoError(t, err)
	require.NotNil(t, version)
}

func TestGetCurrentHeaderHeight_NoHeader(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	height, err := dao.GetCurrentBlockHeight()
	require.Error(t, err)
	require.Equal(t, uint32(0), height)
}

func TestGetCurrentHeaderHeight_Store(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	b := &block.Block{
		Header: block.Header{
			Script: transaction.Witness{
				VerificationScript: []byte{byte(opcode.PUSH1)},
				InvocationScript:   []byte{byte(opcode.NOP)},
			},
		},
	}
	err := dao.StoreAsCurrentBlock(b, nil)
	require.NoError(t, err)
	height, err := dao.GetCurrentBlockHeight()
	require.NoError(t, err)
	require.Equal(t, uint32(0), height)
}

func TestStoreAsTransaction(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, false)
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 1)
	hash := tx.Hash()
	err := dao.StoreAsTransaction(tx, 0, nil)
	require.NoError(t, err)
	err = dao.HasTransaction(hash)
	require.NotNil(t, err)
}

func TestMakeStorageItemKey(t *testing.T) {
	var id int32 = 5

	expected := []byte{byte(storage.STStorage), 0, 0, 0, 0, 1, 2, 3}
	binary.LittleEndian.PutUint32(expected[1:5], uint32(id))
	actual := makeStorageItemKey(id, []byte{1, 2, 3})
	require.Equal(t, expected, actual)

	expected = expected[0:5]
	actual = makeStorageItemKey(id, nil)
	require.Equal(t, expected, actual)
}
