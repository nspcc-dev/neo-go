/*
Package rolemgmt allows to work with the native RoleManagement contract via RPC.

Safe methods are encapsulated into ContractReader structure while Contract provides
various methods to perform the only RoleManagement state-changing call.
*/
package rolemgmt

import (
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Invoker is used by ContractReader to call various methods.
type Invoker interface {
	Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error)
}

// Actor is used by Contract to create and send transactions.
type Actor interface {
	Invoker

	MakeCall(contract util.Uint160, method string, params ...any) (*transaction.Transaction, error)
	MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...any) (*transaction.Transaction, error)
	SendCall(contract util.Uint160, method string, params ...any) (util.Uint256, uint32, error)
}

// Hash stores the hash of the native RoleManagement contract.
var Hash = nativehashes.Designation

const designateMethod = "designateAsRole"

// ContractReader provides an interface to call read-only RoleManagement
// contract's methods.
type ContractReader struct {
	invoker Invoker
}

// Contract represents a RoleManagement contract client that can be used to
// invoke all of its methods.
type Contract struct {
	ContractReader

	actor Actor
}

// DesignationEvent represents an event emitted by RoleManagement contract when
// a new role designation is done.
type DesignationEvent struct {
	Role       noderoles.Role
	BlockIndex uint32
}

// NewReader creates an instance of ContractReader that can be used to read
// data from the contract.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{invoker}
}

// New creates an instance of Contract to perform actions using
// the given Actor. Notice that RoleManagement's state can be changed
// only by the network's committee, so the Actor provided must be a committee
// actor for designation methods to work properly.
func New(actor Actor) *Contract {
	return &Contract{*NewReader(actor), actor}
}

// GetDesignatedByRole returns the list of the keys designated to serve for the
// given role at the given height. The list can be empty if no keys are
// configured for this role/height.
func (c *ContractReader) GetDesignatedByRole(role noderoles.Role, index uint32) (keys.PublicKeys, error) {
	return unwrap.ArrayOfPublicKeys(c.invoker.Call(Hash, "getDesignatedByRole", int64(role), index))
}

// DesignateAsRole creates and sends a transaction that sets the keys used for
// the given node role. The action is successful when transaction ends in HALT
// state. The returned values are transaction hash, its ValidUntilBlock value
// and an error if any.
func (c *Contract) DesignateAsRole(role noderoles.Role, pubs keys.PublicKeys) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, designateMethod, int(role), pubs)
}

// DesignateAsRoleTransaction creates a transaction that sets the keys for the
// given node role. This transaction is signed, but not sent to the network,
// instead it's returned to the caller.
func (c *Contract) DesignateAsRoleTransaction(role noderoles.Role, pubs keys.PublicKeys) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, designateMethod, int(role), pubs)
}

// DesignateAsRoleUnsigned creates a transaction that sets the keys for the
// given node role. This transaction is not signed and just returned to the
// caller.
func (c *Contract) DesignateAsRoleUnsigned(role noderoles.Role, pubs keys.PublicKeys) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, designateMethod, nil, int(role), pubs)
}
