package nep11

import (
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// DivisibleReader is a reader interface for divisible NEP-11 contract.
type DivisibleReader struct {
	BaseReader
}

// DivisibleWriter is a state-changing interface for divisible NEP-11 contract.
// It's mostly useful not directly, but as a reusable layer for higher-level
// structures.
type DivisibleWriter struct {
	BaseWriter
}

// Divisible is a full  reader interface for divisible NEP-11 contract.
type Divisible struct {
	DivisibleReader
	DivisibleWriter
}

// OwnerIterator is used for iterating over OwnerOf (for divisible NFTs) results.
type OwnerIterator struct {
	client   Invoker
	session  uuid.UUID
	iterator result.Iterator
}

// NewDivisibleReader creates an instance of DivisibleReader for a contract
// with the given hash using the given invoker.
func NewDivisibleReader(invoker Invoker, hash util.Uint160) *DivisibleReader {
	return &DivisibleReader{*NewBaseReader(invoker, hash)}
}

// NewDivisible creates an instance of Divisible for a contract
// with the given hash using the given actor.
func NewDivisible(actor Actor, hash util.Uint160) *Divisible {
	return &Divisible{*NewDivisibleReader(actor, hash), DivisibleWriter{BaseWriter{hash, actor}}}
}

// OwnerOf returns returns an iterator that allows to walk through all owners of
// the given token. It depends on the server to provide proper session-based
// iterator, but can also work with expanded one.
func (t *DivisibleReader) OwnerOf(token []byte) (*OwnerIterator, error) {
	sess, iter, err := unwrap.SessionIterator(t.invoker.Call(t.hash, "ownerOf", token))
	if err != nil {
		return nil, err
	}
	return &OwnerIterator{t.invoker, sess, iter}, nil
}

// OwnerOfExpanded uses the same NEP-11 method as OwnerOf, but can be useful if
// the server used doesn't support sessions and doesn't expand iterators. It
// creates a script that will get num of result items from the iterator right in
// the VM and return them to you. It's only limited by VM stack and GAS available
// for RPC invocations.
func (t *DivisibleReader) OwnerOfExpanded(token []byte, num int) ([]util.Uint160, error) {
	return unwrap.ArrayOfUint160(t.invoker.CallAndExpandIterator(t.hash, "ownerOf", num, token))
}

// BalanceOfD is a BalanceOf for divisible NFTs, it returns the amount of token
// owned by a particular account.
func (t *DivisibleReader) BalanceOfD(owner util.Uint160, token []byte) (*big.Int, error) {
	return unwrap.BigInt(t.invoker.Call(t.hash, "balanceOf", owner, token))
}

// TransferD is a divisible version of (*Base).Transfer, allowing to transfer a
// part of NFT. It creates and sends a transaction that performs a `transfer`
// method call using the given parameters and checks for this call result,
// failing the transaction if it's not true. The returned values are transaction
// hash, its ValidUntilBlock value and an error if any.
func (t *DivisibleWriter) TransferD(from util.Uint160, to util.Uint160, amount *big.Int, id []byte, data interface{}) (util.Uint256, uint32, error) {
	script, err := t.transferScript(from, to, amount, id, data)
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return t.actor.SendRun(script)
}

// TransferDTransaction is a divisible version of (*Base).TransferTransaction,
// allowing to transfer a part of NFT. It creates a transaction that performs a
// `transfer` method call using the given parameters and checks for this call
// result, failing the transaction if it's not true. This transaction is signed,
// but not sent to the network, instead it's returned to the caller.
func (t *DivisibleWriter) TransferDTransaction(from util.Uint160, to util.Uint160, amount *big.Int, id []byte, data interface{}) (*transaction.Transaction, error) {
	script, err := t.transferScript(from, to, amount, id, data)
	if err != nil {
		return nil, err
	}
	return t.actor.MakeRun(script)
}

// TransferDUnsigned is a divisible version of (*Base).TransferUnsigned,
// allowing to transfer a part of NFT. It creates a transaction that performs a
// `transfer` method call using the given parameters and checks for this call
// result, failing the transaction if it's not true. This transaction is not
// signed and just returned to the caller.
func (t *DivisibleWriter) TransferDUnsigned(from util.Uint160, to util.Uint160, amount *big.Int, id []byte, data interface{}) (*transaction.Transaction, error) {
	script, err := t.transferScript(from, to, amount, id, data)
	if err != nil {
		return nil, err
	}
	return t.actor.MakeUnsignedRun(script, nil)
}

// Next returns the next set of elements from the iterator (up to num of them).
// It can return less than num elements in case iterator doesn't have that many
// or zero elements if the iterator has no more elements or the session is
// expired.
func (v *OwnerIterator) Next(num int) ([]util.Uint160, error) {
	items, err := v.client.TraverseIterator(v.session, &v.iterator, num)
	if err != nil {
		return nil, err
	}
	res := make([]util.Uint160, len(items))
	for i := range items {
		b, err := items[i].TryBytes()
		if err != nil {
			return nil, fmt.Errorf("element %d is not a byte string: %w", i, err)
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return nil, fmt.Errorf("element %d is not a uint160: %w", i, err)
		}
		res[i] = u
	}
	return res, nil
}

// Terminate closes the iterator session used by OwnerIterator (if it's
// session-based).
func (v *OwnerIterator) Terminate() error {
	if v.iterator.ID == nil {
		return nil
	}
	return v.client.TerminateSession(v.session)
}
