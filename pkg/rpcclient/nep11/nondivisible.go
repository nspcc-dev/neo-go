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
	NonDivisibleReader
	BaseWriter
}

// NewNonDivisibleReader creates an instance of NonDivisibleReader for a contract
// with the given hash using the given invoker.
func NewNonDivisibleReader(invoker Invoker, hash util.Uint160) *NonDivisibleReader {
	return &NonDivisibleReader{*NewBaseReader(invoker, hash)}
}

// NewNonDivisible creates an instance of NonDivisible for a contract
// with the given hash using the given actor.
func NewNonDivisible(actor Actor, hash util.Uint160) *NonDivisible {
	return &NonDivisible{*NewNonDivisibleReader(actor, hash), BaseWriter{hash, actor}}
}

// OwnerOf returns the owner of the given NFT.
func (t *NonDivisibleReader) OwnerOf(token []byte) (util.Uint160, error) {
	return unwrap.Uint160(t.invoker.Call(t.hash, "ownerOf", token))
}
