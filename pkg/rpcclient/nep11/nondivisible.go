package nep11

import (
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NonDivisibleReader is a reader interface for non-divisble NEP-11 contract.
type NonDivisibleReader struct {
	BaseReader
}

// NonDivisible is a state-changing interface for non-divisble NEP-11 contract.
type NonDivisible struct {
	Base
}

// NewNonDivisibleReader creates an instance of NonDivisibleReader for a contract
// with the given hash using the given invoker.
func NewNonDivisibleReader(invoker Invoker, hash util.Uint160) *NonDivisibleReader {
	return &NonDivisibleReader{*NewBaseReader(invoker, hash)}
}

// NewNonDivisible creates an instance of NonDivisible for a contract
// with the given hash using the given actor.
func NewNonDivisible(actor Actor, hash util.Uint160) *NonDivisible {
	return &NonDivisible{*NewBase(actor, hash)}
}

// OwnerOf returns the owner of the given NFT.
func (t *NonDivisibleReader) OwnerOf(token []byte) (util.Uint160, error) {
	return unwrap.Uint160(t.invoker.Call(t.hash, "ownerOf", token))
}

// OwnerOf is the same as (*NonDivisibleReader).OwnerOf.
func (t *NonDivisible) OwnerOf(token []byte) (util.Uint160, error) {
	r := NonDivisibleReader{t.BaseReader}
	return r.OwnerOf(token)
}
