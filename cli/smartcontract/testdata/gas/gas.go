// Package gastoken contains RPC wrappers for GasToken contract.
package gastoken

import (
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Hash contains contract hash.
var Hash = util.Uint160{0xcf, 0x76, 0xe2, 0x8b, 0xd0, 0x6, 0x2c, 0x4a, 0x47, 0x8e, 0xe3, 0x55, 0x61, 0x1, 0x13, 0x19, 0xf3, 0xcf, 0xa4, 0xd2}

// Invoker is used by ContractReader to call various safe methods.
type Invoker interface {
	nep17.Invoker
}

// Actor is used by Contract to call state-changing methods.
type Actor interface {
	Invoker

	nep17.Actor
}

// ContractReader implements safe contract methods.
type ContractReader struct {
	nep17.TokenReader
	invoker Invoker
}

// Contract implements all contract methods.
type Contract struct {
	ContractReader
	nep17.TokenWriter
	actor Actor
}

// NewReader creates an instance of ContractReader using Hash and the given Invoker.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{*nep17.NewReader(invoker, Hash), invoker}
}

// New creates an instance of Contract using Hash and the given Actor.
func New(actor Actor) *Contract {
	var nep17t = nep17.New(actor, Hash)
	return &Contract{ContractReader{nep17t.TokenReader, actor}, nep17t.TokenWriter, actor}
}

