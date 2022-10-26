// Package nameservice contains RPC wrappers for NameService contract.
package nameservice

import (
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep11"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"math/big"
)

// Hash contains contract hash.
var Hash = util.Uint160{0xde, 0x46, 0x5f, 0x5d, 0x50, 0x57, 0xcf, 0x33, 0x28, 0x47, 0x94, 0xc5, 0xcf, 0xc2, 0xc, 0x69, 0x37, 0x1c, 0xac, 0x50}

// Invoker is used by ContractReader to call various safe methods.
type Invoker interface {
	nep11.Invoker
	Call(contract util.Uint160, operation string, params ...interface{}) (*result.Invoke, error)
}

// ContractReader implements safe contract methods.
type ContractReader struct {
	nep11.NonDivisibleReader
	invoker Invoker
}

// NewReader creates an instance of ContractReader using Hash and the given Invoker.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{*nep11.NewNonDivisibleReader(invoker, Hash), invoker}
}


// Roots invokes `roots` method of contract.
func (c *ContractReader) Roots() (stackitem.Item, error) {
	return unwrap.Item(c.invoker.Call(Hash, "roots"))
}

// GetPrice invokes `getPrice` method of contract.
func (c *ContractReader) GetPrice(length *big.Int) (*big.Int, error) {
	return unwrap.BigInt(c.invoker.Call(Hash, "getPrice", length))
}

// IsAvailable invokes `isAvailable` method of contract.
func (c *ContractReader) IsAvailable(name string) (bool, error) {
	return unwrap.Bool(c.invoker.Call(Hash, "isAvailable", name))
}

// GetRecord invokes `getRecord` method of contract.
func (c *ContractReader) GetRecord(name string, type *big.Int) (string, error) {
	return unwrap.UTF8String(c.invoker.Call(Hash, "getRecord", name, type))
}

// GetAllRecords invokes `getAllRecords` method of contract.
func (c *ContractReader) GetAllRecords(name string) (stackitem.Item, error) {
	return unwrap.Item(c.invoker.Call(Hash, "getAllRecords", name))
}

// Resolve invokes `resolve` method of contract.
func (c *ContractReader) Resolve(name string, type *big.Int) (string, error) {
	return unwrap.UTF8String(c.invoker.Call(Hash, "resolve", name, type))
}
