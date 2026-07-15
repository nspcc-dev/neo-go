package native_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newTempStorageClient(t *testing.T) *neotest.ContractInvoker {
	return newCustomTempStorageClient(t, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFHuyao.String(): 0,
		}
	})
}

func newCustomTempStorageClient(t *testing.T, f func(cfg *config.Blockchain)) *neotest.ContractInvoker {
	bc, acc := chain.NewSingleWithCustomConfig(t, f)
	e := neotest.NewExecutor(t, bc, acc, acc)

	return e.CommitteeInvoker(nativehashes.TemporaryStorage)
}

func getTempStorageInvoker(t *testing.T, tempStorageC *neotest.ContractInvoker) (*neotest.ContractInvoker, util.Uint160) {
	src := `package tempstorageinvoker
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop"
			"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
			"github.com/nspcc-dev/neo-go/pkg/interop/storage"
			"github.com/nspcc-dev/neo-go/pkg/interop/native/tempstorage"
		)
		func Put(key, value []byte, validTill int) {
			tempstorage.Put(key, value, validTill)
		}
		func Get(key []byte) []byte {
			return tempstorage.Get(key)
		}
		func GetByHash(hash interop.Hash160, key []byte) []byte {
			return tempstorage.GetByHash(hash, key)
		}
		func GetExpiration(key []byte) int {
			return tempstorage.GetExpiration(key)
		}
		func GetExpirationByHash(hash interop.Hash160, key []byte) int {
			return tempstorage.GetExpirationByHash(hash, key)
		}
		func Delete(key []byte) {
			tempstorage.Delete(key)
		}
		func FindValues(prefix []byte) [][]byte {
			i := tempstorage.Find(prefix, storage.ValuesOnly)
			var res [][]byte
			for iterator.Next(i) {
				res = append(res, iterator.Value(i).([]byte))
			}
			return res
		}
		func FindValuesByHash(hash interop.Hash160, prefix []byte) [][]byte {
			i := tempstorage.FindByHash(hash, prefix, storage.ValuesOnly)
			var res [][]byte
			for iterator.Next(i) {
				res = append(res, iterator.Value(i).([]byte))
			}
			return res
		}
		func Renew(key []byte, validTill int) {
			tempstorage.Renew(key, validTill)
		}
	`
	e := tempStorageC.Executor
	ctr := neotest.CompileSource(t, e.Validator.ScriptHash(), strings.NewReader(src), &compiler.Options{
		Name: "tempstorageinvoker",
		Permissions: []manifest.Permission{
			*manifest.NewPermission(manifest.PermissionWildcard),
		},
	})
	e.DeployContract(t, ctr, nil)
	ctrInvoker := e.NewInvoker(ctr.Hash, e.Committee)
	return ctrInvoker, ctr.Hash
}

func TestTempStorage_Activation(t *testing.T) {
	c := newCustomTempStorageClient(t, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{
			config.HFHuyao.String(): 3,
		}
	})
	till := c.TopBlock(t).Timestamp + uint64(10*c.Chain.GetMillisecondsPerBlock())

	tempStorageInvoker, _ := getTempStorageInvoker(t, c)
	key := []byte{1}
	value := []byte{2}

	// Invoke before Huyao should fail.
	tempStorageInvoker.InvokeWithFeeFail(t, fmt.Sprintf("token contract %s not found: key not found", nativehashes.TemporaryStorage.StringLE()), 10000_0000, "put", key, value, till)

	// Invoke at Huyao should fail.
	tempStorageInvoker.InvokeWithFeeFail(t, "System.Contract.CallNative failed: native contract TemporaryStorage is active after hardfork Huyao", 10000_0000, "put", key, value, till)

	// Invoke after Huyao should succeed.
	tempStorageInvoker.Invoke(t, stackitem.Null{}, "put", key, value, till)
}

func TestTempStorage(t *testing.T) {
	c := newTempStorageClient(t)
	tmp, ctrHash := getTempStorageInvoker(t, c)
	key1 := []byte("aa1")
	value1 := []byte("one")
	key2 := []byte("aa2")
	value2 := []byte("two")
	key3 := []byte("bb")
	value3 := []byte("three")

	topTimestamp := c.TopBlock(t).Timestamp
	msPerBlock := uint64(c.Chain.GetMillisecondsPerBlock())

	validTill1 := topTimestamp + 4*msPerBlock
	validTill2 := topTimestamp + 5*msPerBlock
	validTillRenewed := topTimestamp + 6*msPerBlock
	minValidTill := topTimestamp + 2*msPerBlock
	tmp.Invoke(t, stackitem.Null{}, "put", key1, value1, validTill1)
	tmp.Invoke(t, stackitem.Null{}, "put", key2, value2, validTill2)
	tmp.Invoke(t, stackitem.Null{}, "put", key3, value3, validTill2)
	tmp.Invoke(t, stackitem.Make(value1), "get", key1)
	tmp.Invoke(t, stackitem.Make(value1), "getByHash", ctrHash, key1)
	tmp.Invoke(t, int(validTill1), "getExpiration", key1)
	tmp.Invoke(t, int(validTill1), "getExpirationByHash", ctrHash, key1)

	tmp.Invoke(t, stackitem.Make([]any{stackitem.NewBuffer(value1), stackitem.NewBuffer(value2)}), "findValues", []byte("aa"))
	tmp.Invoke(t, stackitem.Make([]any{stackitem.NewBuffer(value1), stackitem.NewBuffer(value2)}), "findValuesByHash", ctrHash, []byte("aa"))

	tmp.Invoke(t, stackitem.Null{}, "renew", key1, validTillRenewed)
	tmp.Invoke(t, int(validTillRenewed), "getExpiration", key1)

	tmp.Invoke(t, stackitem.Null{}, "delete", key1)
	tmp.Invoke(t, 0, "getExpiration", key1)

	tmp.InvokeFail(t, "item is valid for less than 2*msPerBlock", "put", []byte("low"), []byte("v"), minValidTill-1)
	maxValidTill := c.TopBlock(t).Timestamp + uint64(c.Chain.GetConfig().Genesis.TemporaryStorageMaxTTL/time.Millisecond)
	tmp.InvokeFail(t, "validTill exceeds max limit", "put", []byte("high"), []byte("v"), maxValidTill+2)
	tmp.InvokeFail(t, "failed to get old record", "renew", []byte("missing"), validTillRenewed)
}

func TestTempStorage_PostPersistCleanup(t *testing.T) {
	c := newTempStorageClient(t)
	tmp, _ := getTempStorageInvoker(t, c)

	key1 := []byte("a1")
	value1 := []byte("1")
	key2 := []byte("a2")
	value2 := []byte("2")
	msPerBlock := uint64(c.Chain.GetMillisecondsPerBlock())
	validTill := c.TopBlock(t).Timestamp + 5*msPerBlock
	renewedTill := validTill + msPerBlock

	tmp.Invoke(t, stackitem.Null{}, "put", key1, value1, validTill)
	tmp.Invoke(t, stackitem.Null{}, "put", key2, value2, renewedTill+msPerBlock)
	tmp.Invoke(t, stackitem.Make([]any{stackitem.NewBuffer(value1), stackitem.NewBuffer(value2)}), "findValues", []byte("a"))
	tmp.Invoke(t, stackitem.Null{}, "renew", key1, renewedTill)

	b := c.NewUnsignedBlock(t)
	b.Timestamp = renewedTill + 1
	c.SignBlock(b)
	require.NoError(t, c.Chain.AddBlock(b))

	tmp.Invoke(t, stackitem.Make([]any{stackitem.NewBuffer(value2)}), "findValues", []byte("a"))
	tmp.InvokeFail(t, "failed to get old record", "renew", key1, c.TopBlock(t).Timestamp+3*msPerBlock)
}
