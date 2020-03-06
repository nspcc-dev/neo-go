package client

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
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
