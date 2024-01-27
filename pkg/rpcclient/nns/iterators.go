package nns

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// RecordIterator is used for iterating over GetAllRecords results.
type RecordIterator struct {
	client   Invoker
	session  uuid.UUID
	iterator result.Iterator
}

// RootIterator is used for iterating over Roots results.
type RootIterator struct {
	client   Invoker
	session  uuid.UUID
	iterator result.Iterator
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

func itemsToRoots(arr []stackitem.Item) ([]string, error) {
	res := make([]string, len(arr))
	for i := range arr {
		rs, ok := arr[i].Value().([]stackitem.Item)
		if !ok {
			return nil, errors.New("wrong number of elements")
		}
		myval, _ := rs[0].TryBytes()
		res[i] = string(myval)
	}
	return res, nil
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

// Next returns the next set of elements from the iterator (up to num of them).
// It can return less than num elements in case iterator doesn't have that many
// or zero elements if the iterator has no more elements or the session is
// expired.
func (r *RootIterator) Next(num int) ([]string, error) {
	items, err := r.client.TraverseIterator(r.session, &r.iterator, num)
	if err != nil {
		return nil, err
	}
	return itemsToRoots(items)
}

// Terminate closes the iterator session used by RecordIterator (if it's
// session-based).
func (r *RecordIterator) Terminate() error {
	if r.iterator.ID == nil {
		return nil
	}
	return r.client.TerminateSession(r.session)
}

// Terminate closes the iterator session used by RootIterator (if it's
// session-based).
func (r *RootIterator) Terminate() error {
	if r.iterator.ID == nil {
		return nil
	}
	return r.client.TerminateSession(r.session)
}
