/*
Package nep17 contains RPC wrappers to work with NEP-17 contracts.

Safe methods are encapsulated into TokenReader structure while Token provides
various methods to perform the only NEP-17 state-changing call, Transfer.
*/
package nep17

import (
	"errors"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neptoken"
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
}

// TokenWriter contains NEP-17 token methods that change state. It's not meant
// to be used directly (Token that includes it is more convenient) and just
// separates one set of methods from another to simplify reusing this package
// for other contracts that extend NEP-17 interface.
type TokenWriter struct {
	hash  util.Uint160
	actor Actor
}

// Token provides full NEP-17 interface, both safe and state-changing methods.
type Token struct {
	TokenReader
	TokenWriter
}

// TransferEvent represents a Transfer event as defined in the NEP-17 standard.
type TransferEvent struct {
	From   util.Uint160
	To     util.Uint160
	Amount *big.Int
}

// TransferParameters is a set of parameters for `transfer` method.
type TransferParameters struct {
	From   util.Uint160
	To     util.Uint160
	Amount *big.Int
	Data   interface{}
}

// NewReader creates an instance of TokenReader for contract with the given
// hash using the given Invoker.
func NewReader(invoker Invoker, hash util.Uint160) *TokenReader {
	return &TokenReader{*neptoken.New(invoker, hash)}
}

// New creates an instance of Token for contract with the given hash
// using the given Actor.
func New(actor Actor, hash util.Uint160) *Token {
	return &Token{*NewReader(actor, hash), TokenWriter{hash, actor}}
}

// Transfer creates and sends a transaction that performs a `transfer` method
// call using the given parameters and checks for this call result, failing the
// transaction if it's not true. The returned values are transaction hash, its
// ValidUntilBlock value and an error if any.
func (t *TokenWriter) Transfer(from util.Uint160, to util.Uint160, amount *big.Int, data interface{}) (util.Uint256, uint32, error) {
	return t.MultiTransfer([]TransferParameters{{from, to, amount, data}})
}

// TransferTransaction creates a transaction that performs a `transfer` method
// call using the given parameters and checks for this call result, failing the
// transaction if it's not true. This transaction is signed, but not sent to the
// network, instead it's returned to the caller.
func (t *TokenWriter) TransferTransaction(from util.Uint160, to util.Uint160, amount *big.Int, data interface{}) (*transaction.Transaction, error) {
	return t.MultiTransferTransaction([]TransferParameters{{from, to, amount, data}})
}

// TransferUnsigned creates a transaction that performs a `transfer` method
// call using the given parameters and checks for this call result, failing the
// transaction if it's not true. This transaction is not signed and just returned
// to the caller.
func (t *TokenWriter) TransferUnsigned(from util.Uint160, to util.Uint160, amount *big.Int, data interface{}) (*transaction.Transaction, error) {
	return t.MultiTransferUnsigned([]TransferParameters{{from, to, amount, data}})
}

func (t *TokenWriter) multiTransferScript(params []TransferParameters) ([]byte, error) {
	if len(params) == 0 {
		return nil, errors.New("at least one transfer parameter required")
	}
	b := smartcontract.NewBuilder()
	for i := range params {
		b.InvokeWithAssert(t.hash, "transfer", params[i].From,
			params[i].To, params[i].Amount, params[i].Data)
	}
	return b.Script()
}

// MultiTransfer is not a real NEP-17 method, but rather a convenient way to
// perform multiple transfers (usually from a single account) in one transaction.
// It accepts a set of parameters, creates a script that calls `transfer` as
// many times as needed (with ASSERTs added, so if any of these transfers fail
// whole transaction (with all transfers) fails). The values returned are the
// same as in Transfer.
func (t *TokenWriter) MultiTransfer(params []TransferParameters) (util.Uint256, uint32, error) {
	script, err := t.multiTransferScript(params)
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return t.actor.SendRun(script)
}

// MultiTransferTransaction is similar to MultiTransfer, but returns the same values
// as TransferTransaction (signed transaction that is not yet sent).
func (t *TokenWriter) MultiTransferTransaction(params []TransferParameters) (*transaction.Transaction, error) {
	script, err := t.multiTransferScript(params)
	if err != nil {
		return nil, err
	}
	return t.actor.MakeRun(script)
}

// MultiTransferUnsigned is similar to MultiTransfer, but returns the same values
// as TransferUnsigned (not yet signed transaction).
func (t *TokenWriter) MultiTransferUnsigned(params []TransferParameters) (*transaction.Transaction, error) {
	script, err := t.multiTransferScript(params)
	if err != nil {
		return nil, err
	}
	return t.actor.MakeUnsignedRun(script, nil)
}
