// Package nonnepxxcontractwithiterators contains RPC wrappers for Non-NEPXX contract with iterators contract.
package nonnepxxcontractwithiterators

import (
	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Hash contains contract hash.
var Hash = util.Uint160{0x33, 0x22, 0x11, 0x0, 0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x0}

// Invoker is used by ContractReader to call various safe methods.
type Invoker interface {
	Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error)
	CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...any) (*result.Invoke, error)
	TerminateSession(sessionID uuid.UUID) error
	TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error)
}

// ContractReader implements safe contract methods.
type ContractReader struct {
	invoker Invoker
	hash util.Uint160
}

// NewReader creates an instance of ContractReader using Hash and the given Invoker.
func NewReader(invoker Invoker) *ContractReader {
	var hash = Hash
	return &ContractReader{invoker, hash}
}

// Tokens invokes `tokens` method of contract.
func (c *ContractReader) Tokens() (uuid.UUID, result.Iterator, error) {
	return unwrap.SessionIterator(c.invoker.Call(c.hash, "tokens"))
}

// TokensExpanded is similar to Tokens (uses the same contract
// method), but can be useful if the server used doesn't support sessions and
// doesn't expand iterators. It creates a script that will get the specified
// number of result items from the iterator right in the VM and return them to
// you. It's only limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) TokensExpanded(_numOfIteratorItems int) ([]stackitem.Item, error) {
	return unwrap.Array(c.invoker.CallAndExpandIterator(c.hash, "tokens", _numOfIteratorItems))
}

// GetAllRecords invokes `getAllRecords` method of contract.
func (c *ContractReader) GetAllRecords(name string) (uuid.UUID, result.Iterator, error) {
	return unwrap.SessionIterator(c.invoker.Call(c.hash, "getAllRecords", name))
}

// GetAllRecordsExpanded is similar to GetAllRecords (uses the same contract
// method), but can be useful if the server used doesn't support sessions and
// doesn't expand iterators. It creates a script that will get the specified
// number of result items from the iterator right in the VM and return them to
// you. It's only limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) GetAllRecordsExpanded(name string, _numOfIteratorItems int) ([]stackitem.Item, error) {
	return unwrap.Array(c.invoker.CallAndExpandIterator(c.hash, "getAllRecords", _numOfIteratorItems, name))
}
