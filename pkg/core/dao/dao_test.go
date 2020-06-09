package dao

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestPutGetAndDecode(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
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

func TestGetAccountStateOrNew_New(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	hash := random.Uint160()
	createdAccount, err := dao.GetAccountStateOrNew(hash)
	require.NoError(t, err)
	require.NotNil(t, createdAccount)
}

func TestPutAndGetAccountStateOrNew(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	hash := random.Uint160()
	accountState := &state.Account{ScriptHash: hash}
	err := dao.PutAccountState(accountState)
	require.NoError(t, err)
	gotAccount, err := dao.GetAccountStateOrNew(hash)
	require.NoError(t, err)
	require.Equal(t, accountState.ScriptHash, gotAccount.ScriptHash)
}

func TestPutAndGetContractState(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	contractState := &state.Contract{Script: []byte{}}
	hash := contractState.ScriptHash()
	err := dao.PutContractState(contractState)
	require.NoError(t, err)
	gotContractState, err := dao.GetContractState(hash)
	require.NoError(t, err)
	require.Equal(t, contractState, gotContractState)
}

func TestDeleteContractState(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	contractState := &state.Contract{Script: []byte{}}
	hash := contractState.ScriptHash()
	err := dao.PutContractState(contractState)
	require.NoError(t, err)
	err = dao.DeleteContractState(hash)
	require.NoError(t, err)
	gotContractState, err := dao.GetContractState(hash)
	require.Error(t, err)
	require.Nil(t, gotContractState)
}

func TestPutGetAppExecResult(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	hash := random.Uint256()
	appExecResult := &state.AppExecResult{
		TxHash: hash,
		Events: []state.NotificationEvent{},
		Stack:  []smartcontract.Parameter{},
	}
	err := dao.PutAppExecResult(appExecResult)
	require.NoError(t, err)
	gotAppExecResult, err := dao.GetAppExecResult(hash)
	require.NoError(t, err)
	require.Equal(t, appExecResult, gotAppExecResult)
}

func TestPutGetStorageItem(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	hash := random.Uint160()
	key := []byte{0}
	storageItem := &state.StorageItem{Value: []uint8{}}
	err := dao.PutStorageItem(hash, key, storageItem)
	require.NoError(t, err)
	gotStorageItem := dao.GetStorageItem(hash, key)
	require.Equal(t, storageItem, gotStorageItem)
}

func TestDeleteStorageItem(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	hash := random.Uint160()
	key := []byte{0}
	storageItem := &state.StorageItem{Value: []uint8{}}
	err := dao.PutStorageItem(hash, key, storageItem)
	require.NoError(t, err)
	err = dao.DeleteStorageItem(hash, key)
	require.NoError(t, err)
	gotStorageItem := dao.GetStorageItem(hash, key)
	require.Nil(t, gotStorageItem)
}

func TestGetBlock_NotExists(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	hash := random.Uint256()
	block, err := dao.GetBlock(hash)
	require.Error(t, err)
	require.Nil(t, block)
}

func TestPutGetBlock(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	b := &block.Block{
		Base: block.Base{
			Script: transaction.Witness{
				VerificationScript: []byte{byte(opcode.PUSH1)},
				InvocationScript:   []byte{byte(opcode.NOP)},
			},
		},
	}
	hash := b.Hash()
	err := dao.StoreAsBlock(b)
	require.NoError(t, err)
	gotBlock, err := dao.GetBlock(hash)
	require.NoError(t, err)
	require.NotNil(t, gotBlock)
}

func TestGetVersion_NoVersion(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	version, err := dao.GetVersion()
	require.Error(t, err)
	require.Equal(t, "", version)
}

func TestGetVersion(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	err := dao.PutVersion("testVersion")
	require.NoError(t, err)
	version, err := dao.GetVersion()
	require.NoError(t, err)
	require.NotNil(t, version)
}

func TestGetCurrentHeaderHeight_NoHeader(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	height, err := dao.GetCurrentBlockHeight()
	require.Error(t, err)
	require.Equal(t, uint32(0), height)
}

func TestGetCurrentHeaderHeight_Store(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	b := &block.Block{
		Base: block.Base{
			Script: transaction.Witness{
				VerificationScript: []byte{byte(opcode.PUSH1)},
				InvocationScript:   []byte{byte(opcode.NOP)},
			},
		},
	}
	err := dao.StoreAsCurrentBlock(b)
	require.NoError(t, err)
	height, err := dao.GetCurrentBlockHeight()
	require.NoError(t, err)
	require.Equal(t, uint32(0), height)
}

func TestStoreAsTransaction(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 1)
	hash := tx.Hash()
	err := dao.StoreAsTransaction(tx, 0)
	require.NoError(t, err)
	hasTransaction := dao.HasTransaction(hash)
	require.True(t, hasTransaction)
}
