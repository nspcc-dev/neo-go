// Package nameservice contains RPC wrappers for NameService contract.
package nameservice

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep11"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"math/big"
)

// Hash contains contract hash.
var Hash = util.Uint160{0xde, 0x46, 0x5f, 0x5d, 0x50, 0x57, 0xcf, 0x33, 0x28, 0x47, 0x94, 0xc5, 0xcf, 0xc2, 0xc, 0x69, 0x37, 0x1c, 0xac, 0x50}

// TransferEvent represents "Transfer" event emitted by the contract.
type TransferEvent struct {
	From util.Uint160
	To util.Uint160
	Amount *big.Int
	TokenId []byte
}

// SetAdminEvent represents "SetAdmin" event emitted by the contract.
type SetAdminEvent struct {
	Name string
	OldAdmin util.Uint160
	NewAdmin util.Uint160
}

// RenewEvent represents "Renew" event emitted by the contract.
type RenewEvent struct {
	Name string
	OldExpiration *big.Int
	NewExpiration *big.Int
}

// Invoker is used by ContractReader to call various safe methods.
type Invoker interface {
	nep11.Invoker
}

// Actor is used by Contract to call state-changing methods.
type Actor interface {
	Invoker

	nep11.Actor

	MakeCall(contract util.Uint160, method string, params ...any) (*transaction.Transaction, error)
	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...any) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendCall(contract util.Uint160, method string, params ...any) (util.Uint256, uint32, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
}

// ContractReader implements safe contract methods.
type ContractReader struct {
	nep11.NonDivisibleReader
	invoker Invoker
}

// Contract implements all contract methods.
type Contract struct {
	ContractReader
	nep11.BaseWriter
	actor Actor
}

// NewReader creates an instance of ContractReader using Hash and the given Invoker.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{*nep11.NewNonDivisibleReader(invoker, Hash), invoker}
}

// New creates an instance of Contract using Hash and the given Actor.
func New(actor Actor) *Contract {
	var nep11ndt = nep11.NewNonDivisible(actor, Hash)
	return &Contract{ContractReader{nep11ndt.NonDivisibleReader, actor}, nep11ndt.BaseWriter, actor}
}

// Roots invokes `roots` method of contract.
func (c *ContractReader) Roots() (uuid.UUID, result.Iterator, error) {
	return unwrap.SessionIterator(c.invoker.Call(Hash, "roots"))
}

// RootsExpanded is similar to Roots (uses the same contract
// method), but can be useful if the server used doesn't support sessions and
// doesn't expand iterators. It creates a script that will get the specified
// number of result items from the iterator right in the VM and return them to
// you. It's only limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) RootsExpanded(_numOfIteratorItems int) ([]stackitem.Item, error) {
	return unwrap.Array(c.invoker.CallAndExpandIterator(Hash, "roots", _numOfIteratorItems))
}

// GetPrice invokes `getPrice` method of contract.
func (c *ContractReader) GetPrice(length *big.Int) (*big.Int, error) {
	return unwrap.BigInt(c.invoker.Call(Hash, "getPrice", length))
}

// IsAvailable invokes `isAvailable` method of contract.
func (c *ContractReader) IsAvailable(name string) (bool, error) {
	return unwrap.Bool(c.invoker.Call(Hash, "isAvailable", name))
}

// GetRecord invokes `getRecord` method of contract.
func (c *ContractReader) GetRecord(name string, typev *big.Int) (string, error) {
	return unwrap.UTF8String(c.invoker.Call(Hash, "getRecord", name, typev))
}

// GetAllRecords invokes `getAllRecords` method of contract.
func (c *ContractReader) GetAllRecords(name string) (uuid.UUID, result.Iterator, error) {
	return unwrap.SessionIterator(c.invoker.Call(Hash, "getAllRecords", name))
}

// GetAllRecordsExpanded is similar to GetAllRecords (uses the same contract
// method), but can be useful if the server used doesn't support sessions and
// doesn't expand iterators. It creates a script that will get the specified
// number of result items from the iterator right in the VM and return them to
// you. It's only limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) GetAllRecordsExpanded(name string, _numOfIteratorItems int) ([]stackitem.Item, error) {
	return unwrap.Array(c.invoker.CallAndExpandIterator(Hash, "getAllRecords", _numOfIteratorItems, name))
}

// Resolve invokes `resolve` method of contract.
func (c *ContractReader) Resolve(name string, typev *big.Int) (string, error) {
	return unwrap.UTF8String(c.invoker.Call(Hash, "resolve", name, typev))
}

// Update creates a transaction invoking `update` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Update(nef []byte, manifest string) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "update", nef, manifest)
}

// UpdateTransaction creates a transaction invoking `update` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) UpdateTransaction(nef []byte, manifest string) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "update", nef, manifest)
}

// UpdateUnsigned creates a transaction invoking `update` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) UpdateUnsigned(nef []byte, manifest string) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "update", nil, nef, manifest)
}

// AddRoot creates a transaction invoking `addRoot` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) AddRoot(root string) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "addRoot", root)
}

// AddRootTransaction creates a transaction invoking `addRoot` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) AddRootTransaction(root string) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "addRoot", root)
}

// AddRootUnsigned creates a transaction invoking `addRoot` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) AddRootUnsigned(root string) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "addRoot", nil, root)
}

// SetPrice creates a transaction invoking `setPrice` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) SetPrice(priceList []any) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "setPrice", priceList)
}

// SetPriceTransaction creates a transaction invoking `setPrice` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) SetPriceTransaction(priceList []any) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "setPrice", priceList)
}

// SetPriceUnsigned creates a transaction invoking `setPrice` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) SetPriceUnsigned(priceList []any) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "setPrice", nil, priceList)
}

func scriptForRegister(name string, owner util.Uint160) ([]byte, error) {
	return smartcontract.CreateCallWithAssertScript(Hash, "register", name, owner)
}

// Register creates a transaction invoking `register` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Register(name string, owner util.Uint160) (util.Uint256, uint32, error) {
	script, err := scriptForRegister(name, owner)
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return c.actor.SendRun(script)
}

// RegisterTransaction creates a transaction invoking `register` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) RegisterTransaction(name string, owner util.Uint160) (*transaction.Transaction, error) {
	script, err := scriptForRegister(name, owner)
	if err != nil {
		return nil, err
	}
	return c.actor.MakeRun(script)
}

// RegisterUnsigned creates a transaction invoking `register` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) RegisterUnsigned(name string, owner util.Uint160) (*transaction.Transaction, error) {
	script, err := scriptForRegister(name, owner)
	if err != nil {
		return nil, err
	}
	return c.actor.MakeUnsignedRun(script, nil)
}

// Renew creates a transaction invoking `renew` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Renew(name string) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "renew", name)
}

// RenewTransaction creates a transaction invoking `renew` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) RenewTransaction(name string) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "renew", name)
}

// RenewUnsigned creates a transaction invoking `renew` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) RenewUnsigned(name string) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "renew", nil, name)
}

// Renew_2 creates a transaction invoking `renew` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Renew_2(name string, years *big.Int) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "renew", name, years)
}

// Renew_2Transaction creates a transaction invoking `renew` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) Renew_2Transaction(name string, years *big.Int) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "renew", name, years)
}

// Renew_2Unsigned creates a transaction invoking `renew` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) Renew_2Unsigned(name string, years *big.Int) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "renew", nil, name, years)
}

// SetAdmin creates a transaction invoking `setAdmin` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) SetAdmin(name string, admin util.Uint160) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "setAdmin", name, admin)
}

// SetAdminTransaction creates a transaction invoking `setAdmin` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) SetAdminTransaction(name string, admin util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "setAdmin", name, admin)
}

// SetAdminUnsigned creates a transaction invoking `setAdmin` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) SetAdminUnsigned(name string, admin util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "setAdmin", nil, name, admin)
}

// SetRecord creates a transaction invoking `setRecord` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) SetRecord(name string, typev *big.Int, data string) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "setRecord", name, typev, data)
}

// SetRecordTransaction creates a transaction invoking `setRecord` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) SetRecordTransaction(name string, typev *big.Int, data string) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "setRecord", name, typev, data)
}

// SetRecordUnsigned creates a transaction invoking `setRecord` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) SetRecordUnsigned(name string, typev *big.Int, data string) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "setRecord", nil, name, typev, data)
}

// DeleteRecord creates a transaction invoking `deleteRecord` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) DeleteRecord(name string, typev *big.Int) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, "deleteRecord", name, typev)
}

// DeleteRecordTransaction creates a transaction invoking `deleteRecord` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) DeleteRecordTransaction(name string, typev *big.Int) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, "deleteRecord", name, typev)
}

// DeleteRecordUnsigned creates a transaction invoking `deleteRecord` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) DeleteRecordUnsigned(name string, typev *big.Int) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, "deleteRecord", nil, name, typev)
}

// TransferEventsFromApplicationLog retrieves a set of all emitted events
// with "Transfer" name from the provided ApplicationLog.
func TransferEventsFromApplicationLog(log *result.ApplicationLog) ([]*TransferEvent, error) {
	if log == nil {
		return nil, errors.New("nil application log")
	}

	var res []*TransferEvent
	for i, ex := range log.Executions {
		for j, e := range ex.Events {
			if e.Name != "Transfer" {
				continue
			}
			event := new(TransferEvent)
			err := event.FromStackItem(e.Item)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize TransferEvent from stackitem (execution %d, event %d): %w", i, j, err)
			}
			res = append(res, event)
		}
	}

	return res, nil
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
	e.TokenId, err = arr[index].TryBytes()
	if err != nil {
		return fmt.Errorf("field TokenId: %w", err)
	}

	return nil
}

// SetAdminEventsFromApplicationLog retrieves a set of all emitted events
// with "SetAdmin" name from the provided ApplicationLog.
func SetAdminEventsFromApplicationLog(log *result.ApplicationLog) ([]*SetAdminEvent, error) {
	if log == nil {
		return nil, errors.New("nil application log")
	}

	var res []*SetAdminEvent
	for i, ex := range log.Executions {
		for j, e := range ex.Events {
			if e.Name != "SetAdmin" {
				continue
			}
			event := new(SetAdminEvent)
			err := event.FromStackItem(e.Item)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize SetAdminEvent from stackitem (execution %d, event %d): %w", i, j, err)
			}
			res = append(res, event)
		}
	}

	return res, nil
}

// FromStackItem converts provided stackitem.Array to SetAdminEvent and
// returns an error if so.
func (e *SetAdminEvent) FromStackItem(item *stackitem.Array) error {
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
	e.Name, err = func (item stackitem.Item) (string, error) {
		b, err := item.TryBytes()
		if err != nil {
			return "", err
		}
		if !utf8.Valid(b) {
			return "", errors.New("not a UTF-8 string")
		}
		return string(b), nil
	} (arr[index])
	if err != nil {
		return fmt.Errorf("field Name: %w", err)
	}

	index++
	e.OldAdmin, err = func (item stackitem.Item) (util.Uint160, error) {
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
		return fmt.Errorf("field OldAdmin: %w", err)
	}

	index++
	e.NewAdmin, err = func (item stackitem.Item) (util.Uint160, error) {
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
		return fmt.Errorf("field NewAdmin: %w", err)
	}

	return nil
}

// RenewEventsFromApplicationLog retrieves a set of all emitted events
// with "Renew" name from the provided ApplicationLog.
func RenewEventsFromApplicationLog(log *result.ApplicationLog) ([]*RenewEvent, error) {
	if log == nil {
		return nil, errors.New("nil application log")
	}

	var res []*RenewEvent
	for i, ex := range log.Executions {
		for j, e := range ex.Events {
			if e.Name != "Renew" {
				continue
			}
			event := new(RenewEvent)
			err := event.FromStackItem(e.Item)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize RenewEvent from stackitem (execution %d, event %d): %w", i, j, err)
			}
			res = append(res, event)
		}
	}

	return res, nil
}

// FromStackItem converts provided stackitem.Array to RenewEvent and
// returns an error if so.
func (e *RenewEvent) FromStackItem(item *stackitem.Array) error {
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
	e.Name, err = func (item stackitem.Item) (string, error) {
		b, err := item.TryBytes()
		if err != nil {
			return "", err
		}
		if !utf8.Valid(b) {
			return "", errors.New("not a UTF-8 string")
		}
		return string(b), nil
	} (arr[index])
	if err != nil {
		return fmt.Errorf("field Name: %w", err)
	}

	index++
	e.OldExpiration, err = arr[index].TryInteger()
	if err != nil {
		return fmt.Errorf("field OldExpiration: %w", err)
	}

	index++
	e.NewExpiration, err = arr[index].TryInteger()
	if err != nil {
		return fmt.Errorf("field NewExpiration: %w", err)
	}

	return nil
}
