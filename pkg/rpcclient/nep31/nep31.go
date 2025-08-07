/*
Package nep31 contains RPC wrappers to work with NEP-31 contracts.

Contract provides state-changing method of destroyable contracts.
*/
package nep31

import (
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Actor is used by Contract wrapper to create and send transactions.
type Actor interface {
	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
}

// Contract provides full NEP-31 interface.
type Contract struct {
	hash  util.Uint160
	actor Actor
}

// NewContract creates an instance of Contract for contract with the given hash
// using the given Actor.
func NewContract(actor Actor, hash util.Uint160) *Contract {
	return &Contract{
		hash:  hash,
		actor: actor,
	}
}

// Destroy creates and sends a transaction that performs a `destroy` method
// call. This transaction is signed and immediately sent to the network.
// The returned values are its hash, ValidUntilBlock value and an error, if any.
func (t *Contract) Destroy() (util.Uint256, uint32, error) {
	script, err := t.destroyScript()
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return t.actor.SendRun(script)
}

// DestroyTransaction creates and signs a transaction invoking the `destroy` method
// without sending it. The returned values are the signed Transaction and an error
// if any.
func (t *Contract) DestroyTransaction() (*transaction.Transaction, error) {
	script, err := t.destroyScript()
	if err != nil {
		return nil, err
	}
	return t.actor.MakeRun(script)
}

// DestroyUnsigned creates an unsigned transaction invoking the `destroy` method
// without sending it. The returned values are the unsigned Transaction and
// an error if any.
func (t *Contract) DestroyUnsigned() (*transaction.Transaction, error) {
	script, err := t.destroyScript()
	if err != nil {
		return nil, err
	}
	return t.actor.MakeUnsignedRun(script, nil)
}

// destroyScript builds a VM script invoking the `destroy` method of the contract
// The returned values are the raw script and an error, if any.
func (t *Contract) destroyScript() ([]byte, error) {
	b := smartcontract.NewBuilder()
	b.InvokeMethod(t.hash, "destroy")
	return b.Script()
}
