package rpcclient

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// GetFeePerByte invokes `getFeePerByte` method on a native Policy contract.
func (c *Client) GetFeePerByte() (int64, error) {
	return c.invokeNativePolicyMethod("getFeePerByte")
}

// GetExecFeeFactor invokes `getExecFeeFactor` method on a native Policy contract.
func (c *Client) GetExecFeeFactor() (int64, error) {
	return c.invokeNativePolicyMethod("getExecFeeFactor")
}

// GetStoragePrice invokes `getStoragePrice` method on a native Policy contract.
func (c *Client) GetStoragePrice() (int64, error) {
	return c.invokeNativePolicyMethod("getStoragePrice")
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
	policyHash, err := c.GetNativeContractHash(nativenames.Policy)
	if err != nil {
		return 0, fmt.Errorf("failed to get native Policy hash: %w", err)
	}
	return c.invokeNativeGetMethod(policyHash, operation)
}

func (c *Client) invokeNativeGetMethod(hash util.Uint160, operation string) (int64, error) {
	result, err := c.reader.Call(hash, operation)
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
	policyHash, err := c.GetNativeContractHash(nativenames.Policy)
	if err != nil {
		return false, fmt.Errorf("failed to get native Policy hash: %w", err)
	}
	result, err := c.reader.Call(policyHash, "isBlocked", hash)
	if err != nil {
		return false, err
	}
	err = getInvocationError(result)
	if err != nil {
		return false, fmt.Errorf("failed to check if account is blocked: %w", err)
	}
	return topBoolFromStack(result.Stack)
}
