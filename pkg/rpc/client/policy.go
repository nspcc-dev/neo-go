package client

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// PolicyContractHash represents a hash of native Policy contract.
var PolicyContractHash, _ = util.Uint160DecodeStringBE("e9ff4ca7cc252e1dfddb26315869cd79505906ce")

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

func topBlockedAccountsFromStack(st []stackitem.Item) (native.BlockedAccounts, error) {
	index := len(st) - 1 // top stack element is last in the array
	var (
		ba  native.BlockedAccounts
		err error
	)
	items, ok := st[index].Value().([]stackitem.Item)
	if !ok {
		return nil, fmt.Errorf("invalid stack item type: %s", st[index].Type())
	}
	ba = make(native.BlockedAccounts, len(items))
	for i, account := range items {
		val, ok := account.Value().([]byte)
		if !ok {
			return nil, fmt.Errorf("invalid array element: %s", account.Type())
		}
		ba[i], err = util.Uint160DecodeBytesLE(val)
		if err != nil {
			return nil, err
		}
	}
	return ba, nil
}
