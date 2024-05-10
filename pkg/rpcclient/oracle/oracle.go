/*
Package oracle allows to work with the native OracleContract contract via RPC.

Safe methods are encapsulated into ContractReader structure while Contract provides
various methods to perform state-changing calls.
*/
package oracle

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
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

// Hash stores the hash of the native OracleContract contract.
var Hash = nativehashes.Oracle

const priceSetter = "setPrice"

// ContractReader provides an interface to call read-only OracleContract
// contract's methods. "verify" method is not exposed since it's very specific
// and can't be executed successfully outside of the proper oracle response
// transaction.
type ContractReader struct {
	invoker Invoker
}

// Contract represents the OracleContract contract client that can be used to
// invoke its "setPrice" method. Other methods are useless for direct calls,
// "request" requires a callback that entry script can't provide and "finish"
// will only work in an oracle transaction. Since "setPrice" can be called
// successfully only by the network's committee, an appropriate Actor is needed
// for Contract.
type Contract struct {
	ContractReader

	actor Actor
}

// RequestEvent represents an OracleRequest notification event emitted from
// the OracleContract contract.
type RequestEvent struct {
	ID       int64
	Contract util.Uint160
	URL      string
	Filter   string
}

// ResponseEvent represents an OracleResponse notification event emitted from
// the OracleContract contract.
type ResponseEvent struct {
	ID         int64
	OriginalTx util.Uint256
}

// NewReader creates an instance of ContractReader that can be used to read
// data from the contract.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{invoker}
}

// New creates an instance of Contract to perform actions using
// the given Actor.
func New(actor Actor) *Contract {
	return &Contract{*NewReader(actor), actor}
}

// GetPrice returns current price of the oracle request call.
func (c *ContractReader) GetPrice() (*big.Int, error) {
	return unwrap.BigInt(c.invoker.Call(Hash, "getPrice"))
}

// SetPrice creates and sends a transaction that sets the new price for the
// oracle request call. The action is successful when transaction ends in HALT
// state. The returned values are transaction hash, its ValidUntilBlock value and
// an error if any.
func (c *Contract) SetPrice(value *big.Int) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, priceSetter, value)
}

// SetPriceTransaction creates a transaction that sets the new price for the
// oracle request call. The action is successful when transaction ends in HALT
// state. The transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) SetPriceTransaction(value *big.Int) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, priceSetter, value)
}

// SetPriceUnsigned creates a transaction that sets the new price for the
// oracle request call. The action is successful when transaction ends in HALT
// state. The transaction is not signed and just returned to the caller.
func (c *Contract) SetPriceUnsigned(value *big.Int) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, priceSetter, nil, value)
}
