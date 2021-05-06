package client

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// NEP11Decimals invokes `decimals` NEP11 method on a specified contract.
func (c *Client) NEP11Decimals(tokenHash util.Uint160) (int64, error) {
	return c.nepDecimals(tokenHash)
}

// NEP11Symbol invokes `symbol` NEP11 method on a specified contract.
func (c *Client) NEP11Symbol(tokenHash util.Uint160) (string, error) {
	return c.nepSymbol(tokenHash)
}

// NEP11TotalSupply invokes `totalSupply` NEP11 method on a specified contract.
func (c *Client) NEP11TotalSupply(tokenHash util.Uint160) (int64, error) {
	return c.nepTotalSupply(tokenHash)
}

// NEP11BalanceOf invokes `balanceOf` NEP11 method on a specified contract.
func (c *Client) NEP11BalanceOf(tokenHash, owner util.Uint160) (int64, error) {
	return c.nepBalanceOf(tokenHash, owner, nil)
}

// NEP11TokenInfo returns full NEP11 token info.
func (c *Client) NEP11TokenInfo(tokenHash util.Uint160) (*wallet.Token, error) {
	return c.nepTokenInfo(tokenHash, manifest.NEP11StandardName)
}

// TransferNEP11 creates an invocation transaction that invokes 'transfer' method
// on a given token to move the whole NEP11 token with the specified token ID to
// given account and sends it to the network returning just a hash of it.
func (c *Client) TransferNEP11(acc *wallet.Account, to util.Uint160,
	tokenHash util.Uint160, tokenID string, data interface{}, gas int64, cosigners []SignerAccount) (util.Uint256, error) {
	if !c.initDone {
		return util.Uint256{}, errNetworkNotInitialized
	}
	tx, err := c.CreateNEP11TransferTx(acc, tokenHash, gas, cosigners, to, tokenID, data)
	if err != nil {
		return util.Uint256{}, err
	}

	return c.SignAndPushTx(tx, acc, cosigners)
}

// CreateNEP11TransferTx creates an invocation transaction for the 'transfer'
// method of a given contract (token) to move the whole (or the specified amount
// of) NEP11 token with the specified token ID to given account and returns it.
// The returned transaction is not signed. CreateNEP11TransferTx is also a
// helper for TransferNEP11 and TransferNEP11D.
// `args` for TransferNEP11:  to util.Uint160, tokenID string, data interface{};
// `args` for TransferNEP11D: from, to util.Uint160, amount int64, tokenID string, data interface{}.
func (c *Client) CreateNEP11TransferTx(acc *wallet.Account, tokenHash util.Uint160,
	gas int64, cosigners []SignerAccount, args ...interface{}) (*transaction.Transaction, error) {
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, tokenHash, "transfer", callflag.All, args...)
	emit.Opcodes(w.BinWriter, opcode.ASSERT)
	if w.Err != nil {
		return nil, fmt.Errorf("failed to create NEP11 transfer script: %w", w.Err)
	}
	from, err := address.StringToUint160(acc.Address)
	if err != nil {
		return nil, fmt.Errorf("bad account address: %w", err)
	}
	return c.CreateTxFromScript(w.Bytes(), acc, -1, gas, append([]SignerAccount{{
		Signer: transaction.Signer{
			Account: from,
			Scopes:  transaction.CalledByEntry,
		},
		Account: acc,
	}}, cosigners...))
}

// NEP11TokensOf returns an array of token IDs for the specified owner of the specified NFT token.
func (c *Client) NEP11TokensOf(tokenHash util.Uint160, owner util.Uint160) ([]string, error) {
	result, err := c.InvokeFunction(tokenHash, "tokensOf", []smartcontract.Parameter{
		{
			Type:  smartcontract.Hash160Type,
			Value: owner,
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	err = getInvocationError(result)
	if err != nil {
		return nil, err
	}

	arr, err := topIterableFromStack(result.Stack, string(""))
	if err != nil {
		return nil, fmt.Errorf("failed to get token IDs from stack: %w", err)
	}
	ids := make([]string, len(arr))
	for i := range ids {
		ids[i] = arr[i].(string)
	}
	return ids, nil
}

// Non-divisible NFT methods section start.

// NEP11NDOwnerOf invokes `ownerOf` non-devisible NEP11 method with the
// specified token ID on a specified contract.
func (c *Client) NEP11NDOwnerOf(tokenHash util.Uint160, tokenID string) (util.Uint160, error) {
	result, err := c.InvokeFunction(tokenHash, "ownerOf", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: tokenID,
		},
	}, nil)
	if err != nil {
		return util.Uint160{}, err
	}
	err = getInvocationError(result)
	if err != nil {
		return util.Uint160{}, err
	}

	return topUint160FromStack(result.Stack)
}

// Non-divisible NFT methods section end.

// Divisible NFT methods section start.

// TransferNEP11D creates an invocation transaction that invokes 'transfer'
// method on a given token to move specified amount of divisible NEP11 assets
// (in FixedN format using contract's number of decimals) to given account and
// sends it to the network returning just a hash of it.
func (c *Client) TransferNEP11D(acc *wallet.Account, to util.Uint160,
	tokenHash util.Uint160, amount int64, tokenID string, data interface{}, gas int64, cosigners []SignerAccount) (util.Uint256, error) {
	if !c.initDone {
		return util.Uint256{}, errNetworkNotInitialized
	}
	from, err := address.StringToUint160(acc.Address)
	if err != nil {
		return util.Uint256{}, fmt.Errorf("bad account address: %w", err)
	}
	tx, err := c.CreateNEP11TransferTx(acc, tokenHash, gas, cosigners, from, to, amount, tokenID, data)
	if err != nil {
		return util.Uint256{}, err
	}

	return c.SignAndPushTx(tx, acc, cosigners)
}

// NEP11DBalanceOf invokes `balanceOf` divisible NEP11 method on a
// specified contract.
func (c *Client) NEP11DBalanceOf(tokenHash, owner util.Uint160, tokenID string) (int64, error) {
	return c.nepBalanceOf(tokenHash, owner, &tokenID)
}

// NEP11DOwnerOf returns list of the specified NEP11 divisible token owners.
func (c *Client) NEP11DOwnerOf(tokenHash util.Uint160, tokenID string) ([]util.Uint160, error) {
	result, err := c.InvokeFunction(tokenHash, "ownerOf", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: tokenID,
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	err = getInvocationError(result)
	if err != nil {
		return nil, err
	}

	arr, err := topIterableFromStack(result.Stack, util.Uint160{})
	if err != nil {
		return nil, fmt.Errorf("failed to get token IDs from stack: %w", err)
	}
	owners := make([]util.Uint160, len(arr))
	for i := range owners {
		owners[i] = arr[i].(util.Uint160)
	}
	return owners, nil
}

// Divisible NFT methods section end.

// Optional NFT methods section start.

// NEP11Properties invokes `properties` optional NEP11 method on a
// specified contract.
func (c *Client) NEP11Properties(tokenHash util.Uint160, tokenID string) (*stackitem.Map, error) {
	result, err := c.InvokeFunction(tokenHash, "properties", []smartcontract.Parameter{{
		Type:  smartcontract.StringType,
		Value: tokenID,
	}}, nil)
	if err != nil {
		return nil, err
	}
	err = getInvocationError(result)
	if err != nil {
		return nil, err
	}

	return topMapFromStack(result.Stack)
}

// NEP11Tokens returns list of the tokens minted by the contract.
func (c *Client) NEP11Tokens(tokenHash util.Uint160) ([]string, error) {
	result, err := c.InvokeFunction(tokenHash, "tokens", []smartcontract.Parameter{}, nil)
	if err != nil {
		return nil, err
	}
	err = getInvocationError(result)
	if err != nil {
		return nil, err
	}

	arr, err := topIterableFromStack(result.Stack, string(""))
	if err != nil {
		return nil, fmt.Errorf("failed to get token IDs from stack: %w", err)
	}
	tokens := make([]string, len(arr))
	for i := range tokens {
		tokens[i] = arr[i].(string)
	}
	return tokens, nil
}

// Optional NFT methods section end.
