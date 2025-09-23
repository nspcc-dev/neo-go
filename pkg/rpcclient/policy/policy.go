/*
Package policy allows to work with the native PolicyContract contract via RPC.

Safe methods are encapsulated into ContractReader structure while Contract provides
various methods to perform PolicyContract state-changing calls.
*/
package policy

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Invoker is used by ContractReader to call various methods.
type Invoker interface {
	Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error)
	CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...any) (*result.Invoke, error)
	TerminateSession(sessionID uuid.UUID) error
	TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error)
}

// Actor is used by Contract to create and send transactions.
type Actor interface {
	Invoker

	MakeCall(contract util.Uint160, method string, params ...any) (*transaction.Transaction, error)
	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...any) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendCall(contract util.Uint160, method string, params ...any) (util.Uint256, uint32, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
}

// Hash stores the hash of the native PolicyContract contract.
var Hash = nativehashes.PolicyContract

const (
	execFeeSetter                     = "setExecFeeFactor"
	feePerByteSetter                  = "setFeePerByte"
	storagePriceSetter                = "setStoragePrice"
	attributeFeeSetter                = "setAttributeFee"
	maxValidUntilBlockIncrementSetter = "setMaxValidUntilBlockIncrement"
	millisecondsPerBlockSetter        = "setMillisecondsPerBlock"
)

// ContractReader provides an interface to call read-only PolicyContract
// contract's methods.
type ContractReader struct {
	invoker Invoker
}

// Contract represents a PolicyContract contract client that can be used to
// invoke all of its methods.
type Contract struct {
	ContractReader

	actor Actor
}

// BlockedAccountsIterator is used for iterating over GetBlockedAccounts results.
type BlockedAccountsIterator struct {
	client   Invoker
	session  uuid.UUID
	iterator result.Iterator
}

// NewReader creates an instance of ContractReader that can be used to read
// data from the contract.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{invoker}
}

// New creates an instance of Contract to perform actions using
// the given Actor. Notice that PolicyContract's state can be changed
// only by the network's committee, so the Actor provided must be a committee
// actor for all methods to work properly.
func New(actor Actor) *Contract {
	return &Contract{*NewReader(actor), actor}
}

// GetExecFeeFactor returns current execution fee factor used by the network.
// This setting affects all executions of all transactions.
func (c *ContractReader) GetExecFeeFactor() (int64, error) {
	return unwrap.Int64(c.invoker.Call(Hash, "getExecFeeFactor"))
}

// GetFeePerByte returns current minimal per-byte network fee value which
// affects all transactions on the network.
func (c *ContractReader) GetFeePerByte() (int64, error) {
	return unwrap.Int64(c.invoker.Call(Hash, "getFeePerByte"))
}

// GetStoragePrice returns current per-byte storage price. Any contract saving
// data to the storage pays for it according to this value.
func (c *ContractReader) GetStoragePrice() (int64, error) {
	return unwrap.Int64(c.invoker.Call(Hash, "getStoragePrice"))
}

// GetAttributeFee returns current fee for the specified attribute usage. Any
// contract saving data to the storage pays for it according to this value.
func (c *ContractReader) GetAttributeFee(t transaction.AttrType) (int64, error) {
	return unwrap.Int64(c.invoker.Call(Hash, "getAttributeFee", byte(t)))
}

// GetMaxValidUntilBlockIncrement returns current MaxValidUntilBlockIncrement
// setting. Note that this method is available starting from Echidna hardfork.
func (c *ContractReader) GetMaxValidUntilBlockIncrement() (int64, error) {
	return unwrap.Int64(c.invoker.Call(Hash, "getMaxValidUntilBlockIncrement"))
}

// GetMillisecondsPerBlock returns current MillisecondsPerBlock setting. Note
// that this method is available starting from Echidna hardfork.
func (c *ContractReader) GetMillisecondsPerBlock() (int64, error) {
	return unwrap.Int64(c.invoker.Call(Hash, "getMillisecondsPerBlock"))
}

// GetBlockedAccounts returns current blocked accounts. Note that this method is
// available starting from [config.HFFaun] hardfork.
func (c *ContractReader) GetBlockedAccounts() (*BlockedAccountsIterator, error) {
	sess, iter, err := unwrap.SessionIterator(c.invoker.Call(Hash, "getBlockedAccounts"))
	if err != nil {
		return nil, err
	}

	return &BlockedAccountsIterator{
		client:   c.invoker,
		iterator: iter,
		session:  sess,
	}, nil
}

// GetBlockedAccountsExpanded is similar to GetBlockedAccounts (uses the same
// contract method), but can be useful if the server used doesn't support
// sessions and doesn't expand iterators. It creates a script that will get num
// of result items from the iterator right in the VM and return them to you. It's
// only limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) GetBlockedAccountsExpanded(num int) ([]util.Uint160, error) {
	return unwrap.ArrayOfUint160(c.invoker.CallAndExpandIterator(Hash, "getBlockedAccounts", num))
}

// Next returns the next set of elements from the iterator (up to num of them).
// It can return less than num elements in case iterator doesn't have that many
// or zero elements if the iterator has no more elements or the session is
// expired.
func (b *BlockedAccountsIterator) Next(num int) ([]util.Uint160, error) {
	items, err := b.client.TraverseIterator(b.session, &b.iterator, num)
	if err != nil {
		return nil, err
	}
	res := make([]util.Uint160, len(items))
	for i, itm := range items {
		hb, err := itm.TryBytes()
		if err != nil {
			return nil, fmt.Errorf("item #%d has wrong hash: %w", i, err)
		}
		res[i], err = util.Uint160DecodeBytesBE(hb)
		if err != nil {
			return nil, fmt.Errorf("item #%d has wrong hash: %w", i, err)
		}
	}
	return res, nil
}

// Terminate closes the iterator session used by BlockedAccountsIterator (if it's
// session-based).
func (b *BlockedAccountsIterator) Terminate() error {
	if b.iterator.ID == nil {
		return nil
	}
	return b.client.TerminateSession(b.session)
}

// IsBlocked checks if the given account is blocked in the PolicyContract.
func (c *ContractReader) IsBlocked(account util.Uint160) (bool, error) {
	return unwrap.Bool(c.invoker.Call(Hash, "isBlocked", account))
}

// SetExecFeeFactor creates and sends a transaction that sets the new
// execution fee factor for the network to use. The action is successful when
// transaction ends in HALT state. The returned values are transaction hash, its
// ValidUntilBlock value and an error if any.
func (c *Contract) SetExecFeeFactor(value int64) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, execFeeSetter, value)
}

// SetExecFeeFactorTransaction creates a transaction that sets the new execution
// fee factor. This transaction is signed, but not sent to the network,
// instead it's returned to the caller.
func (c *Contract) SetExecFeeFactorTransaction(value int64) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, execFeeSetter, value)
}

// SetExecFeeFactorUnsigned creates a transaction that sets the new execution
// fee factor. This transaction is not signed and just returned to the caller.
func (c *Contract) SetExecFeeFactorUnsigned(value int64) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, execFeeSetter, nil, value)
}

// SetFeePerByte creates and sends a transaction that sets the new minimal
// per-byte network fee value. The action is successful when transaction ends in
// HALT state. The returned values are transaction hash, its ValidUntilBlock
// value and an error if any.
func (c *Contract) SetFeePerByte(value int64) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, feePerByteSetter, value)
}

// SetFeePerByteTransaction creates a transaction that sets the new minimal
// per-byte network fee value. This transaction is signed, but not sent to the
// network, instead it's returned to the caller.
func (c *Contract) SetFeePerByteTransaction(value int64) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, feePerByteSetter, value)
}

// SetFeePerByteUnsigned creates a transaction that sets the new minimal per-byte
// network fee value. This transaction is not signed and just returned to the
// caller.
func (c *Contract) SetFeePerByteUnsigned(value int64) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, feePerByteSetter, nil, value)
}

// SetStoragePrice creates and sends a transaction that sets the storage price
// for contracts. The action is successful when transaction ends in HALT
// state. The returned values are transaction hash, its ValidUntilBlock value
// and an error if any.
func (c *Contract) SetStoragePrice(value int64) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, storagePriceSetter, value)
}

// SetStoragePriceTransaction creates a transaction that sets the storage price
// for contracts. This transaction is signed, but not sent to the network,
// instead it's returned to the caller.
func (c *Contract) SetStoragePriceTransaction(value int64) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, storagePriceSetter, value)
}

// SetStoragePriceUnsigned creates a transaction that sets the storage price
// for contracts. This transaction is not signed and just returned to the
// caller.
func (c *Contract) SetStoragePriceUnsigned(value int64) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, storagePriceSetter, nil, value)
}

// SetAttributeFee creates and sends a transaction that sets the new attribute
// fee value for the specified attribute. The action is successful when
// transaction ends in HALT state. The returned values are transaction hash, its
// ValidUntilBlock value and an error if any.
func (c *Contract) SetAttributeFee(t transaction.AttrType, value int64) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, attributeFeeSetter, byte(t), value)
}

// SetAttributeFeeTransaction creates a transaction that sets the new attribute
// fee value for the specified attribute. This transaction is signed, but not
// sent to the network, instead it's returned to the caller.
func (c *Contract) SetAttributeFeeTransaction(t transaction.AttrType, value int64) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, attributeFeeSetter, byte(t), value)
}

// SetAttributeFeeUnsigned creates a transaction that sets the new attribute fee
// value for the specified attribute. This transaction is not signed and just
// returned to the caller.
func (c *Contract) SetAttributeFeeUnsigned(t transaction.AttrType, value int64) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, attributeFeeSetter, nil, byte(t), value)
}

// SetMaxValidUntilBlockIncrement creates and sends a transaction that sets the
// MaxValidUntilBlockIncrement protocol setting. The action is successful when
// transaction ends in HALT state. The returned values are transaction hash,
// its ValidUntilBlock value and an error if any. Note that this method is available
// starting from Echidna hardfork.
func (c *Contract) SetMaxValidUntilBlockIncrement(value int64) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, maxValidUntilBlockIncrementSetter, value)
}

// SetMaxValidUntilBlockIncrementTransaction creates a transaction that sets the
// MaxValidUntilBlockIncrement value. This transaction is signed, but not sent to
// the network, instead it's returned to the caller. Note that this method is available
// starting from Echidna hardfork.
func (c *Contract) SetMaxValidUntilBlockIncrementTransaction(value int64) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, maxValidUntilBlockIncrementSetter, value)
}

// SetMaxValidUntilBlockIncrementUnsigned creates a transaction that sets the
// MaxValidUntilBlockIncrement value. This transaction is not signed and just
// returned to the caller. Note that this method is available starting from
// Echidna hardfork.
func (c *Contract) SetMaxValidUntilBlockIncrementUnsigned(value int64) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, maxValidUntilBlockIncrementSetter, nil, value)
}

// SetMillisecondsPerBlock creates and sends a transaction that sets the
// block generation time in milliseconds. The action is successful when
// transaction ends in HALT state. The returned values are transaction hash,
// its ValidUntilBlock value and an error if any. Note that this method is
// available starting from Echidna hardfork.
func (c *Contract) SetMillisecondsPerBlock(value int64) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, millisecondsPerBlockSetter, value)
}

// SetMillisecondsPerBlockTransaction creates a transaction that sets the
// block generation time in milliseconds. This transaction is signed, but not
// sent to the network, instead it's returned to the caller. Note that this
// method is available starting from Echidna hardfork.
func (c *Contract) SetMillisecondsPerBlockTransaction(value int64) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, millisecondsPerBlockSetter, value)
}

// SetMillisecondsPerBlockUnsigned creates a transaction that sets the
// block generation time in milliseconds. This transaction is not signed and
// just returned to the caller. Note that this method is available starting
// from Echidna hardfork.
func (c *Contract) SetMillisecondsPerBlockUnsigned(value int64) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, millisecondsPerBlockSetter, nil, value)
}

// BlockAccount creates and sends a transaction that blocks an account on the
// network (via `blockAccount` method), it fails (with FAULT state) if it's not
// successful. The returned values are transaction hash, its
// ValidUntilBlock value and an error if any.
func (c *Contract) BlockAccount(account util.Uint160) (util.Uint256, uint32, error) {
	return c.actor.SendRun(blockScript(account))
}

// BlockAccountTransaction creates a transaction that blocks an account on the
// network and checks for the result of the appropriate call, failing the
// transaction if it's not true. This transaction is signed, but not sent to the
// network, instead it's returned to the caller.
func (c *Contract) BlockAccountTransaction(account util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeRun(blockScript(account))
}

// BlockAccountUnsigned creates a transaction that blocks an account on the
// network and checks for the result of the appropriate call, failing the
// transaction if it's not true. This transaction is not signed and just returned
// to the caller.
func (c *Contract) BlockAccountUnsigned(account util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedRun(blockScript(account), nil)
}

func blockScript(account util.Uint160) []byte {
	// We know parameters exactly (unlike with nep17.Transfer), so this can't fail.
	script, _ := smartcontract.CreateCallWithAssertScript(Hash, "blockAccount", account)
	return script
}

// UnblockAccount creates and sends a transaction that removes previously blocked
// account from the stop list. It uses `unblockAccount` method and checks for the
// result returned, failing the transaction if it's not true. The returned values
// are transaction hash, its ValidUntilBlock value and an error if any.
func (c *Contract) UnblockAccount(account util.Uint160) (util.Uint256, uint32, error) {
	return c.actor.SendRun(unblockScript(account))
}

// UnblockAccountTransaction creates a transaction that unblocks previously
// blocked account via `unblockAccount` method and checks for the result returned,
// failing the transaction if it's not true. This transaction is signed, but not
// sent to the network, instead it's returned to the caller.
func (c *Contract) UnblockAccountTransaction(account util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeRun(unblockScript(account))
}

// UnblockAccountUnsigned creates a transaction that unblocks the given account
// if it was blocked previously. It uses `unblockAccount` method and checks for
// its return value, failing the transaction if it's not true. This transaction
// is not signed and just returned to the caller.
func (c *Contract) UnblockAccountUnsigned(account util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedRun(unblockScript(account), nil)
}

func unblockScript(account util.Uint160) []byte {
	// We know parameters exactly (unlike with nep17.Transfer), so this can't fail.
	script, _ := smartcontract.CreateCallWithAssertScript(Hash, "unblockAccount", account)
	return script
}
