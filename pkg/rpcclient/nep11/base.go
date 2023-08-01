/*
Package nep11 contains RPC wrappers for NEP-11 contracts.

The set of types provided is split between common NEP-11 methods (BaseReader and
Base types) and divisible (DivisibleReader and Divisible) and non-divisible
(NonDivisibleReader and NonDivisible). If you don't know the type of NEP-11
contract you're going to use you can use Base and BaseReader types for many
purposes, otherwise more specific types are recommended.
*/
package nep11

import (
	"errors"
	"fmt"
	"math/big"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neptoken"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Invoker is used by reader types to call various methods.
type Invoker interface {
	neptoken.Invoker

	CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...any) (*result.Invoke, error)
	TerminateSession(sessionID uuid.UUID) error
	TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error)
}

// Actor is used by complete NEP-11 types to create and send transactions.
type Actor interface {
	Invoker

	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
}

// BaseReader is a reader interface for common divisible and non-divisible NEP-11
// methods. It allows to invoke safe methods.
type BaseReader struct {
	neptoken.Base

	invoker Invoker
	hash    util.Uint160
}

// BaseWriter is a transaction-creating interface for common divisible and
// non-divisible NEP-11 methods. It simplifies reusing this set of methods,
// but a complete Base is expected to be used in other packages.
type BaseWriter struct {
	hash  util.Uint160
	actor Actor
}

// Base is a state-changing interface for common divisible and non-divisible NEP-11
// methods.
type Base struct {
	BaseReader
	BaseWriter
}

// TransferEvent represents a Transfer event as defined in the NEP-11 standard.
type TransferEvent struct {
	From   util.Uint160
	To     util.Uint160
	Amount *big.Int
	ID     []byte
}

// TokenIterator is used for iterating over TokensOf results.
type TokenIterator struct {
	client   Invoker
	session  uuid.UUID
	iterator result.Iterator
}

// NewBaseReader creates an instance of BaseReader for a contract with the given
// hash using the given invoker.
func NewBaseReader(invoker Invoker, hash util.Uint160) *BaseReader {
	return &BaseReader{*neptoken.New(invoker, hash), invoker, hash}
}

// NewBase creates an instance of Base for contract with the given
// hash using the given actor.
func NewBase(actor Actor, hash util.Uint160) *Base {
	return &Base{*NewBaseReader(actor, hash), BaseWriter{hash, actor}}
}

// Properties returns a set of token's properties such as name or URL. The map
// is returned as is from this method (stack item) for maximum flexibility,
// contracts can return a lot of specific data there. Most of the time though
// they return well-defined properties outlined in NEP-11 and
// UnwrapKnownProperties can be used to get them in more convenient way. It's an
// optional method per NEP-11 specification, so it can fail.
func (t *BaseReader) Properties(token []byte) (*stackitem.Map, error) {
	return unwrap.Map(t.invoker.Call(t.hash, "properties", token))
}

// Tokens returns an iterator that allows to retrieve all tokens minted by the
// contract. It depends on the server to provide proper session-based
// iterator, but can also work with expanded one. The method itself is optional
// per NEP-11 specification, so it can fail.
func (t *BaseReader) Tokens() (*TokenIterator, error) {
	sess, iter, err := unwrap.SessionIterator(t.invoker.Call(t.hash, "tokens"))
	if err != nil {
		return nil, err
	}
	return &TokenIterator{t.invoker, sess, iter}, nil
}

// TokensExpanded uses the same NEP-11 method as Tokens, but can be useful if
// the server used doesn't support sessions and doesn't expand iterators. It
// creates a script that will get num of result items from the iterator right in
// the VM and return them to you. It's only limited by VM stack and GAS available
// for RPC invocations.
func (t *BaseReader) TokensExpanded(num int) ([][]byte, error) {
	return unwrap.ArrayOfBytes(t.invoker.CallAndExpandIterator(t.hash, "tokens", num))
}

// TokensOf returns an iterator that allows to walk through all tokens owned by
// the given account. It depends on the server to provide proper session-based
// iterator, but can also work with expanded one.
func (t *BaseReader) TokensOf(account util.Uint160) (*TokenIterator, error) {
	sess, iter, err := unwrap.SessionIterator(t.invoker.Call(t.hash, "tokensOf", account))
	if err != nil {
		return nil, err
	}
	return &TokenIterator{t.invoker, sess, iter}, nil
}

// TokensOfExpanded uses the same NEP-11 method as TokensOf, but can be useful if
// the server used doesn't support sessions and doesn't expand iterators. It
// creates a script that will get num of result items from the iterator right in
// the VM and return them to you. It's only limited by VM stack and GAS available
// for RPC invocations.
func (t *BaseReader) TokensOfExpanded(account util.Uint160, num int) ([][]byte, error) {
	return unwrap.ArrayOfBytes(t.invoker.CallAndExpandIterator(t.hash, "tokensOf", num, account))
}

// Transfer creates and sends a transaction that performs a `transfer` method
// call using the given parameters and checks for this call result, failing the
// transaction if it's not true. It works for divisible NFTs only when there is
// one owner for the particular token. The returned values are transaction hash,
// its ValidUntilBlock value and an error if any.
func (t *BaseWriter) Transfer(to util.Uint160, id []byte, data any) (util.Uint256, uint32, error) {
	script, err := t.transferScript(to, id, data)
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return t.actor.SendRun(script)
}

// TransferTransaction creates a transaction that performs a `transfer` method
// call using the given parameters and checks for this call result, failing the
// transaction if it's not true. It works for divisible NFTs only when there is
// one owner for the particular token. This transaction is signed, but not sent
// to the network, instead it's returned to the caller.
func (t *BaseWriter) TransferTransaction(to util.Uint160, id []byte, data any) (*transaction.Transaction, error) {
	script, err := t.transferScript(to, id, data)
	if err != nil {
		return nil, err
	}
	return t.actor.MakeRun(script)
}

// TransferUnsigned creates a transaction that performs a `transfer` method
// call using the given parameters and checks for this call result, failing the
// transaction if it's not true. It works for divisible NFTs only when there is
// one owner for the particular token. This transaction is not signed and just
// returned to the caller.
func (t *BaseWriter) TransferUnsigned(to util.Uint160, id []byte, data any) (*transaction.Transaction, error) {
	script, err := t.transferScript(to, id, data)
	if err != nil {
		return nil, err
	}
	return t.actor.MakeUnsignedRun(script, nil)
}

func (t *BaseWriter) transferScript(params ...any) ([]byte, error) {
	return smartcontract.CreateCallWithAssertScript(t.hash, "transfer", params...)
}

// Next returns the next set of elements from the iterator (up to num of them).
// It can return less than num elements in case iterator doesn't have that many
// or zero elements if the iterator has no more elements or the session is
// expired.
func (v *TokenIterator) Next(num int) ([][]byte, error) {
	items, err := v.client.TraverseIterator(v.session, &v.iterator, num)
	if err != nil {
		return nil, err
	}
	res := make([][]byte, len(items))
	for i := range items {
		b, err := items[i].TryBytes()
		if err != nil {
			return nil, fmt.Errorf("element %d is not a byte string: %w", i, err)
		}
		res[i] = b
	}
	return res, nil
}

// Terminate closes the iterator session used by TokenIterator (if it's
// session-based).
func (v *TokenIterator) Terminate() error {
	if v.iterator.ID == nil {
		return nil
	}
	return v.client.TerminateSession(v.session)
}

// UnwrapKnownProperties can be used as a proxy function to extract well-known
// NEP-11 properties (name/description/image/tokenURI) defined in the standard.
// These properties are checked to be valid UTF-8 strings, but can contain
// control codes or special characters.
func UnwrapKnownProperties(m *stackitem.Map, err error) (map[string]string, error) {
	if err != nil {
		return nil, err
	}
	elems := m.Value().([]stackitem.MapElement)
	res := make(map[string]string)
	for _, e := range elems {
		k, err := e.Key.TryBytes()
		if err != nil { // Shouldn't ever happen in the valid Map, but.
			continue
		}
		ks := string(k)
		if !result.KnownNEP11Properties[ks] { // Some additional elements are OK.
			continue
		}
		v, err := e.Value.TryBytes()
		if err != nil { // But known ones MUST be proper strings.
			return nil, fmt.Errorf("invalid %s property: %w", ks, err)
		}
		if !utf8.Valid(v) {
			return nil, fmt.Errorf("invalid %s property: not a UTF-8 string", ks)
		}
		res[ks] = string(v)
	}
	return res, nil
}

// TransferEventsFromApplicationLog retrieves all emitted TransferEvents from the
// provided [result.ApplicationLog].
func TransferEventsFromApplicationLog(log *result.ApplicationLog) ([]*TransferEvent, error) {
	if log == nil {
		return nil, errors.New("nil application log")
	}
	var res []*TransferEvent
	for i, ex := range log.Executions {
		for j, e := range ex.Events {
			if e.Name != "Transfer" {
				continue
			}
			event := new(TransferEvent)
			err := event.FromStackItem(e.Item)
			if err != nil {
				return nil, fmt.Errorf("failed to decode event from stackitem (event #%d, execution #%d): %w", j, i, err)
			}
			res = append(res, event)
		}
	}
	return res, nil
}

// FromStackItem converts provided [stackitem.Array] to TransferEvent or returns an
// error if it's not possible to do to so.
func (e *TransferEvent) FromStackItem(item *stackitem.Array) error {
	if item == nil {
		return errors.New("nil item")
	}
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != 4 {
		return errors.New("wrong number of event parameters")
	}

	b, err := arr[0].TryBytes()
	if err != nil {
		return fmt.Errorf("invalid From: %w", err)
	}
	e.From, err = util.Uint160DecodeBytesBE(b)
	if err != nil {
		return fmt.Errorf("failed to decode From: %w", err)
	}

	b, err = arr[1].TryBytes()
	if err != nil {
		return fmt.Errorf("invalid To: %w", err)
	}
	e.To, err = util.Uint160DecodeBytesBE(b)
	if err != nil {
		return fmt.Errorf("failed to decode To: %w", err)
	}

	e.Amount, err = arr[2].TryInteger()
	if err != nil {
		return fmt.Errorf("field to decode Avount: %w", err)
	}

	e.ID, err = arr[3].TryBytes()
	if err != nil {
		return fmt.Errorf("failed to decode ID: %w", err)
	}

	return nil
}
