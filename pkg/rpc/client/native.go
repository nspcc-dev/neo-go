package client

// Various non-policy things from native contracts.

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
)

// GetOraclePrice invokes `getPrice` method on a native Oracle contract.
func (c *Client) GetOraclePrice() (int64, error) {
	oracleHash, err := c.GetNativeContractHash(nativenames.Notary)
	if err != nil {
		return 0, fmt.Errorf("failed to get native Oracle hash: %w", err)
	}
	return c.invokeNativeGetMethod(oracleHash, "getPrice")
}

// GetNNSPrice invokes `getPrice` method on a native NameService contract.
func (c *Client) GetNNSPrice() (int64, error) {
	nnsHash, err := c.GetNativeContractHash(nativenames.NameService)
	if err != nil {
		return 0, fmt.Errorf("failed to get native NameService hash: %w", err)
	}
	return c.invokeNativeGetMethod(nnsHash, "getPrice")
}

// GetGasPerBlock invokes `getGasPerBlock` method on a native NEO contract.
func (c *Client) GetGasPerBlock() (int64, error) {
	neoHash, err := c.GetNativeContractHash(nativenames.Neo)
	if err != nil {
		return 0, fmt.Errorf("failed to get native NEO hash: %w", err)
	}
	return c.invokeNativeGetMethod(neoHash, "getGasPerBlock")
}
