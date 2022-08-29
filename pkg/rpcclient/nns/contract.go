/*
Package nns provide some RPC wrappers for the non-native NNS contract.

It's not yet a complete interface because there are different NNS versions
available, yet it provides the most widely used ones that were available from
the old RPC client API.
*/
package nns

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Invoker is used by ContractReader to call various methods.
type Invoker interface {
	Call(contract util.Uint160, operation string, params ...interface{}) (*result.Invoke, error)
	CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...interface{}) (*result.Invoke, error)
	TerminateSession(sessionID uuid.UUID) error
	TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error)
}

// ContractReader provides an interface to call read-only NNS contract methods.
type ContractReader struct {
	invoker Invoker
	hash    util.Uint160
}

// RecordIterator is used for iterating over GetAllRecords results.
type RecordIterator struct {
	client   Invoker
	session  uuid.UUID
	iterator result.Iterator
}

// NewReader creates an instance of ContractReader that can be used to read
// data from the contract.
func NewReader(invoker Invoker, hash util.Uint160) *ContractReader {
	return &ContractReader{invoker, hash}
}

// GetPrice returns current domain registration price in GAS.
func (c *ContractReader) GetPrice() (int64, error) {
	return unwrap.Int64(c.invoker.Call(c.hash, "getPrice"))
}

// IsAvailable checks whether the domain given is available for registration.
func (c *ContractReader) IsAvailable(name string) (bool, error) {
	return unwrap.Bool(c.invoker.Call(c.hash, "isAvailable", name))
}

// Resolve resolves the given record type for the given domain (with no more
// than three redirects).
func (c *ContractReader) Resolve(name string, typ RecordType) (string, error) {
	return unwrap.UTF8String(c.invoker.Call(c.hash, "resolve", name, int64(typ)))
}

// GetAllRecords returns an iterator that allows to retrieve all RecordState
// items for the given domain name. It depends on the server to provide proper
// session-based iterator, but can also work with expanded one.
func (c *ContractReader) GetAllRecords(name string) (*RecordIterator, error) {
	sess, iter, err := unwrap.SessionIterator(c.invoker.Call(c.hash, "getAllRecords", name))
	if err != nil {
		return nil, err
	}

	return &RecordIterator{
		client:   c.invoker,
		iterator: iter,
		session:  sess,
	}, nil
}

// Next returns the next set of elements from the iterator (up to num of them).
// It can return less than num elements in case iterator doesn't have that many
// or zero elements if the iterator has no more elements or the session is
// expired.
func (r *RecordIterator) Next(num int) ([]RecordState, error) {
	items, err := r.client.TraverseIterator(r.session, &r.iterator, num)
	if err != nil {
		return nil, err
	}
	return itemsToRecords(items)
}

// Terminate closes the iterator session used by RecordIterator (if it's
// session-based).
func (r *RecordIterator) Terminate() error {
	if r.iterator.ID == nil {
		return nil
	}
	return r.client.TerminateSession(r.session)
}

// GetAllRecordsExpanded is similar to GetAllRecords (uses the same NNS
// method), but can be useful if the server used doesn't support sessions and
// doesn't expand iterators. It creates a script that will get num of result
// items from the iterator right in the VM and return them to you. It's only
// limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) GetAllRecordsExpanded(name string, num int) ([]RecordState, error) {
	arr, err := unwrap.Array(c.invoker.CallAndExpandIterator(c.hash, "getAllRecords", num, name))
	if err != nil {
		return nil, err
	}
	return itemsToRecords(arr)
}

func itemsToRecords(arr []stackitem.Item) ([]RecordState, error) {
	res := make([]RecordState, len(arr))
	for i := range arr {
		err := res[i].FromStackItem(arr[i])
		if err != nil {
			return nil, fmt.Errorf("item #%d: %w", i, err)
		}
	}
	return res, nil
}
