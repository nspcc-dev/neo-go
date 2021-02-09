package client

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// GetMaxTransactionsPerBlock invokes `getMaxTransactionsPerBlock` method on a
// native Policy contract.
func (c *Client) GetMaxTransactionsPerBlock() (int64, error) {
	return c.invokeNativePolicyMethod("getMaxTransactionsPerBlock")
}

// GetMaxBlockSize invokes `getMaxBlockSize` method on a native Policy contract.
func (c *Client) GetMaxBlockSize() (int64, error) {
	return c.invokeNativePolicyMethod("getMaxBlockSize")
}

// GetFeePerByte invokes `getFeePerByte` method on a native Policy contract.
func (c *Client) GetFeePerByte() (int64, error) {
	if !c.initDone {
		return 0, errNetworkNotInitialized
	}
	return c.invokeNativePolicyMethod("getFeePerByte")
}

// GetExecFeeFactor invokes `getExecFeeFactor` method on a native Policy contract.
func (c *Client) GetExecFeeFactor() (int64, error) {
	if !c.initDone {
		return 0, errNetworkNotInitialized
	}
	return c.invokeNativePolicyMethod("getExecFeeFactor")
}

// GetMaxNotValidBeforeDelta invokes `getMaxNotValidBeforeDelta` method on a native Notary contract.
func (c *Client) GetMaxNotValidBeforeDelta() (int64, error) {
	notaryHash, err := c.GetNativeContractHash(nativenames.Notary)
	if err != nil {
		return 0, fmt.Errorf("failed to get native Notary hash: %w", err)
	}
	return c.invokeNativeGetMethod(notaryHash, "getMaxNotValidBeforeDelta")
}

// invokeNativePolicy method invokes Get* method on a native Policy contract.
func (c *Client) invokeNativePolicyMethod(operation string) (int64, error) {
	if !c.initDone {
		return 0, errNetworkNotInitialized
	}
	return c.invokeNativeGetMethod(c.cache.nativeHashes[nativenames.Policy], operation)
}

func (c *Client) invokeNativeGetMethod(hash util.Uint160, operation string) (int64, error) {
	result, err := c.InvokeFunction(hash, operation, []smartcontract.Parameter{}, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, fmt.Errorf("failed to invoke %s method of native contract %s: %w", operation, hash.StringLE(), err)
	}
	return topIntFromStack(result.Stack)
}

// IsBlocked invokes `isBlocked` method on native Policy contract.
func (c *Client) IsBlocked(hash util.Uint160) (bool, error) {
	if !c.initDone {
		return false, errNetworkNotInitialized
	}
	result, err := c.InvokeFunction(c.cache.nativeHashes[nativenames.Policy], "isBlocked", []smartcontract.Parameter{{
		Type:  smartcontract.Hash160Type,
		Value: hash,
	}}, nil)
	if err != nil {
		return false, err
	}
	err = getInvocationError(result)
	if err != nil {
		return false, fmt.Errorf("failed to check if account is blocked: %w", err)
	}
	return topBoolFromStack(result.Stack)
}

// topBoolFromStack returns the top boolean value from stack
func topBoolFromStack(st []stackitem.Item) (bool, error) {
	index := len(st) - 1 // top stack element is last in the array
	result, ok := st[index].Value().(bool)
	if !ok {
		return false, fmt.Errorf("invalid stack item type: %s", st[index].Type())
	}
	return result, nil
}
