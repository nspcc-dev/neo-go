package rpcclient

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
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
	return unwrap.Int64(c.reader.Call(hash, operation))
}

// IsBlocked invokes `isBlocked` method on native Policy contract.
func (c *Client) IsBlocked(hash util.Uint160) (bool, error) {
	policyHash, err := c.GetNativeContractHash(nativenames.Policy)
	if err != nil {
		return false, fmt.Errorf("failed to get native Policy hash: %w", err)
	}
	return unwrap.Bool(c.reader.Call(policyHash, "isBlocked", hash))
}
