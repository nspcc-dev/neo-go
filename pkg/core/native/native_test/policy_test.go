package native_test

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

func newPolicyClient(t *testing.T) *neotest.ContractInvoker {
	return newNativeClient(t, nativenames.Policy)
}

func TestPolicy_FeePerByte(t *testing.T) {
	testGetSet(t, newPolicyClient(t), "FeePerByte", 1000, 0, 100_000_000)
}

func TestPolicy_FeePerByteCache(t *testing.T) {
	testGetSetCache(t, newPolicyClient(t), "FeePerByte", 1000)
}

func TestPolicy_ExecFeeFactor(t *testing.T) {
	testGetSet(t, newPolicyClient(t), "ExecFeeFactor", interop.DefaultBaseExecFee, 1, 1000)
}

func TestPolicy_ExecFeeFactorCache(t *testing.T) {
	testGetSetCache(t, newPolicyClient(t), "ExecFeeFactor", interop.DefaultBaseExecFee)
}

func TestPolicy_StoragePrice(t *testing.T) {
	testGetSet(t, newPolicyClient(t), "StoragePrice", native.DefaultStoragePrice, 1, 10000000)
}

func TestPolicy_StoragePriceCache(t *testing.T) {
	testGetSetCache(t, newPolicyClient(t), "StoragePrice", native.DefaultStoragePrice)
}

func TestPolicy_BlockedAccounts(t *testing.T) {
	c := newPolicyClient(t)
	e := c.Executor
	randomInvoker := c.WithSigners(c.NewAccount(t))
	committeeInvoker := c.WithSigners(c.Committee)
	unlucky := util.Uint160{1, 2, 3}

	t.Run("isBlocked", func(t *testing.T) {
		randomInvoker.Invoke(t, false, "isBlocked", unlucky)
	})

	t.Run("block-unblock account", func(t *testing.T) {
		committeeInvoker.Invoke(t, true, "blockAccount", unlucky)
		randomInvoker.Invoke(t, true, "isBlocked", unlucky)
		committeeInvoker.Invoke(t, true, "unblockAccount", unlucky)
		randomInvoker.Invoke(t, false, "isBlocked", unlucky)
	})

	t.Run("double-block", func(t *testing.T) {
		// block
		committeeInvoker.Invoke(t, true, "blockAccount", unlucky)

		// double-block should fail
		committeeInvoker.Invoke(t, false, "blockAccount", unlucky)

		// unblock
		committeeInvoker.Invoke(t, true, "unblockAccount", unlucky)

		// unblock the same account should fail as we don't have it blocked
		committeeInvoker.Invoke(t, false, "unblockAccount", unlucky)
	})

	t.Run("not signed by committee", func(t *testing.T) {
		randomInvoker.InvokeFail(t, "invalid committee signature", "blockAccount", unlucky)
		randomInvoker.InvokeFail(t, "invalid committee signature", "unblockAccount", unlucky)
	})

	t.Run("block-unblock contract", func(t *testing.T) {
		committeeInvoker.InvokeFail(t, "cannot block native contract", "blockAccount", c.NativeHash(t, nativenames.Neo))

		helper := neotest.CompileFile(t, c.CommitteeHash, "./helpers/policyhelper", "./helpers/policyhelper/policyhelper.yml")
		e.DeployContract(t, helper, nil)
		helperInvoker := e.CommitteeInvoker(helper.Hash)

		helperInvoker.Invoke(t, true, "do")
		committeeInvoker.Invoke(t, true, "blockAccount", helper.Hash)
		helperInvoker.InvokeFail(t, fmt.Sprintf("contract %s is blocked", helper.Hash.StringLE()), "do")

		committeeInvoker.Invoke(t, true, "unblockAccount", helper.Hash)
		helperInvoker.Invoke(t, true, "do")
	})
}
