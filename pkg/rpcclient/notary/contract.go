/*
Package notary provides an RPC-based wrapper for the Notary subsystem.

It provides both regular ContractReader/Contract interfaces for the notary
contract and notary-specific Actor as well as some helper functions to simplify
creation of notary requests.
*/
package notary

import (
	"math"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	setMaxNVBDeltaMethod = "setMaxNotValidBeforeDelta"
	setFeePKMethod       = "setNotaryServiceFeePerKey"
)

// ContractInvoker is used by ContractReader to perform read-only calls.
type ContractInvoker interface {
	Call(contract util.Uint160, operation string, params ...interface{}) (*result.Invoke, error)
}

// ContractActor is used by Contract to create and send transactions.
type ContractActor interface {
	ContractInvoker

	MakeCall(contract util.Uint160, method string, params ...interface{}) (*transaction.Transaction, error)
	MakeRun(script []byte) (*transaction.Transaction, error)
	MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...interface{}) (*transaction.Transaction, error)
	MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendCall(contract util.Uint160, method string, params ...interface{}) (util.Uint256, uint32, error)
	SendRun(script []byte) (util.Uint256, uint32, error)
}

// ContractReader represents safe (read-only) methods of Notary. It can be
// used to query various data, but `verify` method is not exposed there because
// it can't be successful in standalone invocation (missing transaction with the
// NotaryAssisted attribute and its signature).
type ContractReader struct {
	invoker ContractInvoker
}

// Contract provides full Notary interface, both safe and state-changing methods.
// The only method omitted is onNEP17Payment which can only be called
// successfully from the GASToken native contract.
type Contract struct {
	ContractReader

	actor ContractActor
}

// OnNEP17PaymentData is the data set that is accepted by the notary contract
// onNEP17Payment handler. It's mandatory for GAS tranfers to this contract.
type OnNEP17PaymentData struct {
	// Account can be nil, in this case transfer sender (from) account is used.
	Account *util.Uint160
	// Till specifies the deposit lock time (in blocks).
	Till uint32
}

// Hash stores the hash of the native Notary contract.
var Hash = state.CreateNativeContractHash(nativenames.Notary)

// NewReader creates an instance of ContractReader to get data from the Notary
// contract.
func NewReader(invoker ContractInvoker) *ContractReader {
	return &ContractReader{invoker}
}

// New creates an instance of Contract to perform state-changing actions in the
// Notary contract.
func New(actor ContractActor) *Contract {
	return &Contract{*NewReader(actor), actor}
}

// BalanceOf returns the locked GAS balance for the given account.
func (c *ContractReader) BalanceOf(account util.Uint160) (*big.Int, error) {
	return unwrap.BigInt(c.invoker.Call(Hash, "balanceOf", account))
}

// ExpirationOf returns the index of the block when the GAS deposit for the given
// account will expire.
func (c *ContractReader) ExpirationOf(account util.Uint160) (uint32, error) {
	res, err := c.invoker.Call(Hash, "expirationOf", account)
	ret, err := unwrap.LimitedInt64(res, err, 0, math.MaxUint32)
	return uint32(ret), err
}

// GetMaxNotValidBeforeDelta returns the maximum NotValidBefore attribute delta
// that can be used in notary-assisted transactions.
func (c *ContractReader) GetMaxNotValidBeforeDelta() (uint32, error) {
	res, err := c.invoker.Call(Hash, "getMaxNotValidBeforeDelta")
	ret, err := unwrap.LimitedInt64(res, err, 0, math.MaxUint32)
	return uint32(ret), err
}

// GetNotaryServiceFeePerKey returns the per-key fee amount paid by transactions
// for the NotaryAssisted attribute.
func (c *ContractReader) GetNotaryServiceFeePerKey() (int64, error) {
	return unwrap.Int64(c.invoker.Call(Hash, "getNotaryServiceFeePerKey"))
}

// LockDepositUntil creates and sends a transaction that extends the deposit lock
// time for the given account. The return result from the "lockDepositUntil"
// method is checked to be true, so transaction fails (with FAULT state) if not
// successful. The returned values are transaction hash, its ValidUntilBlock
// value and an error if any.
func (c *Contract) LockDepositUntil(account util.Uint160, index uint32) (util.Uint256, uint32, error) {
	return c.actor.SendRun(lockScript(account, index))
}

// LockDepositUntilTransaction creates a transaction that extends the deposit lock
// time for the given account. The return result from the "lockDepositUntil"
// method is checked to be true, so transaction fails (with FAULT state) if not
// successful. The returned values are transaction hash, its ValidUntilBlock
// value and an error if any. The transaction is signed, but not sent to the
// network, instead it's returned to the caller.
func (c *Contract) LockDepositUntilTransaction(account util.Uint160, index uint32) (*transaction.Transaction, error) {
	return c.actor.MakeRun(lockScript(account, index))
}

// LockDepositUntilUnsigned creates a transaction that extends the deposit lock
// time for the given account. The return result from the "lockDepositUntil"
// method is checked to be true, so transaction fails (with FAULT state) if not
// successful. The returned values are transaction hash, its ValidUntilBlock
// value and an error if any. The transaction is not signed and just returned to
// the caller.
func (c *Contract) LockDepositUntilUnsigned(account util.Uint160, index uint32) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedRun(lockScript(account, index), nil)
}

func lockScript(account util.Uint160, index uint32) []byte {
	// We know parameters exactly (unlike with nep17.Transfer), so this can't fail.
	script, _ := smartcontract.CreateCallWithAssertScript(Hash, "lockDepositUntil", account.BytesBE(), int64(index))
	return script
}

// SetMaxNotValidBeforeDelta creates and sends a transaction that sets the new
// maximum NotValidBefore attribute value delta that can be used in
// notary-assisted transactions. The action is successful when transaction
// ends in HALT state. Notice that this setting can be changed only by the
// network's committee, so use an appropriate Actor. The returned values are
// transaction hash, its ValidUntilBlock value and an error if any.
func (c *Contract) SetMaxNotValidBeforeDelta(blocks uint32) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, setMaxNVBDeltaMethod, blocks)
}

// SetMaxNotValidBeforeDeltaTransaction creates a transaction that sets the new
// maximum NotValidBefore attribute value delta that can be used in
// notary-assisted transactions. The action is successful when transaction
// ends in HALT state. Notice that this setting can be changed only by the
// network's committee, so use an appropriate Actor. The transaction is signed,
// but not sent to the network, instead it's returned to the caller.
func (c *Contract) SetMaxNotValidBeforeDeltaTransaction(blocks uint32) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, setMaxNVBDeltaMethod, blocks)
}

// SetMaxNotValidBeforeDeltaUnsigned creates a transaction that sets the new
// maximum NotValidBefore attribute value delta that can be used in
// notary-assisted transactions. The action is successful when transaction
// ends in HALT state. Notice that this setting can be changed only by the
// network's committee, so use an appropriate Actor. The transaction is not
// signed and just returned to the caller.
func (c *Contract) SetMaxNotValidBeforeDeltaUnsigned(blocks uint32) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, setMaxNVBDeltaMethod, nil, blocks)
}

// SetNotaryServiceFeePerKey creates and sends a transaction that sets the new
// per-key fee value paid for using the notary service. The action is successful
// when transaction ends in HALT state. Notice that this setting can be changed
// only by the network's committee, so use an appropriate Actor. The returned
// values are transaction hash, its ValidUntilBlock value and an error if any.
func (c *Contract) SetNotaryServiceFeePerKey(fee int64) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, setFeePKMethod, fee)
}

// SetNotaryServiceFeePerKeyTransaction creates a transaction that sets the new
// per-key fee value paid for using the notary service. The action is successful
// when transaction ends in HALT state. Notice that this setting can be changed
// only by the network's committee, so use an appropriate Actor. The transaction
// is signed, but not sent to the network, instead it's returned to the caller.
func (c *Contract) SetNotaryServiceFeePerKeyTransaction(fee int64) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, setFeePKMethod, fee)
}

// SetNotaryServiceFeePerKeyUnsigned creates a transaction that sets the new
// per-key fee value paid for using the notary service. The action is successful
// when transaction ends in HALT state. Notice that this setting can be changed
// only by the network's committee, so use an appropriate Actor. The transaction
// is not signed and just returned to the caller.
func (c *Contract) SetNotaryServiceFeePerKeyUnsigned(fee int64) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, setFeePKMethod, nil, fee)
}

// Withdraw creates and sends a transaction that withdraws the deposit belonging
// to "from" account and sends it to "to" account. The return result from the
// "withdraw" method is checked to be true, so transaction fails (with FAULT
// state) if not successful. The returned values are transaction hash, its
// ValidUntilBlock value and an error if any.
func (c *Contract) Withdraw(from util.Uint160, to util.Uint160) (util.Uint256, uint32, error) {
	return c.actor.SendRun(withdrawScript(from, to))
}

// WithdrawTransaction creates a transaction that withdraws the deposit belonging
// to "from" account and sends it to "to" account. The return result from the
// "withdraw" method is checked to be true, so transaction fails (with FAULT
// state) if not successful. The transaction is signed, but not sent to the
// network, instead it's returned to the caller.
func (c *Contract) WithdrawTransaction(from util.Uint160, to util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeRun(withdrawScript(from, to))
}

// WithdrawUnsigned creates a transaction that withdraws the deposit belonging
// to "from" account and sends it to "to" account. The return result from the
// "withdraw" method is checked to be true, so transaction fails (with FAULT
// state) if not successful. The transaction is not signed and just returned to
// the caller.
func (c *Contract) WithdrawUnsigned(from util.Uint160, to util.Uint160) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedRun(withdrawScript(from, to), nil)
}

func withdrawScript(from util.Uint160, to util.Uint160) []byte {
	// We know parameters exactly (unlike with nep17.Transfer), so this can't fail.
	script, _ := smartcontract.CreateCallWithAssertScript(Hash, "withdraw", from.BytesBE(), to.BytesBE())
	return script
}
