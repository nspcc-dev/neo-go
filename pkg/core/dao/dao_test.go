package dao

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
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

func TestPutAndGetAssetState(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	id := random.Uint256()
	assetState := &state.Asset{ID: id, Owner: keys.PublicKey{}}
	err := dao.PutAssetState(assetState)
	require.NoError(t, err)
	gotAssetState, err := dao.GetAssetState(id)
	require.NoError(t, err)
	require.Equal(t, assetState, gotAssetState)
}

func TestPutAndGetContractState(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	contractState := &state.Contract{Script: []byte{}, ParamList: []smartcontract.ParamType{}}
	hash := contractState.ScriptHash()
	err := dao.PutContractState(contractState)
	require.NoError(t, err)
	gotContractState, err := dao.GetContractState(hash)
	require.NoError(t, err)
	require.Equal(t, contractState, gotContractState)
}

func TestDeleteContractState(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	contractState := &state.Contract{Script: []byte{}, ParamList: []smartcontract.ParamType{}}
	hash := contractState.ScriptHash()
	err := dao.PutContractState(contractState)
	require.NoError(t, err)
	err = dao.DeleteContractState(hash)
	require.NoError(t, err)
	gotContractState, err := dao.GetContractState(hash)
	require.Error(t, err)
	require.Nil(t, gotContractState)
}

func TestGetUnspentCoinState_Err(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	hash := random.Uint256()
	gotUnspentCoinState, err := dao.GetUnspentCoinState(hash)
	require.Error(t, err)
	require.Nil(t, gotUnspentCoinState)
}

func TestPutGetUnspentCoinState(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	hash := random.Uint256()
	unspentCoinState := &state.UnspentCoin{Height: 42, States: []state.OutputState{}}
	err := dao.PutUnspentCoinState(hash, unspentCoinState)
	require.NoError(t, err)
	gotUnspentCoinState, err := dao.GetUnspentCoinState(hash)
	require.NoError(t, err)
	require.Equal(t, unspentCoinState, gotUnspentCoinState)
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

func TestPutGetStorageItems(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	hash := random.Uint160()
	for i := 3; i > 0; i-- {
		key := []byte{1, byte(i)}
		storageItem := &state.StorageItem{Value: []uint8{}}
		err := dao.PutStorageItem(hash, key, storageItem)
		require.NoError(t, err)
	}
	err := dao.PutStorageItem(random.Uint160(), []byte{1}, &state.StorageItem{Value: []uint8{}})
	require.NoError(t, err)
	foundItems, err := dao.GetStorageItems(hash, false)
	require.NoError(t, err)
	require.Equal(t, 3, len(foundItems))
	foundItems, err = dao.GetStorageItems(hash, true)
	require.NoError(t, err)
	require.Equal(t, 3, len(foundItems))
	// items should be sorted by key
	for i, item := range foundItems {
		require.Equal(t, []byte{1, byte(i + 1)}, item.Key)
	}
}

func TestPutGetStorageItemsWithPrefix(t *testing.T) {
	dao := NewSimple(storage.NewMemoryStore())
	hash := random.Uint160()
	for i := 3; i > 0; i-- {
		key := []byte{1, byte(i)}
		storageItem := &state.StorageItem{Value: []uint8{}}
		err := dao.PutStorageItem(hash, key, storageItem)
		require.NoError(t, err)
	}
	err := dao.PutStorageItem(hash, []byte{2}, &state.StorageItem{Value: []uint8{}})
	require.NoError(t, err)
	foundItems, err := dao.GetStorageItemsWithPrefix(hash, []byte{1}, true)
	require.NoError(t, err)
	require.Equal(t, 3, len(foundItems))
	for i, item := range foundItems {
		require.Equal(t, []byte{byte(i + 1)}, item.Key)
	}
	// getting item shouldn't change the result order
	item := dao.GetStorageItem(hash, []byte{1, 2})
	require.Equal(t, &state.StorageItem{
		Value:   []uint8{},
		IsConst: false,
	}, item)
	foundItems, err = dao.GetStorageItemsWithPrefix(hash, []byte{1}, true)
	require.NoError(t, err)
	require.Equal(t, 3, len(foundItems))
	for i, item := range foundItems {
		require.Equal(t, []byte{byte(i + 1)}, item.Key)
	}
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
	block, _, err := dao.GetBlock(hash)
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
	err := dao.StoreAsBlock(b, 42)
	require.NoError(t, err)
	gotBlock, sysfee, err := dao.GetBlock(hash)
	require.NoError(t, err)
	require.NotNil(t, gotBlock)
	require.EqualValues(t, 42, sysfee)
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
	tx := transaction.NewIssueTX()
	hash := tx.Hash()
	err := dao.StoreAsTransaction(tx, 0)
	require.NoError(t, err)
	hasTransaction := dao.HasTransaction(hash)
	require.True(t, hasTransaction)
}
