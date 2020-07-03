package client

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/pkg/errors"
)

// PolicyContractHash represents BE hash of native Policy contract.
var PolicyContractHash = util.Uint160{154, 97, 164, 110, 236, 151, 184, 147, 6, 215, 206, 129, 241, 91, 70, 32, 145, 208, 9, 50}

// GetMaxTransactionsPerBlock invokes `getMaxTransactionsPerBlock` method on a
// native Policy contract.
func (c *Client) GetMaxTransactionsPerBlock() (int64, error) {
	return c.invokeNativePolicyMethod("getMaxTransactionsPerBlock")
}

// GetMaxBlockSize invokes `getMaxBlockSize` method on a native Policy contract.
func (c *Client) GetMaxBlockSize() (int64, error) {
	return c.invokeNativePolicyMethod("getMaxBlockSize")
}

// GetFeePerByte invokes `getFeePerByte` method on a native Policy contract.
func (c *Client) GetFeePerByte() (int64, error) {
	return c.invokeNativePolicyMethod("getFeePerByte")
}

func (c *Client) invokeNativePolicyMethod(operation string) (int64, error) {
	result, err := c.InvokeFunction(PolicyContractHash, operation, []smartcontract.Parameter{}, nil)
	if err != nil {
		return 0, err
	} else if result.State != "HALT" || len(result.Stack) == 0 {
		return 0, errors.New("invalid VM state")
	}

	return topIntFromStack(result.Stack)
}

// GetBlockedAccounts invokes `getBlockedAccounts` method on a native Policy contract.
func (c *Client) GetBlockedAccounts() (native.BlockedAccounts, error) {
	result, err := c.InvokeFunction(PolicyContractHash, "getBlockedAccounts", []smartcontract.Parameter{}, nil)
	if err != nil {
		return nil, err
	} else if result.State != "HALT" || len(result.Stack) == 0 {
		return nil, errors.New("invalid VM state")
	}

	return topBlockedAccountsFromStack(result.Stack)
}

func topBlockedAccountsFromStack(st []smartcontract.Parameter) (native.BlockedAccounts, error) {
	index := len(st) - 1 // top stack element is last in the array
	var (
		ba  native.BlockedAccounts
		err error
	)
	switch typ := st[index].Type; typ {
	case smartcontract.ArrayType:
		data, ok := st[index].Value.([]smartcontract.Parameter)
		if !ok {
			return nil, errors.New("invalid Array item")
		}
		ba = make(native.BlockedAccounts, len(data))
		for i, account := range data {
			ba[i], err = util.Uint160DecodeBytesLE(account.Value.([]byte))
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("invalid stack item type: %s", typ)
	}
	return ba, nil
}
