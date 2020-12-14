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
	return c.invokeNativePolicyMethod("getFeePerByte")
}

func (c *Client) invokeNativePolicyMethod(operation string) (int64, error) {
	if !c.initDone {
		return 0, errNetworkNotInitialized
	}
	result, err := c.InvokeFunction(c.cache.nativeHashes[nativenames.Policy], operation, []smartcontract.Parameter{}, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, fmt.Errorf("failed to invoke %s Policy method: %w", operation, err)
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
