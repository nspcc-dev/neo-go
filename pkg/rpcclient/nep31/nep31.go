package nep31

import (
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Actor is used by Token to create and send transactions.
type Actor interface {
	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
}

// Token provides full NEP-22 interface.
type Token struct {
	hash  util.Uint160
	actor Actor
}

// New creates an instance of Token for contract with the given hash
// using the given Actor.
func New(actor Actor, hash util.Uint160) *Token {
	return &Token{
		hash:  hash,
		actor: actor,
	}
}

// Destroy calls the contract's "destroy" method in a single transaction.
func (t *Token) Destroy() (util.Uint256, uint32, error) {
	script, err := t.buildDestroyScript()
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return t.actor.SendRun(script)
}

// DestroyTransaction builds and signs a destroy transaction without sending it.
func (t *Token) DestroyTransaction() (*transaction.Transaction, error) {
	script, err := t.buildDestroyScript()
	if err != nil {
		return nil, err
	}
	return t.actor.MakeRun(script)
}

// DestroyUnsigned builds an unsigned destroy transaction.
func (t *Token) DestroyUnsigned() (*transaction.Transaction, error) {
	script, err := t.buildDestroyScript()
	if err != nil {
		return nil, err
	}
	return t.actor.MakeUnsignedRun(script, nil)
}

// buildUpdateScript generates a VM script that invokes update for parameter set.
func (t *Token) buildDestroyScript() ([]byte, error) {
	b := smartcontract.NewBuilder()
	b.InvokeMethod(t.hash, "destroy")
	return b.Script()
}
