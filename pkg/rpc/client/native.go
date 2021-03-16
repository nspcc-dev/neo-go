package client

// Various non-policy things from native contracs.

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
