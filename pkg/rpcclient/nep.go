package rpcclient

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// nepDecimals invokes `decimals` NEP* method on the specified contract.
func (c *Client) nepDecimals(tokenHash util.Uint160) (int64, error) {
	return unwrap.Int64(c.reader.Call(tokenHash, "decimals"))
}

// nepSymbol invokes `symbol` NEP* method on the specified contract.
func (c *Client) nepSymbol(tokenHash util.Uint160) (string, error) {
	return unwrap.PrintableASCIIString(c.reader.Call(tokenHash, "symbol"))
}

// nepTotalSupply invokes `totalSupply` NEP* method on the specified contract.
func (c *Client) nepTotalSupply(tokenHash util.Uint160) (int64, error) {
	return unwrap.Int64(c.reader.Call(tokenHash, "totalSupply"))
}

// nepBalanceOf invokes `balanceOf` NEP* method on the specified contract.
func (c *Client) nepBalanceOf(tokenHash, acc util.Uint160, tokenID []byte) (int64, error) {
	params := []interface{}{acc}
	if tokenID != nil {
		params = append(params, tokenID)
	}
	return unwrap.Int64(c.reader.Call(tokenHash, "balanceOf", params...))
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
