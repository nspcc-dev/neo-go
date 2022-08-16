/*
Package management provides an RPC wrapper for the native ContractManagement contract.

Safe methods are encapsulated in the ContractReader structure while Contract provides
various methods to perform state-changing calls.
*/
package management

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Invoker is used by ContractReader to call various methods.
type Invoker interface {
	Call(contract util.Uint160, operation string, params ...interface{}) (*result.Invoke, error)
}

// Actor is used by Contract to create and send transactions.
type Actor interface {
	Invoker

	MakeCall(contract util.Uint160, method string, params ...interface{}) (*transaction.Transaction, error)
	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...interface{}) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendCall(contract util.Uint160, method string, params ...interface{}) (util.Uint256, uint32, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
}

// ContractReader provides an interface to call read-only ContractManagement
// contract's methods.
type ContractReader struct {
	invoker Invoker
}

// Contract represents a ContractManagement contract client that can be used to
// invoke all of its methods except 'update' and 'destroy' because they can be
// called successfully only from the contract itself (that is doing an update
// or self-destruction).
type Contract struct {
	ContractReader

	actor Actor
}

// Hash stores the hash of the native ContractManagement contract.
var Hash = state.CreateNativeContractHash(nativenames.Management)

// Event is the event emitted on contract deployment/update/destroy.
// Even though these events are different they all have the same field inside.
type Event struct {
	Hash util.Uint160
}

const setMinFeeMethod = "setMinimumDeploymentFee"

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

// GetContract allows to get contract data from its hash. This method is mostly
// useful for historic invocations since for current contracts there is a direct
// getcontractstate RPC API that has more options and works faster than going
// via contract invocation.
func (c *ContractReader) GetContract(hash util.Uint160) (*state.Contract, error) {
	itm, err := unwrap.Item(c.invoker.Call(Hash, "getContract", hash))
	if err != nil {
		return nil, err
	}
	res := new(state.Contract)
	err = res.FromStackItem(itm)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GetMinimumDeploymentFee returns the minimal amount of GAS needed to deploy a
// contract on the network.
func (c *ContractReader) GetMinimumDeploymentFee() (*big.Int, error) {
	return unwrap.BigInt(c.invoker.Call(Hash, "getMinimumDeploymentFee"))
}

// HasMethod checks if the contract specified has a method with the given name
// and number of parameters.
func (c *ContractReader) HasMethod(hash util.Uint160, method string, pcount int) (bool, error) {
	return unwrap.Bool(c.invoker.Call(Hash, "hasMethod", hash, method, pcount))
}

// Deploy creates and sends to the network a transaction that deploys the given
// contract (with the manifest provided), if data is not nil then it also added
// to the invocation and will be used for "_deploy" method invocation done by
// the ContractManagement contract. If successful, this method returns deployed
// contract state that can be retrieved from the stack after execution.
func (c *Contract) Deploy(exe *nef.File, manif *manifest.Manifest, data interface{}) (util.Uint256, uint32, error) {
	script, err := mkDeployScript(exe, manif, data)
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return c.actor.SendRun(script)
}

// DeployTransaction creates and returns a transaction that deploys the given
// contract (with the manifest provided), if data is not nil then it also added
// to the invocation and will be used for "_deploy" method invocation done by
// the ContractManagement contract. If successful, this method returns deployed
// contract state that can be retrieved from the stack after execution.
func (c *Contract) DeployTransaction(exe *nef.File, manif *manifest.Manifest, data interface{}) (*transaction.Transaction, error) {
	script, err := mkDeployScript(exe, manif, data)
	if err != nil {
		return nil, err
	}
	return c.actor.MakeRun(script)
}

// DeployUnsigned creates and returns an unsigned transaction that deploys the given
// contract (with the manifest provided), if data is not nil then it also added
// to the invocation and will be used for "_deploy" method invocation done by
// the ContractManagement contract. If successful, this method returns deployed
// contract state that can be retrieved from the stack after execution.
func (c *Contract) DeployUnsigned(exe *nef.File, manif *manifest.Manifest, data interface{}) (*transaction.Transaction, error) {
	script, err := mkDeployScript(exe, manif, data)
	if err != nil {
		return nil, err
	}
	return c.actor.MakeUnsignedRun(script, nil)
}

func mkDeployScript(exe *nef.File, manif *manifest.Manifest, data interface{}) ([]byte, error) {
	exeB, err := exe.Bytes()
	if err != nil {
		return nil, fmt.Errorf("bad NEF: %w", err)
	}
	manifB, err := json.Marshal(manif)
	if err != nil {
		return nil, fmt.Errorf("bad manifest: %w", err)
	}
	if data != nil {
		return smartcontract.CreateCallScript(Hash, "deploy", exeB, manifB, data)
	}
	return smartcontract.CreateCallScript(Hash, "deploy", exeB, manifB)
}

// SetMinimumDeploymentFee creates and sends a transaction that changes the
// minimum GAS amount required to deploy a contract. This method can be called
// successfully only by the network's committee, so make sure you're using an
// appropriate Actor. This invocation returns nothing and is successful when
// transactions ends up in the HALT state.
func (c *Contract) SetMinimumDeploymentFee(value *big.Int) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, setMinFeeMethod, value)
}

// SetMinimumDeploymentFeeTransaction creates a transaction that changes the
// minimum GAS amount required to deploy a contract. This method can be called
// successfully only by the network's committee, so make sure you're using an
// appropriate Actor. This invocation returns nothing and is successful when
// transactions ends up in the HALT state. The transaction returned is signed,
// but not sent to the network.
func (c *Contract) SetMinimumDeploymentFeeTransaction(value *big.Int) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, setMinFeeMethod, value)
}

// SetMinimumDeploymentFeeUnsigned creates a transaction that changes the
// minimum GAS amount required to deploy a contract. This method can be called
// successfully only by the network's committee, so make sure you're using an
// appropriate Actor. This invocation returns nothing and is successful when
// transactions ends up in the HALT state. The transaction returned is not
// signed.
func (c *Contract) SetMinimumDeploymentFeeUnsigned(value *big.Int) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, setMinFeeMethod, nil, value)
}
