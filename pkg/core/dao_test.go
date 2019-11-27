package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/entities"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/testutil"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestPutGetAndDecode(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	serializable := &TestSerializable{field: testutil.RandomString(4)}
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
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint160()
	createdAccount, err := dao.GetAccountStateOrNew(hash)
	require.NoError(t, err)
	require.NotNil(t, createdAccount)
	gotAccount, err := dao.GetAccountState(hash)
	require.NoError(t, err)
	require.Equal(t, createdAccount, gotAccount)
}

func TestPutAndGetAccountStateOrNew(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint160()
	accountState := &entities.AccountState{ScriptHash: hash}
	err := dao.PutAccountState(accountState)
	require.NoError(t, err)
	gotAccount, err := dao.GetAccountStateOrNew(hash)
	require.NoError(t, err)
	require.Equal(t, accountState.ScriptHash, gotAccount.ScriptHash)
}

func TestPutAndGetAssetState(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	id := testutil.RandomUint256()
	assetState := &entities.AssetState{ID: id, Owner: keys.PublicKey{}}
	err := dao.PutAssetState(assetState)
	require.NoError(t, err)
	gotAssetState, err := dao.GetAssetState(id)
	require.NoError(t, err)
	require.Equal(t, assetState, gotAssetState)
}

func TestPutAndGetContractState(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	contractState := &entities.ContractState{Script: []byte{}, ParamList:[]smartcontract.ParamType{}}
	hash := contractState.ScriptHash()
	err := dao.PutContractState(contractState)
	require.NoError(t, err)
	gotContractState, err := dao.GetContractState(hash)
	require.NoError(t, err)
	require.Equal(t, contractState, gotContractState)
}

func TestDeleteContractState(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	contractState := &entities.ContractState{Script: []byte{}, ParamList:[]smartcontract.ParamType{}}
	hash := contractState.ScriptHash()
	err := dao.PutContractState(contractState)
	require.NoError(t, err)
	err = dao.DeleteContractState(hash)
	require.NoError(t, err)
	gotContractState, err := dao.GetContractState(hash)
	require.Error(t, err)
	require.Nil(t, gotContractState)
}

func TestGetUnspentCoinStateOrNew_New(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint256()
	unspentCoinState, err := dao.GetUnspentCoinStateOrNew(hash)
	require.NoError(t, err)
	require.NotNil(t, unspentCoinState)
	gotUnspentCoinState, err := dao.GetUnspentCoinState(hash)
	require.NoError(t, err)
	require.Equal(t, unspentCoinState, gotUnspentCoinState)
}

func TestGetUnspentCoinState_Err(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint256()
	gotUnspentCoinState, err := dao.GetUnspentCoinState(hash)
	require.Error(t, err)
	require.Nil(t, gotUnspentCoinState)
}

func TestPutGetUnspentCoinState(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint256()
	unspentCoinState := &UnspentCoinState{states:[]entities.CoinState{}}
	err := dao.PutUnspentCoinState(hash, unspentCoinState)
	require.NoError(t, err)
	gotUnspentCoinState, err := dao.GetUnspentCoinState(hash)
	require.NoError(t, err)
	require.Equal(t, unspentCoinState, gotUnspentCoinState)
}

func TestGetSpentCoinStateOrNew_New(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint256()
	spentCoinState, err := dao.GetSpentCoinsOrNew(hash)
	require.NoError(t, err)
	require.NotNil(t, spentCoinState)
	gotSpentCoinState, err := dao.GetSpentCoinState(hash)
	require.NoError(t, err)
	require.Equal(t, spentCoinState, gotSpentCoinState)
}

func TestPutAndGetSpentCoinState(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint256()
	spentCoinState := &SpentCoinState{items:make(map[uint16]uint32)}
	err := dao.PutSpentCoinState(hash, spentCoinState)
	require.NoError(t, err)
	gotSpentCoinState, err := dao.GetSpentCoinState(hash)
	require.NoError(t, err)
	require.Equal(t, spentCoinState, gotSpentCoinState)
}

func TestGetSpentCoinState_Err(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint256()
	spentCoinState, err := dao.GetSpentCoinState(hash)
	require.Error(t, err)
	require.Nil(t, spentCoinState)
}

func TestDeleteSpentCoinState(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint256()
	spentCoinState := &SpentCoinState{items:make(map[uint16]uint32)}
	err := dao.PutSpentCoinState(hash, spentCoinState)
	require.NoError(t, err)
	err = dao.DeleteSpentCoinState(hash)
	require.NoError(t, err)
	gotSpentCoinState, err := dao.GetSpentCoinState(hash)
	require.Error(t, err)
	require.Nil(t, gotSpentCoinState)
}

func TestGetValidatorStateOrNew_New(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	publicKey := &keys.PublicKey{}
	validatorState, err := dao.GetValidatorStateOrNew(publicKey)
	require.NoError(t, err)
	require.NotNil(t, validatorState)
	gotValidatorState, err := dao.GetValidatorState(publicKey)
	require.NoError(t, err)
	require.Equal(t, validatorState, gotValidatorState)
}

func TestPutGetValidatorState(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	publicKey := &keys.PublicKey{}
	validatorState := &entities.ValidatorState{
		PublicKey:  publicKey,
		Registered: false,
		Votes:      0,
	}
	err := dao.PutValidatorState(validatorState)
	require.NoError(t, err)
	gotValidatorState, err := dao.GetValidatorState(publicKey)
	require.NoError(t, err)
	require.Equal(t, validatorState, gotValidatorState)
}

func TestDeleteValidatorState(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	publicKey := &keys.PublicKey{}
	validatorState := &entities.ValidatorState{
		PublicKey:  publicKey,
		Registered: false,
		Votes:      0,
	}
	err := dao.PutValidatorState(validatorState)
	require.NoError(t, err)
	err = dao.DeleteValidatorState(validatorState)
	require.NoError(t, err)
	gotValidatorState, err := dao.GetValidatorState(publicKey)
	require.Error(t, err)
	require.Nil(t, gotValidatorState)
}

func TestGetValidators(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	publicKey := &keys.PublicKey{}
	validatorState := &entities.ValidatorState{
		PublicKey:  publicKey,
		Registered: false,
		Votes:      0,
	}
	err := dao.PutValidatorState(validatorState)
	require.NoError(t, err)
	validators := dao.GetValidators()
	require.Equal(t, validatorState, validators[0])
	require.Len(t, validators, 1)
}

func TestPutGetAppExecResult(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint256()
	appExecResult := &entities.AppExecResult{TxHash: hash, Events:[]entities.NotificationEvent{}}
	err := dao.PutAppExecResult(appExecResult)
	require.NoError(t, err)
	gotAppExecResult, err := dao.GetAppExecResult(hash)
	require.NoError(t, err)
	require.Equal(t, appExecResult, gotAppExecResult)
}

func TestPutGetStorageItem(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint160()
	key := []byte{0}
	storageItem := &entities.StorageItem{Value:[]uint8{}}
	err := dao.PutStorageItem(hash, key, storageItem)
	require.NoError(t, err)
	gotStorageItem := dao.GetStorageItem(hash, key)
	require.Equal(t, storageItem, gotStorageItem)
}

func TestDeleteStorageItem(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint160()
	key := []byte{0}
	storageItem := &entities.StorageItem{Value:[]uint8{}}
	err := dao.PutStorageItem(hash, key, storageItem)
	require.NoError(t, err)
	err = dao.DeleteStorageItem(hash, key)
	require.NoError(t, err)
	gotStorageItem := dao.GetStorageItem(hash, key)
	require.Nil(t, gotStorageItem)
}

func TestGetBlock_NotExists(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	hash := testutil.RandomUint256()
	block, err := dao.GetBlock(hash)
	require.Error(t, err)
	require.Nil(t, block)
}

func TestPutGetBlock(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	block := &Block{
		BlockBase: BlockBase{
			Script: transaction.Witness{
				VerificationScript: []byte{byte(opcode.PUSH1)},
				InvocationScript:   []byte{byte(opcode.NOP)},
			},
		},
	}
	hash := block.Hash()
	err := dao.StoreAsBlock(block, 0)
	require.NoError(t, err)
	gotBlock, err := dao.GetBlock(hash)
	require.NoError(t, err)
	require.NotNil(t, gotBlock)
}

func TestGetVersion_NoVersion(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	version, err := dao.GetVersion()
	require.Error(t, err)
	require.Equal(t, "", version)
}

func TestGetVersion(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	err := dao.PutVersion("testVersion")
	require.NoError(t, err)
	version, err := dao.GetVersion()
	require.NoError(t, err)
	require.NotNil(t, version)
}

func TestGetCurrentHeaderHeight_NoHeader(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	height, err := dao.GetCurrentBlockHeight()
	require.Error(t, err)
	require.Equal(t, uint32(0), height)
}

func TestGetCurrentHeaderHeight_Store(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	block := &Block{
		BlockBase: BlockBase{
			Script: transaction.Witness{
				VerificationScript: []byte{byte(opcode.PUSH1)},
				InvocationScript:   []byte{byte(opcode.NOP)},
			},
		},
	}
	err := dao.StoreAsCurrentBlock(block)
	require.NoError(t, err)
	height, err := dao.GetCurrentBlockHeight()
	require.NoError(t, err)
	require.Equal(t, uint32(0), height)
}

func TestStoreAsTransaction(t *testing.T) {
	dao := &dao{store: storage.NewMemCachedStore(storage.NewMemoryStore())}
	tx := &transaction.Transaction{}
	hash := tx.Hash()
	err := dao.StoreAsTransaction(tx, 0)
	require.NoError(t, err)
	hasTransaction := dao.HasTransaction(hash)
	require.True(t, hasTransaction)
}
