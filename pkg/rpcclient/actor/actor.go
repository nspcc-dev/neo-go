/*
Package actor provides a way to change chain state via RPC client.

This layer builds on top of the basic RPC client and [invoker] package, it
simplifies creating, signing and sending transactions to the network (since
that's the only way chain state is changed). It's generic enough to be used for
any contract that you may want to invoke and contract-specific functions can
build on top of it.
*/
package actor

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// RPCActor is an interface required from the RPC client to successfully
// create and send transactions.
type RPCActor interface {
	invoker.RPCInvoke

	// CalculateNetworkFee calculates network fee for the given transaction.
	//
	// CalculateNetworkFee MUST NOT call state-changing methods (like Hash or Size)
	// of the transaction through the passed pointer: make a copy if necessary.
	CalculateNetworkFee(tx *transaction.Transaction) (int64, error)
	GetBlockCount() (uint32, error)
	GetVersion() (*result.Version, error)
	SendRawTransaction(tx *transaction.Transaction) (util.Uint256, error)
}

// SignerAccount represents combination of the transaction.Signer and the
// corresponding wallet.Account. It's used to create and sign transactions, each
// transaction has a set of signers that must witness the transaction with their
// signatures.
type SignerAccount struct {
	Signer  transaction.Signer
	Account *wallet.Account
}

// Actor keeps a connection to the RPC endpoint and allows to perform
// state-changing actions (via transactions that can also be created without
// sending them to the network) on behalf of a set of signers. It also provides
// an Invoker interface to perform test calls with the same set of signers.
//
// Actor-specific APIs follow the naming scheme set by Invoker in method
// suffixes. *Call methods operate with function calls and require a contract
// hash, a method and parameters if any. *Run methods operate with scripts and
// require a NeoVM script that will be used directly. Prefixes denote the
// action to be performed, "Make" prefix is used for methods that create
// transactions in various ways, while "Send" prefix is used by methods that
// directly transmit created transactions to the RPC server.
//
// Actor also provides a Waiter interface to wait until transaction will be
// accepted to the chain. Depending on the underlying RPCActor functionality,
// transaction awaiting can be performed via web-socket using RPC notifications
// subsystem with EventWaiter, via regular RPC requests using a poll-based
// algorithm with PollingWaiter or can not be performed if RPCActor doesn't
// implement none of RPCEventWaiter and RPCPollingWaiter interfaces with
// NullWaiter. ErrAwaitingNotSupported will be returned on attempt to await the
// transaction in the latter case. Waiter uses context of the underlying RPCActor
// and interrupts transaction awaiting process if the context is done.
// ErrContextDone wrapped with the context's error will be returned in this case.
// Otherwise, transaction awaiting process is ended with ValidUntilBlock acceptance
// and ErrTxNotAccepted is returned if transaction wasn't accepted by this moment.
type Actor struct {
	invoker.Invoker
	Waiter

	client    RPCActor
	opts      Options
	signers   []SignerAccount
	txSigners []transaction.Signer
	version   *result.Version
}

// Options are used to create Actor with non-standard transaction checkers or
// additional attributes to be applied for all transactions.
type Options struct {
	// Attributes are set as is into every transaction created by Actor,
	// unless they're explicitly set in a method call that accepts
	// attributes (like MakeTuned* or MakeUnsigned*).
	Attributes []transaction.Attribute
	// CheckerModifier is used by any method that creates and signs a
	// transaction inside (some of them provide ways to override this
	// default, some don't).
	CheckerModifier TransactionCheckerModifier
	// Modifier is used only by MakeUncheckedRun to modify transaction
	// before it's signed (other methods that perform test invocations
	// use CheckerModifier). MakeUnsigned* methods do not run it.
	Modifier TransactionModifier
}

// New creates an Actor instance using the specified RPC interface and the set of
// signers with corresponding accounts. Every transaction created by this Actor
// will have this set of signers and all communication will be performed via this
// RPC. Upon Actor instance creation a GetVersion call is made and the result of
// it is cached forever (and used for internal purposes). The actor will use
// default Options (which can be overridden using NewTuned).
func New(ra RPCActor, signers []SignerAccount) (*Actor, error) {
	if len(signers) < 1 {
		return nil, errors.New("at least one signer (sender) is required")
	}
	invSigners := make([]transaction.Signer, len(signers))
	for i := range signers {
		if signers[i].Account.Contract == nil {
			return nil, fmt.Errorf("empty contract for account %s", signers[i].Account.Address)
		}
		if !signers[i].Account.Contract.Deployed && signers[i].Account.Contract.ScriptHash() != signers[i].Signer.Account {
			return nil, fmt.Errorf("signer account doesn't match script hash for signer %s", signers[i].Account.Address)
		}

		invSigners[i] = signers[i].Signer
	}
	inv := invoker.New(ra, invSigners)
	version, err := ra.GetVersion()
	if err != nil {
		return nil, err
	}
	return &Actor{
		Invoker:   *inv,
		Waiter:    NewWaiter(ra, version),
		client:    ra,
		opts:      NewDefaultOptions(),
		signers:   signers,
		txSigners: invSigners,
		version:   version,
	}, nil
}

// NewSimple makes it easier to create an Actor for the most widespread case
// when transactions have only one signer that uses CalledByEntry scope. When
// other scopes or multiple signers are needed use New.
func NewSimple(ra RPCActor, acc *wallet.Account) (*Actor, error) {
	return New(ra, []SignerAccount{{
		Signer: transaction.Signer{
			Account: acc.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: acc,
	}})
}

// NewDefaultOptions returns Options that have no attributes and use the default
// TransactionCheckerModifier function (that checks for the invocation result to
// be in HALT state) and TransactionModifier (that does nothing).
func NewDefaultOptions() Options {
	return Options{
		CheckerModifier: DefaultCheckerModifier,
		Modifier:        DefaultModifier,
	}
}

// NewTuned creates an Actor that will use the specified Options as defaults when
// creating new transactions. If checker/modifier callbacks are not provided
// (nil), then default ones (from NewDefaultOptions) are used.
func NewTuned(ra RPCActor, signers []SignerAccount, opts Options) (*Actor, error) {
	a, err := New(ra, signers)
	if err != nil {
		return nil, err
	}
	a.opts.Attributes = opts.Attributes
	if opts.CheckerModifier != nil {
		a.opts.CheckerModifier = opts.CheckerModifier
	}
	if opts.Modifier != nil {
		a.opts.Modifier = opts.Modifier
	}
	return a, err
}

// CalculateNetworkFee wraps RPCActor's CalculateNetworkFee, making it available
// to Actor users directly. It returns network fee value for the given
// transaction.
func (a *Actor) CalculateNetworkFee(tx *transaction.Transaction) (int64, error) {
	return a.client.CalculateNetworkFee(tx)
}

// GetBlockCount wraps RPCActor's GetBlockCount, making it available to
// Actor users directly. It returns current number of blocks in the chain.
func (a *Actor) GetBlockCount() (uint32, error) {
	return a.client.GetBlockCount()
}

// GetNetwork is a convenience method that returns the network's magic number.
func (a *Actor) GetNetwork() netmode.Magic {
	return a.version.Protocol.Network
}

// GetVersion returns version data from the RPC endpoint.
func (a *Actor) GetVersion() result.Version {
	return *a.version
}

// Send allows to send arbitrary prepared transaction to the network. It returns
// transaction hash and ValidUntilBlock value.
func (a *Actor) Send(tx *transaction.Transaction) (util.Uint256, uint32, error) {
	h, err := a.client.SendRawTransaction(tx)
	return h, tx.ValidUntilBlock, err
}

// Sign adds signatures to arbitrary transaction using Actor signers wallets.
// Most of the time it shouldn't be used directly since it'll be successful only
// if the transaction is made using the same set of accounts as the one used
// for Actor creation.
func (a *Actor) Sign(tx *transaction.Transaction) error {
	if len(tx.Signers) != len(a.signers) {
		return errors.New("incorrect number of signers in the transaction")
	}
	for i, signer := range a.signers {
		err := signer.Account.SignTx(a.GetNetwork(), tx)
		if err != nil { // then account is non-contract-based and locked, but let's provide more detailed error
			if paramNum := len(signer.Account.Contract.Parameters); paramNum != 0 && signer.Account.Contract.Deployed {
				return fmt.Errorf("failed to add contract-based witness for signer #%d (%s): "+
					"%d parameters must be provided to construct invocation script", i, signer.Account.Address, paramNum)
			}
			return fmt.Errorf("failed to add witness for signer #%d (%s): account should be unlocked to add the signature. "+
				"Store partially-signed transaction and then use 'wallet sign' command to cosign it", i, signer.Account.Address)
		}
	}
	return nil
}

// SignAndSend signs arbitrary transaction (see also Sign) and sends it to the
// network.
func (a *Actor) SignAndSend(tx *transaction.Transaction) (util.Uint256, uint32, error) {
	return a.sendWrapper(tx, a.Sign(tx))
}

// sendWrapper simplifies wrapping methods that create transactions.
func (a *Actor) sendWrapper(tx *transaction.Transaction, err error) (util.Uint256, uint32, error) {
	if err != nil {
		return util.Uint256{}, 0, err
	}
	return a.Send(tx)
}

// SendCall creates a transaction that calls the given method of the given
// contract with the given parameters (see also MakeCall) and sends it to the
// network.
func (a *Actor) SendCall(contract util.Uint160, method string, params ...any) (util.Uint256, uint32, error) {
	return a.sendWrapper(a.MakeCall(contract, method, params...))
}

// SendTunedCall creates a transaction that calls the given method of the given
// contract with the given parameters (see also MakeTunedCall) and attributes,
// allowing to check for execution results of this call and modify transaction
// before it's signed; this transaction is then sent to the network.
func (a *Actor) SendTunedCall(contract util.Uint160, method string, attrs []transaction.Attribute, txHook TransactionCheckerModifier, params ...any) (util.Uint256, uint32, error) {
	return a.sendWrapper(a.MakeTunedCall(contract, method, attrs, txHook, params...))
}

// SendRun creates a transaction with the given executable script (see also
// MakeRun) and sends it to the network.
func (a *Actor) SendRun(script []byte) (util.Uint256, uint32, error) {
	return a.sendWrapper(a.MakeRun(script))
}

// SendTunedRun creates a transaction with the given executable script and
// attributes, allowing to check for execution results of this script and modify
// transaction before it's signed (see also MakeTunedRun). This transaction is
// then sent to the network.
func (a *Actor) SendTunedRun(script []byte, attrs []transaction.Attribute, txHook TransactionCheckerModifier) (util.Uint256, uint32, error) {
	return a.sendWrapper(a.MakeTunedRun(script, attrs, txHook))
}

// SendUncheckedRun creates a transaction with the given executable script and
// attributes that can use up to sysfee GAS for its execution, allowing to modify
// this transaction before it's signed (see also MakeUncheckedRun). This
// transaction is then sent to the network.
func (a *Actor) SendUncheckedRun(script []byte, sysfee int64, attrs []transaction.Attribute, txHook TransactionModifier) (util.Uint256, uint32, error) {
	return a.sendWrapper(a.MakeUncheckedRun(script, sysfee, attrs, txHook))
}

// Sender return the sender address that will be used in transactions created
// by Actor.
func (a *Actor) Sender() util.Uint160 {
	return a.txSigners[0].Account
}
