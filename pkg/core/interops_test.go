package core

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/smartcontract/trigger"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

func testNonInterop(t *testing.T, value interface{}, f func(*interopContext, *vm.VM) error) {
	v := vm.New()
	v.Estack().PushVal(value)
	context := newInteropContext(trigger.Application, newTestChain(t), storage.NewMemoryStore(), nil, nil)
	require.Error(t, f(context, v))
}

func TestUnexpectedNonInterops(t *testing.T) {
	vals := map[string]interface{}{
		"int":    1,
		"bool":   false,
		"string": "smth",
		"array":  []int{1, 2, 3},
	}

	// All of these functions expect an interop item on the stack.
	funcs := []func(*interopContext, *vm.VM) error{
		(*interopContext).accountGetBalance,
		(*interopContext).accountGetScriptHash,
		(*interopContext).accountGetVotes,
		(*interopContext).assetGetAdmin,
		(*interopContext).assetGetAmount,
		(*interopContext).assetGetAssetID,
		(*interopContext).assetGetAssetType,
		(*interopContext).assetGetAvailable,
		(*interopContext).assetGetIssuer,
		(*interopContext).assetGetOwner,
		(*interopContext).assetGetPrecision,
		(*interopContext).assetRenew,
		(*interopContext).attrGetData,
		(*interopContext).attrGetUsage,
		(*interopContext).blockGetTransaction,
		(*interopContext).blockGetTransactionCount,
		(*interopContext).blockGetTransactions,
		(*interopContext).contractGetScript,
		(*interopContext).contractGetStorageContext,
		(*interopContext).contractIsPayable,
		(*interopContext).headerGetConsensusData,
		(*interopContext).headerGetHash,
		(*interopContext).headerGetIndex,
		(*interopContext).headerGetMerkleRoot,
		(*interopContext).headerGetNextConsensus,
		(*interopContext).headerGetPrevHash,
		(*interopContext).headerGetTimestamp,
		(*interopContext).headerGetVersion,
		(*interopContext).inputGetHash,
		(*interopContext).inputGetIndex,
		(*interopContext).invocationTxGetScript,
		(*interopContext).outputGetAssetID,
		(*interopContext).outputGetScriptHash,
		(*interopContext).outputGetValue,
		(*interopContext).storageContextAsReadOnly,
		(*interopContext).storageDelete,
		(*interopContext).storageFind,
		(*interopContext).storageGet,
		(*interopContext).storagePut,
		(*interopContext).storagePutEx,
		(*interopContext).txGetAttributes,
		(*interopContext).txGetHash,
		(*interopContext).txGetInputs,
		(*interopContext).txGetOutputs,
		(*interopContext).txGetReferences,
		(*interopContext).txGetType,
		(*interopContext).txGetUnspentCoins,
		(*interopContext).txGetWitnesses,
		(*interopContext).witnessGetVerificationScript,
	}
	for _, f := range funcs {
		for k, v := range vals {
			fname := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
			t.Run(k+"/"+fname, func(t *testing.T) {
				testNonInterop(t, v, f)
			})
		}
	}
}
