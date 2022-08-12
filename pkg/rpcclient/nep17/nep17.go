/*
Package nep17 contains RPC wrappers to work with NEP-17 contracts.

Safe methods are encapsulated into TokenReader structure while Token provides
various methods to perform the only NEP-17 state-changing call, Transfer.
*/
package nep17

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neptoken"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Invoker is used by TokenReader to call various safe methods.
type Invoker interface {
	neptoken.Invoker
}

// Actor is used by Token to create and send transactions.
type Actor interface {
	Invoker

	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
}

// TokenReader represents safe (read-only) methods of NEP-17 token. It can be
// used to query various data.
type TokenReader struct {
	neptoken.Base

	invoker Invoker
	hash    util.Uint160
}

// Token provides full NEP-17 interface, both safe and state-changing methods.
type Token struct {
	TokenReader

	actor Actor
}

// TransferEvent represents a Transfer event as defined in the NEP-17 standard.
type TransferEvent struct {
	From   util.Uint160
	To     util.Uint160
	Amount *big.Int
}

// NewReader creates an instance of TokenReader for contract with the given hash
// using the given Invoker.
func NewReader(invoker Invoker, hash util.Uint160) *TokenReader {
	return &TokenReader{*neptoken.New(invoker, hash), invoker, hash}
}

// New creates an instance of Token for contract with the given hash
// using the given Actor.
func New(actor Actor, hash util.Uint160) *Token {
	return &Token{*NewReader(actor, hash), actor}
}

// BalanceOf returns the token balance of the given account.
func (t *TokenReader) BalanceOf(account util.Uint160) (*big.Int, error) {
	return unwrap.BigInt(t.invoker.Call(t.hash, "balanceOf", account))
}

// Transfer creates and sends a transaction that performs a `transfer` method
// call using the given parameters and checks for this call result, failing the
// transaction if it's not true. The returned values are transaction hash, its
// ValidUntilBlock value and an error if any.
func (t *Token) Transfer(from util.Uint160, to util.Uint160, amount *big.Int, data interface{}) (util.Uint256, uint32, error) {
	script, err := t.transferScript(from, to, amount, data)
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return t.actor.SendRun(script)
}

// TransferTransaction creates a transaction that performs a `transfer` method
// call using the given parameters and checks for this call result, failing the
// transaction if it's not true. This transaction is signed, but not sent to the
// network, instead it's returned to the caller.
func (t *Token) TransferTransaction(from util.Uint160, to util.Uint160, amount *big.Int, data interface{}) (*transaction.Transaction, error) {
	script, err := t.transferScript(from, to, amount, data)
	if err != nil {
		return nil, err
	}
	return t.actor.MakeRun(script)
}

// TransferUnsigned creates a transaction that performs a `transfer` method
// call using the given parameters and checks for this call result, failing the
// transaction if it's not true. This transaction is not signed and just returned
// to the caller.
func (t *Token) TransferUnsigned(from util.Uint160, to util.Uint160, amount *big.Int, data interface{}) (*transaction.Transaction, error) {
	script, err := t.transferScript(from, to, amount, data)
	if err != nil {
		return nil, err
	}
	return t.actor.MakeUnsignedRun(script, nil)
}

func (t *Token) transferScript(from util.Uint160, to util.Uint160, amount *big.Int, data interface{}) ([]byte, error) {
	return smartcontract.CreateCallWithAssertScript(t.hash, "transfer", from, to, amount, data)
}
