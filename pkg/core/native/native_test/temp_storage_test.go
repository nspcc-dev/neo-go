package native_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/limits"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
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
		func FindValues(prefix []byte, opts storage.FindFlags) [][]byte {
			i := tempstorage.Find(prefix, opts)
			var res [][]byte
			for iterator.Next(i) {
				res = append(res, iterator.Value(i).([]byte))
			}
			return res
		}
		func FindValuesByHash(hash interop.Hash160, prefix []byte, opts storage.FindFlags) [][]byte {
			i := tempstorage.FindByHash(hash, prefix, opts)
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

func TestTempStorage_PutGetRenewDelete(t *testing.T) {
	c := newTempStorageClient(t)
	tmp, ctrHash := getTempStorageInvoker(t, c)
	key1 := []byte("aa1")
	value1 := []byte("one")
	key2 := []byte("aa2")
	value2 := []byte("two")
	key3 := []byte("bb")
	value3 := []byte("three")
	key4 := []byte("cc") // available for a single block only.
	value4 := []byte("four")

	topTimestamp := c.TopBlock(t).Timestamp
	msPerBlock := uint64(c.Chain.GetMillisecondsPerBlock())

	// put: good.
	validTill1 := topTimestamp + 4*msPerBlock
	validTill2 := topTimestamp + 5*msPerBlock
	validTillRenewed := topTimestamp + 6*msPerBlock
	tmp.Invoke(t, stackitem.Null{}, "put", key1, value1, validTill1)
	tmp.Invoke(t, stackitem.Null{}, "put", key2, value2, validTill2)
	tmp.Invoke(t, stackitem.Null{}, "put", key3, value3, validTill2)

	// get*: existing entry.
	tmp.Invoke(t, stackitem.Make(value1), "get", key1)
	tmp.Invoke(t, stackitem.Make(value1), "getByHash", ctrHash, key1)

	// getExpiration*: existing entry.
	tmp.Invoke(t, int(validTill1), "getExpiration", key1)
	tmp.Invoke(t, int(validTill1), "getExpirationByHash", ctrHash, key1)

	// get: retrieve the item available at the current block only.
	tx1 := tmp.PrepareInvoke(t, "put", key4, value4, c.TopBlock(t).Timestamp+1)
	tx2 := tmp.PrepareInvoke(t, "get", key4)
	tmp.AddNewBlock(t, tx1, tx2)
	c.CheckHalt(t, tx2.Hash(), stackitem.Make(value4))
	tmp.Invoke(t, stackitem.Null{}, "get", key4)

	// find*: values only.
	tmp.Invoke(t, stackitem.Make([]any{stackitem.NewBuffer(value1), stackitem.NewBuffer(value2)}), "findValues", []byte("aa"), istorage.FindValuesOnly)
	tmp.Invoke(t, stackitem.Make([]any{stackitem.NewBuffer(value1), stackitem.NewBuffer(value2)}), "findValuesByHash", ctrHash, []byte("aa"), istorage.FindValuesOnly)

	// find*: keys only, with/without prefix.
	tmp.Invoke(t, stackitem.Make([]any{stackitem.NewBuffer(key1), stackitem.NewBuffer(key2)}), "findValues", []byte("aa"), istorage.FindKeysOnly)
	tmp.Invoke(t, stackitem.Make([]any{stackitem.NewBuffer([]byte(strings.TrimLeft(string(key1), "a"))), stackitem.NewBuffer([]byte(strings.TrimLeft(string(key2), "a")))}), "findValues", []byte("aa"), istorage.FindKeysOnly|istorage.FindRemovePrefix)

	// renew: existing entry.
	tmp.Invoke(t, stackitem.Null{}, "renew", key1, validTillRenewed)
	tmp.Invoke(t, int(validTillRenewed), "getExpiration", key1)

	// delete: existing entry, getExpiration of missing entry.
	tmp.Invoke(t, stackitem.Null{}, "delete", key1)
	tmp.Invoke(t, 0, "getExpiration", key1)

	// renew: ensure stale validTill metadata is properly removed.
	key := []byte("123")
	value := []byte("123")
	oldTill := c.TopBlock(t).Timestamp + 2
	newTill := oldTill + 10
	tmp.Invoke(t, stackitem.Null{}, "put", key, value, oldTill)
	tmp.Invoke(t, stackitem.Null{}, "renew", key, newTill)
	tmp.Invoke(t, stackitem.Make(value), "get", key)
	tmp.Invoke(t, int(newTill), "getExpiration", key)
	tmp.Invoke(t, stackitem.Make(value), "get", key) // one more time to ensure it's still available.

	// put: max key.
	tmp.Invoke(t, stackitem.Null{}, "put", make([]byte, limits.MaxStorageKeyLen), value1, c.TopBlock(t).Timestamp+2)

	// Error cases.
	t.Run("put", func(t *testing.T) {
		t.Run("large key", func(t *testing.T) {
			maxValidTill := c.TopBlock(t).Timestamp + uint64(c.Chain.GetConfig().Genesis.TemporaryStorageMaxTTL.Milliseconds())
			tmp.InvokeFail(t, "input is too big: 65 vs 64", "put", make([]byte, limits.MaxStorageKeyLen+1), []byte("v"), maxValidTill)
		})
		t.Run("large validTill", func(t *testing.T) {
			maxValidTill := c.TopBlock(t).Timestamp + uint64(c.Chain.GetConfig().Genesis.TemporaryStorageMaxTTL.Milliseconds())
			tmp.InvokeFail(t, "validTill exceeds max limit", "put", []byte("high"), []byte("v"), maxValidTill+2)
		})
		t.Run("not enough gas", func(t *testing.T) {
			tmp.InvokeWithFeeFail(t, "failed to charge temporary storage fee", 1_5000000, "put", []byte("nogas"), make([]byte, 1000), c.TopBlock(t).Timestamp+uint64(c.Chain.GetConfig().Genesis.TemporaryStorageMaxTTL.Milliseconds()))
		})
		t.Run("caller is not a contract", func(t *testing.T) {
			c.InvokeFail(t, "failed to get calling contract", "put", []byte("k"), []byte("v"), c.TopBlock(t).Timestamp+uint64(c.Chain.GetConfig().Genesis.TemporaryStorageMaxTTL.Milliseconds()))
		})
	})
	t.Run("renew", func(t *testing.T) {
		t.Run("unexisting entry", func(t *testing.T) {
			tmp.InvokeFail(t, "failed to get old record", "renew", []byte("missing"), validTillRenewed)
		})
		t.Run("large key", func(t *testing.T) {
			tmp.InvokeFail(t, "input is too big", "renew", make([]byte, limits.MaxStorageKeyLen+1), validTillRenewed)
		})
		t.Run("large validTill", func(t *testing.T) {
			maxValidTill := c.TopBlock(t).Timestamp + uint64(c.Chain.GetConfig().Genesis.TemporaryStorageMaxTTL.Milliseconds())
			tmp.InvokeFail(t, "validTill exceeds max limit", "renew", key2, maxValidTill+2)
		})
		t.Run("not newer", func(t *testing.T) {
			tmp.InvokeFail(t, "new expiration point should be newer than the old one", "renew", key2, validTill2)
			tmp.InvokeFail(t, "new expiration point should be newer than the old one", "renew", key2, validTill2-1)
		})
		t.Run("expired entry", func(t *testing.T) {
			tmp.Invoke(t, stackitem.Null{}, "put", []byte("short"), []byte("v"), c.TopBlock(t).Timestamp+1)
			tmp.InvokeFail(t, "already expired", "renew", []byte("short"), validTillRenewed)
		})
		t.Run("caller is not a contract", func(t *testing.T) {
			c.InvokeFail(t, "failed to get calling contract", "renew", key2, validTillRenewed)
		})
	})
	t.Run("delete", func(t *testing.T) {
		t.Run("large key", func(t *testing.T) {
			tmp.InvokeFail(t, "input is too big", "delete", make([]byte, limits.MaxStorageKeyLen+1))
		})
		t.Run("missing entry", func(t *testing.T) {
			tmp.Invoke(t, stackitem.Null{}, "delete", []byte("missing"))
		})
		t.Run("caller is not a contract", func(t *testing.T) {
			c.InvokeFail(t, "failed to get calling contract", "delete", key2)
		})
	})
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
	tmp.Invoke(t, stackitem.Make([]any{stackitem.NewBuffer(value1), stackitem.NewBuffer(value2)}), "findValues", []byte("a"), istorage.FindValuesOnly)
	tmp.Invoke(t, stackitem.Null{}, "renew", key1, renewedTill)

	b := c.NewUnsignedBlock(t)
	b.Timestamp = renewedTill + 1
	c.SignBlock(b)
	require.NoError(t, c.Chain.AddBlock(b))

	tmp.Invoke(t, stackitem.Make([]any{stackitem.NewBuffer(value2)}), "findValues", []byte("a"), istorage.FindValuesOnly)
	tmp.InvokeFail(t, "failed to get old record", "renew", key1, c.TopBlock(t).Timestamp+3*msPerBlock)
}
