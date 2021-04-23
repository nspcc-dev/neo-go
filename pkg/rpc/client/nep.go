package client

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
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

// nepTokenInfo returns full NEP* token info.
func (c *Client) nepTokenInfo(tokenHash util.Uint160, standard string) (*wallet.Token, error) {
	cs, err := c.GetContractStateByHash(tokenHash)
	if err != nil {
		return nil, err
	}
	var isStandardOK bool
	for _, st := range cs.Manifest.SupportedStandards {
		if st == standard {
			isStandardOK = true
			break
		}
	}
	if !isStandardOK {
		return nil, fmt.Errorf("token %s does not support %s standard", tokenHash.StringLE(), standard)
	}
	symbol, err := c.nepSymbol(tokenHash)
	if err != nil {
		return nil, err
	}
	decimals, err := c.nepDecimals(tokenHash)
	if err != nil {
		return nil, err
	}
	return wallet.NewToken(tokenHash, cs.Manifest.Name, symbol, decimals, standard), nil
}
