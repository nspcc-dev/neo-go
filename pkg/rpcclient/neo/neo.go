/*
Package neo provides an RPC-based wrapper for the NEOToken contract.

Safe methods are encapsulated into ContractReader structure while Contract provides
various methods to perform state-changing calls.
*/
package neo

import (
	"crypto/elliptic"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	setGasMethod = "setGasPerBlock"
	setRegMethod = "setRegisterPrice"
)

// Invoker is used by ContractReader to perform read-only calls.
type Invoker interface {
	nep17.Invoker

	CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...interface{}) (*result.Invoke, error)
	TerminateSession(sessionID uuid.UUID) error
	TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error)
}

// Actor is used by Contract to create and send transactions.
type Actor interface {
	nep17.Actor
	Invoker

	Run(script []byte) (*result.Invoke, error)
	MakeCall(contract util.Uint160, method string, params ...interface{}) (*transaction.Transaction, error)
	MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...interface{}) (*transaction.Transaction, error)
	MakeUnsignedUncheckedRun(script []byte, sysFee int64, attrs []transaction.Attribute) (*transaction.Transaction, error)
	SendCall(contract util.Uint160, method string, params ...interface{}) (util.Uint256, uint32, error)
	Sign(tx *transaction.Transaction) error
	SignAndSend(tx *transaction.Transaction) (util.Uint256, uint32, error)
}

// ContractReader represents safe (read-only) methods of NEO. It can be
// used to query various data.
type ContractReader struct {
	nep17.TokenReader

	invoker Invoker
}

// Contract provides full NEO interface, both safe and state-changing methods.
type Contract struct {
	ContractReader
	nep17.Token

	actor Actor
}

// CandidateStateEvent represents a CandidateStateChanged NEO event.
type CandidateStateEvent struct {
	Key        *keys.PublicKey
	Registered bool
	Votes      *big.Int
}

// VoteEvent represents a Vote NEO event.
type VoteEvent struct {
	Account util.Uint160
	From    *keys.PublicKey
	To      *keys.PublicKey
	Amount  *big.Int
}

// ValidatorIterator is used for iterating over GetAllCandidates results.
type ValidatorIterator struct {
	client   Invoker
	session  uuid.UUID
	iterator result.Iterator
}

// Hash stores the hash of the native NEOToken contract.
var Hash = state.CreateNativeContractHash(nativenames.Neo)

// NewReader creates an instance of ContractReader to get data from the NEO
// contract.
func NewReader(invoker Invoker) *ContractReader {
	return &ContractReader{*nep17.NewReader(invoker, Hash), invoker}
}

// New creates an instance of Contract to perform state-changing actions in the
// NEO contract.
func New(actor Actor) *Contract {
	return &Contract{*NewReader(actor), *nep17.New(actor, Hash), actor}
}

// GetAccountState returns current NEO balance state for the account which
// includes balance and voting data. It can return nil balance with no error
// if the account given has no NEO.
func (c *ContractReader) GetAccountState(account util.Uint160) (*state.NEOBalance, error) {
	itm, err := unwrap.Item(c.invoker.Call(Hash, "getAccountState", account))
	if err != nil {
		return nil, err
	}
	if _, ok := itm.(stackitem.Null); ok {
		return nil, nil
	}
	res := new(state.NEOBalance)
	err = res.FromStackItem(itm)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GetAllCandidates returns an iterator that allows to retrieve all registered
// validators from it. It depends on the server to provide proper session-based
// iterator, but can also work with expanded one.
func (c *ContractReader) GetAllCandidates() (*ValidatorIterator, error) {
	sess, iter, err := unwrap.SessionIterator(c.invoker.Call(Hash, "getAllCandidates"))
	if err != nil {
		return nil, err
	}

	return &ValidatorIterator{
		client:   c.invoker,
		iterator: iter,
		session:  sess,
	}, nil
}

// GetAllCandidatesExpanded is similar to GetAllCandidates (uses the same NEO
// method), but can be useful if the server used doesn't support sessions and
// doesn't expand iterators. It creates a script that will get num of result
// items from the iterator right in the VM and return them to you. It's only
// limited by VM stack and GAS available for RPC invocations.
func (c *ContractReader) GetAllCandidatesExpanded(num int) ([]result.Validator, error) {
	arr, err := unwrap.Array(c.invoker.CallAndExpandIterator(Hash, "getAllCandidates", num))
	if err != nil {
		return nil, err
	}
	return itemsToValidators(arr)
}

// Next returns the next set of elements from the iterator (up to num of them).
// It can return less than num elements in case iterator doesn't have that many
// or zero elements if the iterator has no more elements or the session is
// expired.
func (v *ValidatorIterator) Next(num int) ([]result.Validator, error) {
	items, err := v.client.TraverseIterator(v.session, &v.iterator, num)
	if err != nil {
		return nil, err
	}
	return itemsToValidators(items)
}

// Terminate closes the iterator session used by ValidatorIterator (if it's
// session-based).
func (v *ValidatorIterator) Terminate() error {
	if v.iterator.ID == nil {
		return nil
	}
	return v.client.TerminateSession(v.session)
}

// GetCandidates returns the list of validators with their vote count. This
// method is mostly useful for historic invocations because the RPC protocol
// provides direct getcandidates call that returns more data and works faster.
// The contract only returns up to 256 candidates in response to this method, so
// if there are more of them on the network you will get a truncated result, use
// GetAllCandidates to solve this problem.
func (c *ContractReader) GetCandidates() ([]result.Validator, error) {
	arr, err := unwrap.Array(c.invoker.Call(Hash, "getCandidates"))
	if err != nil {
		return nil, err
	}
	return itemsToValidators(arr)
}

func itemsToValidators(arr []stackitem.Item) ([]result.Validator, error) {
	res := make([]result.Validator, len(arr))
	for i, itm := range arr {
		str, ok := itm.Value().([]stackitem.Item)
		if !ok {
			return nil, fmt.Errorf("item #%d is not a structure", i)
		}
		if len(str) != 2 {
			return nil, fmt.Errorf("item #%d has wrong length", i)
		}
		b, err := str[0].TryBytes()
		if err != nil {
			return nil, fmt.Errorf("item #%d has wrong key: %w", i, err)
		}
		k, err := keys.NewPublicKeyFromBytes(b, elliptic.P256())
		if err != nil {
			return nil, fmt.Errorf("item #%d has wrong key: %w", i, err)
		}
		votes, err := str[1].TryInteger()
		if err != nil {
			return nil, fmt.Errorf("item #%d has wrong votes: %w", i, err)
		}
		if !votes.IsInt64() {
			return nil, fmt.Errorf("item #%d has too big number of votes", i)
		}
		res[i].PublicKey = *k
		res[i].Votes = votes.Int64()
	}
	return res, nil
}

// GetCommittee returns the list of committee member public keys. This
// method is mostly useful for historic invocations because the RPC protocol
// provides direct getcommittee call that works faster.
func (c *ContractReader) GetCommittee() (keys.PublicKeys, error) {
	return unwrap.ArrayOfPublicKeys(c.invoker.Call(Hash, "getCommittee"))
}

// GetNextBlockValidators returns the list of validator keys that will sign the
// next block. This method is mostly useful for historic invocations because the
// RPC protocol provides direct getnextblockvalidators call that provides more
// data and works faster.
func (c *ContractReader) GetNextBlockValidators() (keys.PublicKeys, error) {
	return unwrap.ArrayOfPublicKeys(c.invoker.Call(Hash, "getNextBlockValidators"))
}

// GetGasPerBlock returns the amount of GAS generated in each block.
func (c *ContractReader) GetGasPerBlock() (int64, error) {
	return unwrap.Int64(c.invoker.Call(Hash, "getGasPerBlock"))
}

// GetRegisterPrice returns the price of candidate key registration.
func (c *ContractReader) GetRegisterPrice() (int64, error) {
	return unwrap.Int64(c.invoker.Call(Hash, "getRegisterPrice"))
}

// UnclaimedGas allows to calculate the amount of GAS that will be generated if
// any NEO state change ("claim") is to happen for the given account at the given
// block number. This method is mostly useful for historic invocations because
// the RPC protocol provides direct getunclaimedgas method that works faster.
func (c *ContractReader) UnclaimedGas(account util.Uint160, end uint32) (*big.Int, error) {
	return unwrap.BigInt(c.invoker.Call(Hash, "unclaimedGas", account, end))
}

// RegisterCandidate creates and sends a transaction that adds the given key to
// the list of candidates that can be voted for. The return result from the
// "registerCandidate" method is checked to be true, so transaction fails (with
// FAULT state) if not successful. Notice that for this call to work it must be
// witnessed by the simple account derived from the given key, so use an
// appropriate Actor. The returned values are transaction hash, its
// ValidUntilBlock value and an error if any.
//
// Notice that unlike for all other methods the script for this one is not
// test-executed in its final form because most networks have registration price
// set to be much higher than typical RPC server allows to spend during
// test-execution. This adds some risk that it might fail on-chain, but in
// practice it's not likely to happen if signers are set up correctly.
func (c *Contract) RegisterCandidate(k *keys.PublicKey) (util.Uint256, uint32, error) {
	tx, err := c.RegisterCandidateUnsigned(k)
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return c.actor.SignAndSend(tx)
}

// RegisterCandidateTransaction creates a transaction that adds the given key to
// the list of candidates that can be voted for. The return result from the
// "registerCandidate" method is checked to be true, so transaction fails (with
// FAULT state) if not successful. Notice that for this call to work it must be
// witnessed by the simple account derived from the given key, so use an
// appropriate Actor. The transaction is signed, but not sent to the network,
// instead it's returned to the caller.
//
// Notice that unlike for all other methods the script for this one is not
// test-executed in its final form because most networks have registration price
// set to be much higher than typical RPC server allows to spend during
// test-execution. This adds some risk that it might fail on-chain, but in
// practice it's not likely to happen if signers are set up correctly.
func (c *Contract) RegisterCandidateTransaction(k *keys.PublicKey) (*transaction.Transaction, error) {
	tx, err := c.RegisterCandidateUnsigned(k)
	if err != nil {
		return nil, err
	}
	err = c.actor.Sign(tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// RegisterCandidateUnsigned creates a transaction that adds the given key to
// the list of candidates that can be voted for. The return result from the
// "registerCandidate" method is checked to be true, so transaction fails (with
// FAULT state) if not successful. Notice that for this call to work it must be
// witnessed by the simple account derived from the given key, so use an
// appropriate Actor. The transaction is not signed and just returned to the
// caller.
//
// Notice that unlike for all other methods the script for this one is not
// test-executed in its final form because most networks have registration price
// set to be much higher than typical RPC server allows to spend during
// test-execution. This adds some risk that it might fail on-chain, but in
// practice it's not likely to happen if signers are set up correctly.
func (c *Contract) RegisterCandidateUnsigned(k *keys.PublicKey) (*transaction.Transaction, error) {
	// It's an unregister script intentionally.
	r, err := c.actor.Run(regScript(true, k))
	if err != nil {
		return nil, err
	}
	regPrice, err := c.GetRegisterPrice()
	if err != nil {
		return nil, err
	}
	return c.actor.MakeUnsignedUncheckedRun(regScript(false, k), r.GasConsumed+regPrice, nil)
}

// UnregisterCandidate creates and sends a transaction that removes the key from
// the list of candidates that can be voted for. The return result from the
// "unregisterCandidate" method is checked to be true, so transaction fails (with
// FAULT state) if not successful. Notice that for this call to work it must be
// witnessed by the simple account derived from the given key, so use an
// appropriate Actor. The returned values are transaction hash, its
// ValidUntilBlock value and an error if any.
func (c *Contract) UnregisterCandidate(k *keys.PublicKey) (util.Uint256, uint32, error) {
	return c.actor.SendRun(regScript(true, k))
}

// UnregisterCandidateTransaction creates a transaction that removes the key from
// the list of candidates that can be voted for. The return result from the
// "unregisterCandidate" method is checked to be true, so transaction fails (with
// FAULT state) if not successful. Notice that for this call to work it must be
// witnessed by the simple account derived from the given key, so use an
// appropriate Actor. The transaction is signed, but not sent to the network,
// instead it's returned to the caller.
func (c *Contract) UnregisterCandidateTransaction(k *keys.PublicKey) (*transaction.Transaction, error) {
	return c.actor.MakeRun(regScript(true, k))
}

// UnregisterCandidateUnsigned creates a transaction that removes the key from
// the list of candidates that can be voted for. The return result from the
// "unregisterCandidate" method is checked to be true, so transaction fails (with
// FAULT state) if not successful. Notice that for this call to work it must be
// witnessed by the simple account derived from the given key, so use an
// appropriate Actor. The transaction is not signed and just returned to the
// caller.
func (c *Contract) UnregisterCandidateUnsigned(k *keys.PublicKey) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedRun(regScript(true, k), nil)
}

func regScript(unreg bool, k *keys.PublicKey) []byte {
	var method = "registerCandidate"

	if unreg {
		method = "unregisterCandidate"
	}

	// We know parameters exactly (unlike with nep17.Transfer), so this can't fail.
	script, _ := smartcontract.CreateCallWithAssertScript(Hash, method, k.Bytes())
	return script
}

// Vote creates and sends a transaction that casts a vote from the given account
// to the given key which can be nil (in which case any previous vote is removed).
// The return result from the "vote" method is checked to be true, so transaction
// fails (with FAULT state) if voting is not successful. The returned values are
// transaction hash, its ValidUntilBlock value and an error if any.
func (c *Contract) Vote(account util.Uint160, voteTo *keys.PublicKey) (util.Uint256, uint32, error) {
	return c.actor.SendRun(voteScript(account, voteTo))
}

// VoteTransaction creates a transaction that casts a vote from the given account
// to the given key which can be nil (in which case any previous vote is removed).
// The return result from the "vote" method is checked to be true, so transaction
// fails (with FAULT state) if voting is not successful. The transaction is signed,
// but not sent to the network, instead it's returned to the caller.
func (c *Contract) VoteTransaction(account util.Uint160, voteTo *keys.PublicKey) (*transaction.Transaction, error) {
	return c.actor.MakeRun(voteScript(account, voteTo))
}

// VoteUnsigned creates a transaction that casts a vote from the given account
// to the given key which can be nil (in which case any previous vote is removed).
// The return result from the "vote" method is checked to be true, so transaction
// fails (with FAULT state) if voting is not successful. The transaction is not
// signed and just returned to the caller.
func (c *Contract) VoteUnsigned(account util.Uint160, voteTo *keys.PublicKey) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedRun(voteScript(account, voteTo), nil)
}

func voteScript(account util.Uint160, voteTo *keys.PublicKey) []byte {
	var param interface{}

	if voteTo != nil {
		param = voteTo.Bytes()
	}
	// We know parameters exactly (unlike with nep17.Transfer), so this can't fail.
	script, _ := smartcontract.CreateCallWithAssertScript(Hash, "vote", account, param)
	return script
}

// SetGasPerBlock creates and sends a transaction that sets the new amount of
// GAS to be generated in each block. The action is successful when transaction
// ends in HALT state. Notice that this setting can be changed only by the
// network's committee, so use an appropriate Actor. The returned values are
// transaction hash, its ValidUntilBlock value and an error if any.
func (c *Contract) SetGasPerBlock(gas int64) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, setGasMethod, gas)
}

// SetGasPerBlockTransaction creates a transaction that sets the new amount of
// GAS to be generated in each block. The action is successful when transaction
// ends in HALT state. Notice that this setting can be changed only by the
// network's committee, so use an appropriate Actor. The transaction is signed,
// but not sent to the network, instead it's returned to the caller.
func (c *Contract) SetGasPerBlockTransaction(gas int64) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, setGasMethod, gas)
}

// SetGasPerBlockUnsigned creates a transaction that sets the new amount of
// GAS to be generated in each block. The action is successful when transaction
// ends in HALT state. Notice that this setting can be changed only by the
// network's committee, so use an appropriate Actor. The transaction is not
// signed and just returned to the caller.
func (c *Contract) SetGasPerBlockUnsigned(gas int64) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, setGasMethod, nil, gas)
}

// SetRegisterPrice creates and sends a transaction that sets the new candidate
// registration price (in GAS). The action is successful when transaction
// ends in HALT state. Notice that this setting can be changed only by the
// network's committee, so use an appropriate Actor. The returned values are
// transaction hash, its ValidUntilBlock value and an error if any.
func (c *Contract) SetRegisterPrice(price int64) (util.Uint256, uint32, error) {
	return c.actor.SendCall(Hash, setRegMethod, price)
}

// SetRegisterPriceTransaction creates a transaction that sets the new candidate
// registration price (in GAS). The action is successful when transaction
// ends in HALT state. Notice that this setting can be changed only by the
// network's committee, so use an appropriate Actor. The transaction is signed,
// but not sent to the network, instead it's returned to the caller.
func (c *Contract) SetRegisterPriceTransaction(price int64) (*transaction.Transaction, error) {
	return c.actor.MakeCall(Hash, setRegMethod, price)
}

// SetRegisterPriceUnsigned creates a transaction that sets the new candidate
// registration price (in GAS). The action is successful when transaction
// ends in HALT state. Notice that this setting can be changed only by the
// network's committee, so use an appropriate Actor. The transaction is not
// signed and just returned to the caller.
func (c *Contract) SetRegisterPriceUnsigned(price int64) (*transaction.Transaction, error) {
	return c.actor.MakeUnsignedCall(Hash, setRegMethod, nil, price)
}
