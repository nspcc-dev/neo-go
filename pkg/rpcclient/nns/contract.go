// Package nns provide RPC wrappers for the non-native NNS contract.
// This is Neo N3 NNS contract wrapper, the source code of the contract can be found here:
// https://github.com/neo-project/non-native-contracts/blob/8d72b92e5e5705d763232bcc24784ced0fb8fc87/src/NameService/NameService.cs
package nns

import (
	"errors"
	"fmt"
	"math/big"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep11"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// MaxNameLength is the max length of domain name.
const MaxNameLength = 255

// SetAdminEvent represents "SetAdmin" event emitted by the contract.
type SetAdminEvent struct {
	Name     string
	OldAdmin util.Uint160
	NewAdmin util.Uint160
}

// RenewEvent represents "Renew" event emitted by the contract.
type RenewEvent struct {
	Name          string
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
	hash    util.Uint160
}

// Contract provides full NeoNameService interface, both safe and state-changing methods.
type Contract struct {
	ContractReader
	nep11.BaseWriter

	actor Actor
	hash  util.Uint160
}

// NewReader creates an instance of ContractReader using provided contract hash and the given Invoker.
func NewReader(invoker Invoker, hash util.Uint160) *ContractReader {
	return &ContractReader{*nep11.NewNonDivisibleReader(invoker, hash), invoker, hash}
}

// New creates an instance of Contract using provided contract hash and the given Actor.
func New(actor Actor, hash util.Uint160) *Contract {
	var nep11ndt = nep11.NewNonDivisible(actor, hash)
	return &Contract{ContractReader{nep11ndt.NonDivisibleReader, actor, hash}, nep11ndt.BaseWriter, actor, hash}
}

// Roots invokes `roots` method of contract.
func (c *ContractReader) Roots() (*RootIterator, error) {
	sess, iter, err := unwrap.SessionIterator(c.invoker.Call(c.hash, "roots"))
	if err != nil {
		return nil, err
	}

	return &RootIterator{
		client:   c.invoker,
		iterator: iter,
		session:  sess,
	}, nil
}

// RootsExpanded is similar to Roots (uses the same contract
// method), but can be useful if the server used doesn't support sessions and
// doesn't expand iterators. It creates a script that will get the specified
// number of result items from the iterator right in the VM and return them to
// you. It's only limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) RootsExpanded(_numOfIteratorItems int) ([]string, error) {
	arr, err := unwrap.Array(c.invoker.CallAndExpandIterator(c.hash, "roots", _numOfIteratorItems))
	if err != nil {
		return nil, err
	}

	return itemsToRoots(arr)
}

// GetPrice invokes `getPrice` method of contract.
func (c *ContractReader) GetPrice(length uint8) (*big.Int, error) {
	return unwrap.BigInt(c.invoker.Call(c.hash, "getPrice", length))
}

// IsAvailable invokes `isAvailable` method of contract.
func (c *ContractReader) IsAvailable(name string) (bool, error) {
	return unwrap.Bool(c.invoker.Call(c.hash, "isAvailable", name))
}

// GetRecord invokes `getRecord` method of contract.
func (c *ContractReader) GetRecord(name string, typev RecordType) (string, error) {
	return unwrap.UTF8String(c.invoker.Call(c.hash, "getRecord", name, typev))
}

// GetAllRecords invokes `getAllRecords` method of contract.
func (c *ContractReader) GetAllRecords(name string) (*RecordIterator, error) {
	sess, iter, err := unwrap.SessionIterator(c.invoker.Call(c.hash, "getAllRecords", name))
	if err != nil {
		return nil, err
	}

	return &RecordIterator{
		client:   c.invoker,
		iterator: iter,
		session:  sess,
	}, nil
}

// GetAllRecordsExpanded is similar to GetAllRecords (uses the same contract
// method), but can be useful if the server used doesn't support sessions and
// doesn't expand iterators. It creates a script that will get the specified
// number of result items from the iterator right in the VM and return them to
// you. It's only limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) GetAllRecordsExpanded(name string, _numOfIteratorItems int) ([]RecordState, error) {
	arr, err := unwrap.Array(c.invoker.CallAndExpandIterator(c.hash, "getAllRecords", _numOfIteratorItems, name))
	if err != nil {
		return nil, err
	}
	return itemsToRecords(arr)
}

// Resolve invokes `resolve` method of contract.
func (c *ContractReader) Resolve(name string, typev RecordType) (string, error) {
	return unwrap.UTF8String(c.invoker.Call(c.hash, "resolve", name, int64(typev)))
}

// Update creates a transaction invoking `update` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Update(nef []byte, manifest string) (util.Uint256, uint32, error) {
	return c.actor.SendCall(c.hash, "update", nef, manifest)
}

// UpdateTransaction creates a transaction invoking `update` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) UpdateTransaction(nef []byte, manifest string) (*transaction.Transaction, error) {
	return c.actor.MakeCall(c.hash, "update", nef, manifest)
}

// UpdateUnsigned creates a transaction invoking `update` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) UpdateUnsigned(nef []byte, manifest string) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(c.hash, "update", nil, nef, manifest)
}

// AddRoot creates a transaction invoking `addRoot` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) AddRoot(root string) (util.Uint256, uint32, error) {
	return c.actor.SendCall(c.hash, "addRoot", root)
}

// AddRootTransaction creates a transaction invoking `addRoot` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) AddRootTransaction(root string) (*transaction.Transaction, error) {
	return c.actor.MakeCall(c.hash, "addRoot", root)
}

// AddRootUnsigned creates a transaction invoking `addRoot` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) AddRootUnsigned(root string) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(c.hash, "addRoot", nil, root)
}

// SetPrice creates a transaction invoking `setPrice` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) SetPrice(priceList []int64) (util.Uint256, uint32, error) {
	anyPriceList := make([]any, len(priceList))
	for i, price := range priceList {
		anyPriceList[i] = price
	}
	return c.actor.SendCall(c.hash, "setPrice", anyPriceList)
}

// SetPriceTransaction creates a transaction invoking `setPrice` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) SetPriceTransaction(priceList []int64) (*transaction.Transaction, error) {
	anyPriceList := make([]any, len(priceList))
	for i, price := range priceList {
		anyPriceList[i] = price
	}
	return c.actor.MakeCall(c.hash, "setPrice", anyPriceList)
}

// SetPriceUnsigned creates a transaction invoking `setPrice` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) SetPriceUnsigned(priceList []int64) (*transaction.Transaction, error) {
	anyPriceList := make([]any, len(priceList))
	for i, price := range priceList {
		anyPriceList[i] = price
	}
	return c.actor.MakeUnsignedCall(c.hash, "setPrice", nil, anyPriceList)
}

func (c *Contract) scriptForRegister(name string, owner util.Uint160) ([]byte, error) {
	return smartcontract.CreateCallWithAssertScript(c.hash, "register", name, owner)
}

// Register creates a transaction invoking `register` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Register(name string, owner util.Uint160) (util.Uint256, uint32, error) {
	script, err := c.scriptForRegister(name, owner)
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return c.actor.SendRun(script)
}

// RegisterTransaction creates a transaction invoking `register` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) RegisterTransaction(name string, owner util.Uint160) (*transaction.Transaction, error) {
	script, err := c.scriptForRegister(name, owner)
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
	script, err := c.scriptForRegister(name, owner)
	if err != nil {
		return nil, err
	}
	return c.actor.MakeUnsignedRun(script, nil)
}

// Renew creates a transaction invoking `renew` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Renew(name string) (util.Uint256, uint32, error) {
	return c.actor.SendCall(c.hash, "renew", name)
}

// RenewTransaction creates a transaction invoking `renew` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) RenewTransaction(name string) (*transaction.Transaction, error) {
	return c.actor.MakeCall(c.hash, "renew", name)
}

// RenewUnsigned creates a transaction invoking `renew` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) RenewUnsigned(name string) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(c.hash, "renew", nil, name)
}

// Renew2 creates a transaction invoking `renew` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) Renew2(name string, years int64) (util.Uint256, uint32, error) {
	return c.actor.SendCall(c.hash, "renew", name, years)
}

// Renew2Transaction creates a transaction invoking `renew` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) Renew2Transaction(name string, years int64) (*transaction.Transaction, error) {
	return c.actor.MakeCall(c.hash, "renew", name, years)
}

// Renew2Unsigned creates a transaction invoking `renew` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) Renew2Unsigned(name string, years int64) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(c.hash, "renew", nil, name, years)
}

// SetAdmin creates a transaction invoking `setAdmin` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) SetAdmin(name string, admin util.Uint160) (util.Uint256, uint32, error) {
	return c.actor.SendCall(c.hash, "setAdmin", name, admin)
}

// SetAdminTransaction creates a transaction invoking `setAdmin` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) SetAdminTransaction(name string, admin util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeCall(c.hash, "setAdmin", name, admin)
}

// SetAdminUnsigned creates a transaction invoking `setAdmin` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) SetAdminUnsigned(name string, admin util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(c.hash, "setAdmin", nil, name, admin)
}

// SetRecord creates a transaction invoking `setRecord` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) SetRecord(name string, typev RecordType, data string) (util.Uint256, uint32, error) {
	return c.actor.SendCall(c.hash, "setRecord", name, typev, data)
}

// SetRecordTransaction creates a transaction invoking `setRecord` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) SetRecordTransaction(name string, typev RecordType, data string) (*transaction.Transaction, error) {
	return c.actor.MakeCall(c.hash, "setRecord", name, typev, data)
}

// SetRecordUnsigned creates a transaction invoking `setRecord` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) SetRecordUnsigned(name string, typev RecordType, data string) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(c.hash, "setRecord", nil, name, typev, data)
}

// DeleteRecord creates a transaction invoking `deleteRecord` method of the contract.
// This transaction is signed and immediately sent to the network.
// The values returned are its hash, ValidUntilBlock value and error if any.
func (c *Contract) DeleteRecord(name string, typev RecordType) (util.Uint256, uint32, error) {
	return c.actor.SendCall(c.hash, "deleteRecord", name, typev)
}

// DeleteRecordTransaction creates a transaction invoking `deleteRecord` method of the contract.
// This transaction is signed, but not sent to the network, instead it's
// returned to the caller.
func (c *Contract) DeleteRecordTransaction(name string, typev RecordType) (*transaction.Transaction, error) {
	return c.actor.MakeCall(c.hash, "deleteRecord", name, typev)
}

// DeleteRecordUnsigned creates a transaction invoking `deleteRecord` method of the contract.
// This transaction is not signed, it's simply returned to the caller.
// Any fields of it that do not affect fees can be changed (ValidUntilBlock,
// Nonce), fee values (NetworkFee, SystemFee) can be increased as well.
func (c *Contract) DeleteRecordUnsigned(name string, typev RecordType) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(c.hash, "deleteRecord", nil, name, typev)
}

// SetAdminEventsFromApplicationLog retrieves a set of all emitted events
// with "SetAdmin" name from the provided [result.ApplicationLog].
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
				return nil, fmt.Errorf("failed to deserialize SetAdminEvent from stackitem (execution #%d, event #%d): %w", i, j, err)
			}
			res = append(res, event)
		}
	}

	return res, nil
}

// FromStackItem converts provided [stackitem.Array] to SetAdminEvent or
// returns an error if it's not possible to do to so.
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
		err   error
	)
	index++
	e.Name, err = func(item stackitem.Item) (string, error) {
		b, err := item.TryBytes()
		if err != nil {
			return "", err
		}
		if !utf8.Valid(b) {
			return "", errors.New("not a UTF-8 string")
		}
		return string(b), nil
	}(arr[index])
	if err != nil {
		return fmt.Errorf("field Name: %w", err)
	}

	index++
	e.OldAdmin, err = func(item stackitem.Item) (util.Uint160, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint160{}, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return util.Uint160{}, err
		}
		return u, nil
	}(arr[index])
	if err != nil {
		return fmt.Errorf("field OldAdmin: %w", err)
	}

	index++
	e.NewAdmin, err = func(item stackitem.Item) (util.Uint160, error) {
		b, err := item.TryBytes()
		if err != nil {
			return util.Uint160{}, err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return util.Uint160{}, err
		}
		return u, nil
	}(arr[index])
	if err != nil {
		return fmt.Errorf("field NewAdmin: %w", err)
	}

	return nil
}

// RenewEventsFromApplicationLog retrieves a set of all emitted events
// with "Renew" name from the provided [result.ApplicationLog].
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
				return nil, fmt.Errorf("failed to deserialize RenewEvent from stackitem (execution #%d, event #%d): %w", i, j, err)
			}
			res = append(res, event)
		}
	}

	return res, nil
}

// FromStackItem converts provided [stackitem.Array] to RenewEvent or
// returns an error if it's not possible to do to so.
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
		err   error
	)
	index++
	e.Name, err = func(item stackitem.Item) (string, error) {
		b, err := item.TryBytes()
		if err != nil {
			return "", err
		}
		if !utf8.Valid(b) {
			return "", errors.New("not a UTF-8 string")
		}
		return string(b), nil
	}(arr[index])
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
