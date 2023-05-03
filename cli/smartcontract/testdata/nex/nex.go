// Package nextoken contains RPC wrappers for NEX Token contract.
package nextoken

import (
	"errors"
	"fmt"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"math/big"
)

// Hash contains contract hash.
var Hash = util.Uint160{0xa8, 0x1a, 0xa1, 0xf0, 0x4b, 0xf, 0xdc, 0x4a, 0xa2, 0xce, 0xd5, 0xbf, 0xc6, 0x22, 0xcf, 0xe8, 0x9, 0x7f, 0xa6, 0xa2}



// TransferEvent represents event emitted by the contract.
type TransferEvent struct {
	From util.Uint160
	To util.Uint160
	Amount *big.Int
}
// OnMintEvent represents event emitted by the contract.
type OnMintEvent struct {
	From util.Uint160
	To util.Uint160
	Amount *big.Int
	SwapId *big.Int
}
// Invoker is used by ContractReader to call various safe methods.
type Invoker interface {
	nep17.Invoker
}

// Actor is used by Contract to call state-changing methods.
type Actor interface {
	Invoker

	nep17.Actor

	MakeCall(contract util.Uint160, method string, params ...any) (*transaction.Transaction, error)
	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...any) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendCall(contract util.Uint160, method string, params ...any) (util.Uint256, uint32, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
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

// ChangeMinter creates a transaction invoking `changeMinter` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) ChangeMinter(newMinter *keys.PublicKey) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "changeMinter", newMinter)
}

// ChangeMinterTransaction creates a transaction invoking `changeMinter` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) ChangeMinterTransaction(newMinter *keys.PublicKey) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "changeMinter", newMinter)
}

// ChangeMinterUnsigned creates a transaction invoking `changeMinter` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) ChangeMinterUnsigned(newMinter *keys.PublicKey) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "changeMinter", nil, newMinter)
}

// ChangeOwner creates a transaction invoking `changeOwner` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) ChangeOwner(newOwner util.Uint160) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "changeOwner", newOwner)
}

// ChangeOwnerTransaction creates a transaction invoking `changeOwner` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) ChangeOwnerTransaction(newOwner util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "changeOwner", newOwner)
}

// ChangeOwnerUnsigned creates a transaction invoking `changeOwner` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) ChangeOwnerUnsigned(newOwner util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "changeOwner", nil, newOwner)
}

// Destroy creates a transaction invoking `destroy` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Destroy() (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "destroy")
}

// DestroyTransaction creates a transaction invoking `destroy` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) DestroyTransaction() (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "destroy")
}

// DestroyUnsigned creates a transaction invoking `destroy` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) DestroyUnsigned() (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "destroy", nil)
}

// MaxSupply creates a transaction invoking `maxSupply` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) MaxSupply() (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "maxSupply")
}

// MaxSupplyTransaction creates a transaction invoking `maxSupply` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) MaxSupplyTransaction() (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "maxSupply")
}

// MaxSupplyUnsigned creates a transaction invoking `maxSupply` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) MaxSupplyUnsigned() (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "maxSupply", nil)
}

// Mint creates a transaction invoking `mint` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Mint(from util.Uint160, to util.Uint160, amount *big.Int, swapId *big.Int, signature []byte, data any) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "mint", from, to, amount, swapId, signature, data)
}

// MintTransaction creates a transaction invoking `mint` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) MintTransaction(from util.Uint160, to util.Uint160, amount *big.Int, swapId *big.Int, signature []byte, data any) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "mint", from, to, amount, swapId, signature, data)
}

// MintUnsigned creates a transaction invoking `mint` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) MintUnsigned(from util.Uint160, to util.Uint160, amount *big.Int, swapId *big.Int, signature []byte, data any) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "mint", nil, from, to, amount, swapId, signature, data)
}

// Update creates a transaction invoking `update` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Update(nef []byte, manifest []byte) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "update", nef, manifest)
}

// UpdateTransaction creates a transaction invoking `update` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) UpdateTransaction(nef []byte, manifest []byte) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "update", nef, manifest)
}

// UpdateUnsigned creates a transaction invoking `update` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) UpdateUnsigned(nef []byte, manifest []byte) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "update", nil, nef, manifest)
}

// UpdateCap creates a transaction invoking `updateCap` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) UpdateCap(newCap *big.Int) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "updateCap", newCap)
}

// UpdateCapTransaction creates a transaction invoking `updateCap` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) UpdateCapTransaction(newCap *big.Int) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "updateCap", newCap)
}

// UpdateCapUnsigned creates a transaction invoking `updateCap` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) UpdateCapUnsigned(newCap *big.Int) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "updateCap", nil, newCap)
}

// TransferEventFromApplicationLog retrieves TransferEvent from the
// provided ApplicationLog located at the specified index in the events list
// of the specified execution.
func TransferEventFromApplicationLog(log *result.ApplicationLog, executionIdx, eventIdx int) (*TransferEvent, error) {
	if log == nil {
		return nil, errors.New("nil application log")
	}
	if len(log.Executions) < executionIdx+1 {
		return nil, fmt.Errorf("missing execution result: expected %d, got %d", executionIdx+1, len(log.Executions))
	}
	ex := log.Executions[executionIdx]
	if len(ex.Events) < eventIdx+1 {
		return nil, fmt.Errorf("missing event: expected %d, got %d", eventIdx+1, len(ex.Events))
	}
	e := ex.Events[eventIdx].Item

	res := new(TransferEvent)
	err := res.FromStackItem(e)
	return res, err
}

// FromStackItem converts provided stackitem.Array to TransferEvent and
// returns an error if so.
func (e *TransferEvent) FromStackItem(item *stackitem.Array) error {
	if item == nil {
		return errors.New("nil item")
	}
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != 3 {
		return errors.New("wrong number of structure elements")
	}

	var (
		index = -1
		err error
	)
	index++
	e.From, err = func (item stackitem.Item) (util.Uint160, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint160{}, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return util.Uint160{}, err
		}
		return u, nil
	} (arr[index])
	if err != nil {
		return fmt.Errorf("field From: %w", err)
	}
	
	index++
	e.To, err = func (item stackitem.Item) (util.Uint160, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint160{}, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return util.Uint160{}, err
		}
		return u, nil
	} (arr[index])
	if err != nil {
		return fmt.Errorf("field To: %w", err)
	}
	
	index++
	e.Amount, err = arr[index].TryInteger()
	if err != nil {
		return fmt.Errorf("field Amount: %w", err)
	}
	
	return nil
}

// OnMintEventFromApplicationLog retrieves OnMintEvent from the
// provided ApplicationLog located at the specified index in the events list
// of the specified execution.
func OnMintEventFromApplicationLog(log *result.ApplicationLog, executionIdx, eventIdx int) (*OnMintEvent, error) {
	if log == nil {
		return nil, errors.New("nil application log")
	}
	if len(log.Executions) < executionIdx+1 {
		return nil, fmt.Errorf("missing execution result: expected %d, got %d", executionIdx+1, len(log.Executions))
	}
	ex := log.Executions[executionIdx]
	if len(ex.Events) < eventIdx+1 {
		return nil, fmt.Errorf("missing event: expected %d, got %d", eventIdx+1, len(ex.Events))
	}
	e := ex.Events[eventIdx].Item

	res := new(OnMintEvent)
	err := res.FromStackItem(e)
	return res, err
}

// FromStackItem converts provided stackitem.Array to OnMintEvent and
// returns an error if so.
func (e *OnMintEvent) FromStackItem(item *stackitem.Array) error {
	if item == nil {
		return errors.New("nil item")
	}
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		return errors.New("not an array")
	}
	if len(arr) != 4 {
		return errors.New("wrong number of structure elements")
	}

	var (
		index = -1
		err error
	)
	index++
	e.From, err = func (item stackitem.Item) (util.Uint160, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint160{}, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return util.Uint160{}, err
		}
		return u, nil
	} (arr[index])
	if err != nil {
		return fmt.Errorf("field From: %w", err)
	}
	
	index++
	e.To, err = func (item stackitem.Item) (util.Uint160, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint160{}, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return util.Uint160{}, err
		}
		return u, nil
	} (arr[index])
	if err != nil {
		return fmt.Errorf("field To: %w", err)
	}
	
	index++
	e.Amount, err = arr[index].TryInteger()
	if err != nil {
		return fmt.Errorf("field Amount: %w", err)
	}
	
	index++
	e.SwapId, err = arr[index].TryInteger()
	if err != nil {
		return fmt.Errorf("field SwapId: %w", err)
	}
	
	return nil
}
