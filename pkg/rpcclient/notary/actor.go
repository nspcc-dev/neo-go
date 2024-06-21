package notary

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

var (
	// ErrFallbackAccepted is returned from [Actor.WaitSuccess] when
	// fallback transaction enters the chain instead of the main one.
	ErrFallbackAccepted = errors.New("fallback transaction accepted")
)

// Actor encapsulates everything needed to create proper notary requests for
// assisted transactions.
type Actor struct {
	// Actor is the main transaction actor, it has appropriate attributes and
	// transaction modifiers to set ValidUntilBlock. Use it to create main
	// transactions that have incomplete set of signatures. They can be
	// signed (using available wallets), but can not be sent directly to the
	// network. Instead of sending them to the network use Actor methods to
	// wrap them into notary requests.
	actor.Actor
	// FbActor is the fallback transaction actor, it has two required signers
	// and a set of attributes expected from a fallback transaction. It can
	// be used to create _unsigned_ transactions with whatever actions
	// required (but no additional attributes can be added). Signing them
	// while technically possible (with notary contract signature missing),
	// will lead to incorrect transaction because NotValidBefore and
	// Conflicts attributes as well as ValidUntilBlock field can be
	// correctly set only when some main transaction is available.
	FbActor actor.Actor

	fbScript []byte
	reader   *ContractReader
	sender   *wallet.Account
	rpc      RPCActor
}

// ActorOptions are used to influence main and fallback actors as well as the
// default Notarize behavior.
type ActorOptions struct {
	// FbAttributes are additional attributes to be added into fallback
	// transaction by an appropriate actor. Irrespective of this setting
	// (which defaults to nil) NotaryAssisted, NotValidBefore and Conflicts
	// attributes are always added.
	FbAttributes []transaction.Attribute
	// FbScript is the script to use in the Notarize convenience method, it
	// defaults to a simple RET instruction (doing nothing).
	FbScript []byte
	// FbSigner is the second signer to be used for the fallback transaction.
	// By default it's derived from the account and has None scope, it has
	// to be a simple signature or deployed contract account, but this setting
	// allows you to give it some other scope to be used in complex fallback
	// scripts.
	FbSigner actor.SignerAccount
	// MainAttribtues are additional attributes to be added into main
	// transaction by an appropriate actor. Irrespective of this setting
	// (which defaults to nil) NotaryAssisted attribute is always added.
	MainAttributes []transaction.Attribute
	// MainCheckerModifier will be used by the main Actor when creating
	// transactions. It defaults to using [actor.DefaultCheckerModifier]
	// for result check and adds MaxNotValidBeforeDelta to the
	// ValidUntilBlock transaction's field. Only override it if you know
	// what you're doing.
	MainCheckerModifier actor.TransactionCheckerModifier
	// MainModifier will be used by the main Actor when creating
	// transactions. By default it adds MaxNotValidBeforeDelta to the
	// ValidUntilBlock transaction's field. Only override it if you know
	// what you're doing.
	MainModifier actor.TransactionModifier
}

// RPCActor is a set of methods required from RPC client to create Actor.
type RPCActor interface {
	actor.RPCActor

	SubmitP2PNotaryRequest(req *payload.P2PNotaryRequest) (util.Uint256, error)
}

// NewDefaultActorOptions returns the default Actor options. Internal functions
// of it need some data from the contract, so it should be added.
func NewDefaultActorOptions(reader *ContractReader, acc *wallet.Account) *ActorOptions {
	opts := &ActorOptions{
		FbScript: []byte{byte(opcode.RET)},
		FbSigner: actor.SignerAccount{
			Signer: transaction.Signer{
				Account: acc.Contract.ScriptHash(),
				Scopes:  transaction.None,
			},
			Account: acc,
		},
		MainModifier: func(t *transaction.Transaction) error {
			nvbDelta, err := reader.GetMaxNotValidBeforeDelta()
			if err != nil {
				return fmt.Errorf("can't get MaxNVBDelta: %w", err)
			}
			t.ValidUntilBlock += nvbDelta
			return nil
		},
	}
	opts.MainCheckerModifier = func(r *result.Invoke, t *transaction.Transaction) error {
		err := actor.DefaultCheckerModifier(r, t)
		if err != nil {
			return err
		}
		return opts.MainModifier(t)
	}
	return opts
}

// NewActor creates a new notary.Actor using the given RPC client, the set of
// signers for main transactions and the account that will sign notary requests
// (one plain signature or contract-based). The set of signers will be extended
// by the notary contract signer with the None scope (as required by the notary
// protocol) and all transactions created with the resulting Actor will get a
// NotaryAssisted attribute with appropriate number of keys specified
// (depending on signers). A fallback Actor will be created as well with the
// notary contract and simpleAcc signers and a full set of required fallback
// transaction attributes (NotaryAssisted, NotValidBefore and Conflicts).
func NewActor(c RPCActor, signers []actor.SignerAccount, simpleAcc *wallet.Account) (*Actor, error) {
	return newTunedActor(c, signers, simpleAcc, nil)
}

// NewTunedActor is the same as NewActor, but allows to override the default
// options (see ActorOptions for details). Use with care.
func NewTunedActor(c RPCActor, signers []actor.SignerAccount, opts *ActorOptions) (*Actor, error) {
	return newTunedActor(c, signers, opts.FbSigner.Account, opts)
}

func newTunedActor(c RPCActor, signers []actor.SignerAccount, simpleAcc *wallet.Account, opts *ActorOptions) (*Actor, error) {
	if len(signers) < 1 {
		return nil, errors.New("at least one signer (sender) is required")
	}
	var nKeys int
	for _, sa := range signers {
		if sa.Account.Contract == nil {
			return nil, fmt.Errorf("empty contract for account %s", sa.Account.Address)
		}
		if sa.Account.Contract.Deployed {
			continue
		}
		if vm.IsSignatureContract(sa.Account.Contract.Script) {
			nKeys++
			continue
		}
		_, pubs, ok := vm.ParseMultiSigContract(sa.Account.Contract.Script)
		if !ok {
			return nil, fmt.Errorf("signer %s is not a contract- or signature-based", sa.Account.Address)
		}
		nKeys += len(pubs)
	}
	if nKeys > 255 {
		return nil, fmt.Errorf("notary subsystem can't handle more than 255 signatures")
	}
	if simpleAcc.Contract == nil {
		return nil, errors.New("bad simple account: no contract")
	}
	if !simpleAcc.CanSign() {
		return nil, errors.New("bad simple account: can't sign")
	}
	if !vm.IsSignatureContract(simpleAcc.Contract.Script) && !simpleAcc.Contract.Deployed {
		return nil, errors.New("bad simple account: neither plain signature, nor contract")
	}
	// Not reusing mainActor/fbActor for ContractReader to make requests a bit lighter.
	reader := NewReader(invoker.New(c, nil))
	if opts == nil {
		defOpts := NewDefaultActorOptions(reader, simpleAcc)
		opts = defOpts
	}
	var notarySA = actor.SignerAccount{
		Signer: transaction.Signer{
			Account: Hash,
			Scopes:  transaction.None,
		},
		Account: FakeContractAccount(Hash),
	}

	var mainSigners = make([]actor.SignerAccount, len(signers), len(signers)+1)
	copy(mainSigners, signers)
	mainSigners = append(mainSigners, notarySA)

	mainOpts := actor.Options{
		Attributes: []transaction.Attribute{{
			Type:  transaction.NotaryAssistedT,
			Value: &transaction.NotaryAssisted{NKeys: uint8(nKeys)},
		}},
		CheckerModifier: opts.MainCheckerModifier,
		Modifier:        opts.MainModifier,
	}
	mainOpts.Attributes = append(mainOpts.Attributes, opts.MainAttributes...)

	mainActor, err := actor.NewTuned(c, mainSigners, mainOpts)
	if err != nil {
		return nil, err
	}

	fbSigners := []actor.SignerAccount{notarySA, opts.FbSigner}
	fbOpts := actor.Options{
		Attributes: []transaction.Attribute{{
			Type:  transaction.NotaryAssistedT,
			Value: &transaction.NotaryAssisted{NKeys: 0},
		}, {
			// A stub, it has correct size, but the contents is to be filled per-request.
			Type:  transaction.NotValidBeforeT,
			Value: &transaction.NotValidBefore{},
		}, {
			// A stub, it has correct size, but the contents is to be filled per-request.
			Type:  transaction.ConflictsT,
			Value: &transaction.Conflicts{},
		}},
	}
	fbOpts.Attributes = append(fbOpts.Attributes, opts.FbAttributes...)
	fbActor, err := actor.NewTuned(c, fbSigners, fbOpts)
	if err != nil {
		return nil, err
	}
	return &Actor{*mainActor, *fbActor, opts.FbScript, reader, simpleAcc, c}, nil
}

// Notarize is a simple wrapper for transaction-creating functions that allows to
// send any partially-signed transaction in a notary request with a fallback
// transaction created based on Actor settings and SendRequest adjustment rules.
// The values returned are main and fallback transaction hashes, ValidUntilBlock
// and error if any.
func (a *Actor) Notarize(mainTx *transaction.Transaction, err error) (util.Uint256, util.Uint256, uint32, error) {
	var (
		// Just to simplify return values on error.
		fbHash   util.Uint256
		mainHash util.Uint256
		vub      uint32
	)
	if err != nil {
		return mainHash, fbHash, vub, err
	}
	fbTx, err := a.FbActor.MakeUnsignedRun(a.fbScript, nil)
	if err != nil {
		return mainHash, fbHash, vub, err
	}
	return a.SendRequest(mainTx, fbTx)
}

// SendRequest creates and sends a notary request using the given main and
// fallback transactions. It accepts signed main transaction and unsigned fallback
// transaction that will be adjusted in its NotValidBefore and Conflicts
// attributes as well as ValidUntilBlock value. Conflicts is set to the main
// transaction hash, while NotValidBefore is set to the middle of current mainTx
// lifetime (between current block and ValidUntilBlock). The values returned are
// main and fallback transaction hashes, ValidUntilBlock and error if any.
func (a *Actor) SendRequest(mainTx *transaction.Transaction, fbTx *transaction.Transaction) (util.Uint256, util.Uint256, uint32, error) {
	var (
		fbHash   util.Uint256
		mainHash = mainTx.Hash()
		vub      = mainTx.ValidUntilBlock
	)
	if len(fbTx.Attributes) < 3 {
		return mainHash, fbHash, vub, errors.New("invalid fallback: missing required attributes")
	}
	if fbTx.Attributes[1].Type != transaction.NotValidBeforeT {
		return mainHash, fbHash, vub, errors.New("invalid fallback: NotValidBefore is missing where expected")
	}
	if fbTx.Attributes[2].Type != transaction.ConflictsT {
		return mainHash, fbHash, vub, errors.New("invalid fallback: Conflicts is missing where expected")
	}
	height, err := a.GetBlockCount()
	if err != nil {
		return mainHash, fbHash, vub, err
	}
	// New values must be created to avoid overwriting originals via a pointer.
	fbTx.Attributes[1].Value = &transaction.NotValidBefore{Height: (height + vub) / 2}
	fbTx.Attributes[2].Value = &transaction.Conflicts{Hash: mainHash}
	fbTx.ValidUntilBlock = vub
	err = a.FbActor.Sign(fbTx)
	if err != nil {
		return mainHash, fbHash, vub, err
	}
	fbTx.Scripts[0].InvocationScript = append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, make([]byte, keys.SignatureLen)...) // Must be present.
	return a.SendRequestExactly(mainTx, fbTx)
}

// SendRequestExactly accepts signed and completely prepared main and fallback
// transactions, creates a P2P notary request containing them, signs and sends
// it to the network. Caller takes full responsibility for transaction
// correctness in this case, use this method only if you know exactly that you
// need to override some of the other method's behavior and you can do it. The
// values returned are main and fallback transaction hashes, ValidUntilBlock
// and error if any.
func (a *Actor) SendRequestExactly(mainTx *transaction.Transaction, fbTx *transaction.Transaction) (util.Uint256, util.Uint256, uint32, error) {
	var (
		fbHash   = fbTx.Hash()
		mainHash = mainTx.Hash()
		vub      = mainTx.ValidUntilBlock
	)
	req := &payload.P2PNotaryRequest{
		MainTransaction:     mainTx,
		FallbackTransaction: fbTx,
	}
	req.Witness = transaction.Witness{
		InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, a.sender.SignHashable(a.GetNetwork(), req)...),
		VerificationScript: a.sender.GetVerificationScript(),
	}
	actualHash, err := a.rpc.SubmitP2PNotaryRequest(req)
	if err != nil {
		return mainHash, fbHash, vub, fmt.Errorf("failed to submit notary request: %w", err)
	}
	if !actualHash.Equals(fbHash) {
		return mainHash, fbHash, vub, fmt.Errorf("sent and actual fallback tx hashes mismatch: %v vs %v", fbHash.StringLE(), actualHash.StringLE())
	}
	return mainHash, fbHash, vub, nil
}

// Wait waits until main or fallback transaction will be accepted to the chain and returns
// the resulting application execution result or actor.ErrTxNotAccepted if both transactions
// failed to persist. Wait can be used if underlying Actor supports transaction awaiting,
// see actor.Actor and actor.Waiter documentation for details. Wait may be used as a wrapper
// for Notarize, SendRequest or SendRequestExactly. Notice that "already exists" or "already
// on chain" answers are not treated as errors by this routine because they mean that some
// of the transactions given might be already accepted or soon going to be accepted. These
// transactions can be waited for in a usual way potentially with positive result.
func (a *Actor) Wait(mainHash, fbHash util.Uint256, vub uint32, err error) (*state.AppExecResult, error) {
	// #2248 will eventually remove this garbage from the code.
	if err != nil && !(strings.Contains(strings.ToLower(err.Error()), "already exists") || strings.Contains(strings.ToLower(err.Error()), "already on chain")) {
		return nil, err
	}
	return a.WaitAny(context.TODO(), vub, mainHash, fbHash)
}

// WaitSuccess works similar to [Actor.Wait], but checks that the main
// transaction was accepted and it has a HALT VM state (executed successfully).
// [state.AppExecResult] is still returned (if there is no error) in case you
// need some additional event or stack checks.
func (a *Actor) WaitSuccess(mainHash, fbHash util.Uint256, vub uint32, err error) (*state.AppExecResult, error) {
	aer, err := a.Wait(mainHash, fbHash, vub, err)
	if err != nil {
		return nil, err
	}
	if aer.Container != mainHash {
		return nil, ErrFallbackAccepted
	}
	if aer.VMState != vmstate.Halt {
		return nil, fmt.Errorf("%w: %s", actor.ErrExecFailed, aer.FaultException)
	}
	return aer, nil
}
