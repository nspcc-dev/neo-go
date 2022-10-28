// Package nextoken contains RPC wrappers for NEX Token contract.
package nextoken

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"math/big"
)

// Hash contains contract hash.
var Hash = util.Uint160{0xa8, 0x1a, 0xa1, 0xf0, 0x4b, 0xf, 0xdc, 0x4a, 0xa2, 0xce, 0xd5, 0xbf, 0xc6, 0x22, 0xcf, 0xe8, 0x9, 0x7f, 0xa6, 0xa2}

// Invoker is used by ContractReader to call various safe methods.
type Invoker interface {
	nep17.Invoker
	Call(contract util.Uint160, operation string, params ...interface{}) (*result.Invoke, error)
}

// ContractReader implements safe contract methods.
type ContractReader struct {
	nep17.TokenReader
	invoker Invoker
}

// NewReader creates an instance of ContractReader using Hash and the given Invoker.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{*nep17.NewReader(invoker, Hash), invoker}
}


// Cap invokes `cap` method of contract.
func (c *ContractReader) Cap() (*big.Int, error) {
	return unwrap.BigInt(c.invoker.Call(Hash, "cap"))
}

// GetMinter invokes `getMinter` method of contract.
func (c *ContractReader) GetMinter() (*keys.PublicKey, error) {
	return unwrap.PublicKey(c.invoker.Call(Hash, "getMinter"))
}

// GetOwner invokes `getOwner` method of contract.
func (c *ContractReader) GetOwner() (util.Uint160, error) {
	return unwrap.Uint160(c.invoker.Call(Hash, "getOwner"))
}

// TotalMinted invokes `totalMinted` method of contract.
func (c *ContractReader) TotalMinted() (*big.Int, error) {
	return unwrap.BigInt(c.invoker.Call(Hash, "totalMinted"))
}
