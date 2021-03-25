package client

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// TransferTarget represents target address and token amount for transfer.
type TransferTarget struct {
	Token   util.Uint160
	Address util.Uint160
	Amount  int64
}

// SignerAccount represents combination of the transaction.Signer and the
// corresponding wallet.Account.
type SignerAccount struct {
	Signer  transaction.Signer
	Account *wallet.Account
}

// NEP17Decimals invokes `decimals` NEP17 method on a specified contract.
func (c *Client) NEP17Decimals(tokenHash util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash, "decimals", []smartcontract.Parameter{}, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, fmt.Errorf("failed to get NEP17 decimals: %w", err)
	}

	return topIntFromStack(result.Stack)
}

// NEP17Symbol invokes `symbol` NEP17 method on a specified contract.
func (c *Client) NEP17Symbol(tokenHash util.Uint160) (string, error) {
	result, err := c.InvokeFunction(tokenHash, "symbol", []smartcontract.Parameter{}, nil)
	if err != nil {
		return "", err
	}
	err = getInvocationError(result)
	if err != nil {
		return "", fmt.Errorf("failed to get NEP17 symbol: %w", err)
	}

	return topStringFromStack(result.Stack)
}

// NEP17TotalSupply invokes `totalSupply` NEP17 method on a specified contract.
func (c *Client) NEP17TotalSupply(tokenHash util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash, "totalSupply", []smartcontract.Parameter{}, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, fmt.Errorf("failed to get NEP17 total supply: %w", err)
	}

	return topIntFromStack(result.Stack)
}

// NEP17BalanceOf invokes `balanceOf` NEP17 method on a specified contract.
func (c *Client) NEP17BalanceOf(tokenHash, acc util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash, "balanceOf", []smartcontract.Parameter{{
		Type:  smartcontract.Hash160Type,
		Value: acc,
	}}, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, fmt.Errorf("failed to get NEP17 balance: %w", err)
	}

	return topIntFromStack(result.Stack)
}

// NEP17TokenInfo returns full NEP17 token info.
func (c *Client) NEP17TokenInfo(tokenHash util.Uint160) (*wallet.Token, error) {
	cs, err := c.GetContractStateByHash(tokenHash)
	if err != nil {
		return nil, err
	}
	symbol, err := c.NEP17Symbol(tokenHash)
	if err != nil {
		return nil, err
	}
	decimals, err := c.NEP17Decimals(tokenHash)
	if err != nil {
		return nil, err
	}
	return wallet.NewToken(tokenHash, cs.Manifest.Name, symbol, decimals), nil
}

// CreateNEP17TransferTx creates an invocation transaction for the 'transfer'
// method of a given contract (token) to move specified amount of NEP17 assets
// (in FixedN format using contract's number of decimals) to given account and
// returns it. The returned transaction is not signed.
func (c *Client) CreateNEP17TransferTx(acc *wallet.Account, to util.Uint160, token util.Uint160, amount int64, gas int64, data interface{}) (*transaction.Transaction, error) {
	return c.CreateNEP17MultiTransferTx(acc, gas, []TransferTarget{
		{Token: token,
			Address: to,
			Amount:  amount,
		},
	}, []interface{}{data})
}

// CreateNEP17MultiTransferTx creates an invocation transaction for performing NEP17 transfers
// from a single sender to multiple recipients with the given data.
func (c *Client) CreateNEP17MultiTransferTx(acc *wallet.Account, gas int64, recipients []TransferTarget, data []interface{}) (*transaction.Transaction, error) {
	from, err := address.StringToUint160(acc.Address)
	if err != nil {
		return nil, fmt.Errorf("bad account address: %w", err)
	}
	if data == nil {
		data = make([]interface{}, len(recipients))
	} else {
		if len(data) != len(recipients) {
			return nil, fmt.Errorf("data and recipients number mismatch: %d vs %d", len(data), len(recipients))
		}
	}
	w := io.NewBufBinWriter()
	for i := range recipients {
		emit.AppCall(w.BinWriter, recipients[i].Token, "transfer", callflag.All,
			from, recipients[i].Address, recipients[i].Amount, data[i])
		emit.Opcodes(w.BinWriter, opcode.ASSERT)
	}
	if w.Err != nil {
		return nil, fmt.Errorf("failed to create transfer script: %w", w.Err)
	}
	accAddr, err := address.StringToUint160(acc.Address)
	if err != nil {
		return nil, fmt.Errorf("bad account address: %v", err)
	}
	return c.CreateTxFromScript(w.Bytes(), acc, -1, gas, []SignerAccount{{
		Signer: transaction.Signer{
			Account: accAddr,
			Scopes:  transaction.CalledByEntry,
		},
		Account: acc,
	}})
}

// CreateTxFromScript creates transaction and properly sets cosigners and NetworkFee.
// If sysFee <= 0, it is determined via result of `invokescript` RPC. You should
// initialize network magic with Init before calling CreateTxFromScript.
func (c *Client) CreateTxFromScript(script []byte, acc *wallet.Account, sysFee, netFee int64,
	cosigners []SignerAccount) (*transaction.Transaction, error) {
	signers, accounts, err := getSigners(acc, cosigners)
	if err != nil {
		return nil, fmt.Errorf("failed to construct tx signers: %w", err)
	}
	if sysFee < 0 {
		result, err := c.InvokeScript(script, signers)
		if err != nil {
			return nil, fmt.Errorf("can't add system fee to transaction: %w", err)
		}
		if result.State != "HALT" {
			return nil, fmt.Errorf("can't add system fee to transaction: bad vm state: %s due to an error: %s", result.State, result.FaultException)
		}
		sysFee = result.GasConsumed
	}

	if !c.initDone {
		return nil, errNetworkNotInitialized
	}
	tx := transaction.New(script, sysFee)
	tx.Signers = signers

	tx.ValidUntilBlock, err = c.CalculateValidUntilBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to add validUntilBlock to transaction: %w", err)
	}

	err = c.AddNetworkFee(tx, netFee, accounts...)
	if err != nil {
		return nil, fmt.Errorf("failed to add network fee: %w", err)
	}

	return tx, nil
}

// TransferNEP17 creates an invocation transaction that invokes 'transfer' method
// on a given token to move specified amount of NEP17 assets (in FixedN format
// using contract's number of decimals) to given account with data specified and
// sends it to the network returning just a hash of it.
func (c *Client) TransferNEP17(acc *wallet.Account, to util.Uint160, token util.Uint160, amount int64, gas int64, data interface{}) (util.Uint256, error) {
	tx, err := c.CreateNEP17TransferTx(acc, to, token, amount, gas, data)
	if err != nil {
		return util.Uint256{}, err
	}

	if err := acc.SignTx(c.GetNetwork(), tx); err != nil {
		return util.Uint256{}, fmt.Errorf("can't sign tx: %w", err)
	}

	return c.SendRawTransaction(tx)
}

// MultiTransferNEP17 is similar to TransferNEP17, buf allows to have multiple recipients.
func (c *Client) MultiTransferNEP17(acc *wallet.Account, gas int64, recipients []TransferTarget, data []interface{}) (util.Uint256, error) {
	tx, err := c.CreateNEP17MultiTransferTx(acc, gas, recipients, data)
	if err != nil {
		return util.Uint256{}, err
	}

	if err := acc.SignTx(c.GetNetwork(), tx); err != nil {
		return util.Uint256{}, fmt.Errorf("can't sign tx: %w", err)
	}

	return c.SendRawTransaction(tx)
}

func topIntFromStack(st []stackitem.Item) (int64, error) {
	index := len(st) - 1 // top stack element is last in the array
	bi, err := st[index].TryInteger()
	if err != nil {
		return 0, err
	}
	return bi.Int64(), nil
}

func topStringFromStack(st []stackitem.Item) (string, error) {
	index := len(st) - 1 // top stack element is last in the array
	bs, err := st[index].TryBytes()
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// getInvocationError returns an error in case of bad VM state or empty stack.
func getInvocationError(result *result.Invoke) error {
	if result.State != "HALT" {
		return fmt.Errorf("invocation failed: %s", result.FaultException)
	}
	if len(result.Stack) == 0 {
		return errors.New("result stack is empty")
	}
	return nil
}
