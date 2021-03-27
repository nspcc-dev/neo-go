package client

import (
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// nepDecimals invokes `decimals` NEP* method on a specified contract.
func (c *Client) nepDecimals(tokenHash util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash, "decimals", []smartcontract.Parameter{}, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, err
	}

	return topIntFromStack(result.Stack)
}

// nepSymbol invokes `symbol` NEP* method on a specified contract.
func (c *Client) nepSymbol(tokenHash util.Uint160) (string, error) {
	result, err := c.InvokeFunction(tokenHash, "symbol", []smartcontract.Parameter{}, nil)
	if err != nil {
		return "", err
	}
	err = getInvocationError(result)
	if err != nil {
		return "", err
	}

	return topStringFromStack(result.Stack)
}

// nepTotalSupply invokes `totalSupply` NEP* method on a specified contract.
func (c *Client) nepTotalSupply(tokenHash util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash, "totalSupply", []smartcontract.Parameter{}, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, err
	}

	return topIntFromStack(result.Stack)
}

// nepBalanceOf invokes `balanceOf` NEP* method on a specified contract.
func (c *Client) nepBalanceOf(tokenHash, acc util.Uint160, tokenID *string) (int64, error) {
	params := []smartcontract.Parameter{{
		Type:  smartcontract.Hash160Type,
		Value: acc,
	}}
	if tokenID != nil {
		params = append(params, smartcontract.Parameter{
			Type:  smartcontract.StringType,
			Value: *tokenID,
		})
	}
	result, err := c.InvokeFunction(tokenHash, "balanceOf", params, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, err
	}

	return topIntFromStack(result.Stack)
}
