package client

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// NEP5Decimals invokes `decimals` NEP5 method on a specified contract.
func (c *Client) NEP5Decimals(tokenHash util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash.StringLE(), "decimals", []smartcontract.Parameter{})
	if err != nil {
		return 0, err
	} else if result.State != "HALT" || len(result.Stack) == 0 {
		return 0, errors.New("invalid VM state")
	}

	return topIntFromStack(result.Stack)
}

// NEP5Name invokes `name` NEP5 method on a specified contract.
func (c *Client) NEP5Name(tokenHash util.Uint160) (string, error) {
	result, err := c.InvokeFunction(tokenHash.StringLE(), "name", []smartcontract.Parameter{})
	if err != nil {
		return "", err
	} else if result.State != "HALT" || len(result.Stack) == 0 {
		return "", errors.New("invalid VM state")
	}

	return topStringFromStack(result.Stack)
}

// NEP5Symbol invokes `symbol` NEP5 method on a specified contract.
func (c *Client) NEP5Symbol(tokenHash util.Uint160) (string, error) {
	result, err := c.InvokeFunction(tokenHash.StringLE(), "symbol", []smartcontract.Parameter{})
	if err != nil {
		return "", err
	} else if result.State != "HALT" || len(result.Stack) == 0 {
		return "", errors.New("invalid VM state")
	}

	return topStringFromStack(result.Stack)
}

// NEP5TotalSupply invokes `totalSupply` NEP5 method on a specified contract.
func (c *Client) NEP5TotalSupply(tokenHash util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash.StringLE(), "totalSupply", []smartcontract.Parameter{})
	if err != nil {
		return 0, err
	} else if result.State != "HALT" || len(result.Stack) == 0 {
		return 0, errors.New("invalid VM state")
	}

	return topIntFromStack(result.Stack)
}

// NEP5BalanceOf invokes `balanceOf` NEP5 method on a specified contract.
func (c *Client) NEP5BalanceOf(tokenHash util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash.StringLE(), "balanceOf", []smartcontract.Parameter{})
	if err != nil {
		return 0, err
	} else if result.State != "HALT" || len(result.Stack) == 0 {
		return 0, errors.New("invalid VM state")
	}

	return topIntFromStack(result.Stack)
}

// NEP5TokenInfo returns full NEP5 token info.
func (c *Client) NEP5TokenInfo(tokenHash util.Uint160) (*wallet.Token, error) {
	name, err := c.NEP5Name(tokenHash)
	if err != nil {
		return nil, err
	}
	symbol, err := c.NEP5Symbol(tokenHash)
	if err != nil {
		return nil, err
	}
	decimals, err := c.NEP5Decimals(tokenHash)
	if err != nil {
		return nil, err
	}
	return wallet.NewToken(tokenHash, name, symbol, decimals), nil
}

// TransferNEP5 creates an invocation transaction that invokes 'transfer' method
// on a given token to move specified amount of NEP5 assets (in FixedN format
// using contract's number of decimals) to given account.
func (c *Client) TransferNEP5(acc *wallet.Account, to util.Uint160, token *wallet.Token, amount int64, gas util.Fixed8) (util.Uint256, error) {
	from, err := address.StringToUint160(acc.Address)
	if err != nil {
		return util.Uint256{}, fmt.Errorf("bad account address: %v", err)
	}
	// Note: we don't use invoke function here because it requires
	// 2 round trips instead of one.
	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, token.Hash, "transfer", from, to, amount)
	emit.Opcode(w.BinWriter, opcode.ASSERT)

	tx := transaction.NewInvocationTX(w.Bytes(), gas)
	tx.Sender = from

	tx.ValidUntilBlock, err = c.CalculateValidUntilBlock()
	if err != nil {
		return util.Uint256{}, fmt.Errorf("can't calculate validUntilBlock: %v", err)
	}

	if err := request.AddInputsAndUnspentsToTx(tx, acc.Address, core.UtilityTokenID(), gas, c); err != nil {
		return util.Uint256{}, fmt.Errorf("can't add GAS to transaction: %v", err)
	}

	if err := acc.SignTx(tx); err != nil {
		return util.Uint256{}, fmt.Errorf("can't sign tx: %v", err)
	}

	if err := c.SendRawTransaction(tx); err != nil {
		return util.Uint256{}, err
	}

	return tx.Hash(), nil
}

func topIntFromStack(st []smartcontract.Parameter) (int64, error) {
	index := len(st) - 1 // top stack element is last in the array
	var decimals int64
	switch typ := st[index].Type; typ {
	case smartcontract.IntegerType:
		var ok bool
		decimals, ok = st[index].Value.(int64)
		if !ok {
			return 0, errors.New("invalid Integer item")
		}
	case smartcontract.ByteArrayType:
		data, ok := st[index].Value.([]byte)
		if !ok {
			return 0, errors.New("invalid ByteArray item")
		}
		decimals = emit.BytesToInt(data).Int64()
	default:
		return 0, fmt.Errorf("invalid stack item type: %s", typ)
	}
	return decimals, nil
}

func topStringFromStack(st []smartcontract.Parameter) (string, error) {
	index := len(st) - 1 // top stack element is last in the array
	var s string
	switch typ := st[index].Type; typ {
	case smartcontract.StringType:
		var ok bool
		s, ok = st[index].Value.(string)
		if !ok {
			return "", errors.New("invalid String item")
		}
	case smartcontract.ByteArrayType:
		data, ok := st[index].Value.([]byte)
		if !ok {
			return "", errors.New("invalid ByteArray item")
		}
		s = string(data)
	default:
		return "", fmt.Errorf("invalid stack item type: %s", typ)
	}
	return s, nil
}
