package nep22

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

// UpdateParameters is a set of parameters for `transfer` method.
type UpdateParameters struct {
	NefFile  []byte
	Manifest []byte
	Data     any
}

// New creates an instance of Token for contract with the given hash
// using the given Actor.
func New(actor Actor, hash util.Uint160) *Token {
	return &Token{
		hash:  hash,
		actor: actor,
	}
}

// Update calls the contract's "update" method in a single transaction.
func (t *Token) Update(nef []byte, manifestBytes []byte, data any) (util.Uint256, uint32, error) {
	script, err := t.buildUpdateScript(&UpdateParameters{nef, manifestBytes, data})
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return t.actor.SendRun(script)
}

// UpdateTransaction builds and signs an update transaction without sending it.
func (t *Token) UpdateTransaction(nef []byte, manifestBytes []byte, data any) (*transaction.Transaction, error) {
	script, err := t.buildUpdateScript(&UpdateParameters{nef, manifestBytes, data})
	if err != nil {
		return nil, err
	}
	return t.actor.MakeRun(script)
}

// UpdateUnsigned builds an unsigned update transaction.
func (t *Token) UpdateUnsigned(nef []byte, manifestBytes []byte, data any) (*transaction.Transaction, error) {
	script, err := t.buildUpdateScript(&UpdateParameters{nef, manifestBytes, data})
	if err != nil {
		return nil, err
	}
	return t.actor.MakeUnsignedRun(script, nil)
}

// buildUpdateScript generates a VM script that invokes update for parameter set.
func (t *Token) buildUpdateScript(p *UpdateParameters) ([]byte, error) {
	b := smartcontract.NewBuilder()
	b.InvokeMethod(t.hash, "update", p.NefFile, p.Manifest, p.Data)
	return b.Script()
}
