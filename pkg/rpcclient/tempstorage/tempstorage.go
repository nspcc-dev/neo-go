/*
Package tempstorage allows to work with the native TemporaryStorage contract via RPC.

Safe methods are encapsulated into ContractReader structure. State-changing
`put`, `renew` and `delete` methods of TemporaryStorage can be called by
contracts only (the records are bound to the calling contract), so they can't
be invoked directly via transaction script and thus have no wrappers here. The
`get`, `getExpiration` and `find` methods are wrapped in their "by hash"
form since the calling contract mocking is not available for the RPC invocations.
*/
package tempstorage

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Invoker is used by ContractReader to call various methods.
type Invoker interface {
	Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error)
	CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...any) (*result.Invoke, error)
	TerminateSession(sessionID uuid.UUID) error
	TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error)
}

// Hash stores the hash of the native TemporaryStorage contract.
var Hash = nativehashes.TemporaryStorage

// ContractReader provides an interface to call read-only TemporaryStorage
// contract's methods.
type ContractReader struct {
	invoker Invoker
}

// FindIterator is used for iterating over Find results.
type FindIterator struct {
	client   Invoker
	session  uuid.UUID
	iterator result.Iterator
}

// NewReader creates an instance of ContractReader that can be used to read
// data from the contract.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{invoker}
}

// Get returns the value stored by the given contract under the given key. It
// returns nil if there's no such record or if it's already expired.
func (c *ContractReader) Get(hash util.Uint160, key []byte) ([]byte, error) {
	r, err := c.invoker.Call(Hash, "get", hash, key)
	itm, err := unwrap.Item(r, err)
	if err != nil {
		return nil, err
	}
	if _, ok := itm.(stackitem.Null); ok {
		return nil, nil
	}
	return itm.TryBytes()
}

// GetExpiration returns the expiration timestamp (in milliseconds) of the
// record stored by the given contract under the given key. It returns 0 if
// there's no such record or if it's already expired.
func (c *ContractReader) GetExpiration(hash util.Uint160, key []byte) (int64, error) {
	return unwrap.Int64(c.invoker.Call(Hash, "getExpiration", hash, key))
}

// Find returns an iterator over the records stored by the given contract with
// the given key prefix. The options parameter is a combination of storage
// find flags (can be taken from the pkg/core/interop/storage package).
func (c *ContractReader) Find(hash util.Uint160, prefix []byte, options int64) (*FindIterator, error) {
	sess, iter, err := unwrap.SessionIterator(c.invoker.Call(Hash, "find", hash, prefix, options))
	if err != nil {
		return nil, err
	}
	return &FindIterator{
		client:   c.invoker,
		iterator: iter,
		session:  sess,
	}, nil
}

// FindExpanded is similar to Find (uses the same contract method), but can be
// useful if the server used doesn't support sessions and doesn't expand
// iterators. It creates a script that will get num of result items from the
// iterator right in the VM and return them to you. It's only limited by VM
// stack and GAS available for RPC invocations.
func (c *ContractReader) FindExpanded(hash util.Uint160, prefix []byte, options int64, num int) ([]stackitem.Item, error) {
	return unwrap.Array(c.invoker.CallAndExpandIterator(Hash, "find", num, hash, prefix, options))
}

// Next returns the next set of elements from the iterator (up to num of them).
// It can return less than num elements in case iterator doesn't have that many
// or zero elements if the iterator has no more elements or the session is
// expired.
func (i *FindIterator) Next(num int) ([]stackitem.Item, error) {
	items, err := i.client.TraverseIterator(i.session, &i.iterator, num)
	if err != nil {
		return nil, fmt.Errorf("traverse iterator: %w", err)
	}
	return items, nil
}

// Terminate closes the iterator session used by FindIterator (if it's
// session-based).
func (i *FindIterator) Terminate() error {
	if i.iterator.ID == nil {
		return nil
	}
	return i.client.TerminateSession(i.session)
}
