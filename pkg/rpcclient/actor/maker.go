package actor

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
)

// TransactionCheckerModifier is a callback that receives the result of
// test-invocation and the transaction that can perform the same invocation
// on chain. This callback is accepted by methods that create transactions, it
// can examine both arguments and return an error if there is anything wrong
// there which will abort the creation process. Notice that when used this
// callback is completely responsible for invocation result checking, including
// checking for HALT execution state (so if you don't check for it in a callback
// you can send a transaction that is known to end up in FAULT state). It can
// also modify the transaction (see TransactionModifier).
type TransactionCheckerModifier func(r *result.Invoke, t *transaction.Transaction) error

// TransactionModifier is a callback that receives the transaction before
// it's signed from a method that creates signed transactions. It can check
// fees and other fields of the transaction and return an error if there is
// anything wrong there which will abort the creation process. It also can modify
// Nonce, SystemFee, NetworkFee and ValidUntilBlock values taking full
// responsibility on the effects of these modifications (smaller fee values, too
// low or too high ValidUntilBlock or bad Nonce can render transaction invalid).
// Modifying other fields is not supported. Mostly it's useful for increasing
// fee values since by default they're just enough for transaction to be
// successfully accepted and executed.
type TransactionModifier func(t *transaction.Transaction) error

// DefaultModifier is the default modifier, it does nothing.
func DefaultModifier(t *transaction.Transaction) error {
	return nil
}

// DefaultCheckerModifier is the default TransactionCheckerModifier, it checks
// for HALT state in the invocation result given to it and does nothing else.
func DefaultCheckerModifier(r *result.Invoke, t *transaction.Transaction) error {
	if r.State != vmstate.Halt.String() {
		return fmt.Errorf("script failed (%s state) due to an error: %s", r.State, r.FaultException)
	}
	return nil
}

// MakeCall creates a transaction that calls the given method of the given
// contract with the given parameters. Test call is performed and filtered through
// Actor-configured TransactionCheckerModifier. The resulting transaction has
// Actor-configured attributes added as well. If you need to override attributes
// and/or TransactionCheckerModifier use MakeTunedCall.
func (a *Actor) MakeCall(contract util.Uint160, method string, params ...any) (*transaction.Transaction, error) {
	return a.MakeTunedCall(contract, method, nil, nil, params...)
}

// MakeTunedCall creates a transaction with the given attributes (or Actor default
// ones if nil) that calls the given method of the given contract with the given
// parameters. It's filtered through the provided callback (or Actor default
// one's if nil, see TransactionCheckerModifier documentation also), so the
// process can be aborted and transaction can be modified before signing.
func (a *Actor) MakeTunedCall(contract util.Uint160, method string, attrs []transaction.Attribute, txHook TransactionCheckerModifier, params ...any) (*transaction.Transaction, error) {
	r, err := a.Call(contract, method, params...)
	return a.makeUncheckedWrapper(r, err, attrs, txHook)
}

// MakeRun creates a transaction with the given executable script. Test
// invocation of this script is performed and filtered through Actor's
// TransactionCheckerModifier. The resulting transaction has attributes that are
// configured for current Actor. If you need to override them or use a different
// TransactionCheckerModifier use MakeTunedRun.
func (a *Actor) MakeRun(script []byte) (*transaction.Transaction, error) {
	return a.MakeTunedRun(script, nil, nil)
}

// MakeTunedRun creates a transaction with the given attributes (or Actor default
// ones if nil) that executes the given script. It's filtered through the
// provided callback (if not nil, otherwise Actor default one is used, see
// TransactionCheckerModifier documentation also), so the process can be aborted
// and transaction can be modified before signing.
func (a *Actor) MakeTunedRun(script []byte, attrs []transaction.Attribute, txHook TransactionCheckerModifier) (*transaction.Transaction, error) {
	r, err := a.Run(script)
	return a.makeUncheckedWrapper(r, err, attrs, txHook)
}

func (a *Actor) makeUncheckedWrapper(r *result.Invoke, err error, attrs []transaction.Attribute, txHook TransactionCheckerModifier) (*transaction.Transaction, error) {
	if err != nil {
		return nil, fmt.Errorf("test invocation failed: %w", err)
	}
	return a.MakeUncheckedRun(r.Script, r.GasConsumed, attrs, func(tx *transaction.Transaction) error {
		if txHook == nil {
			txHook = a.opts.CheckerModifier
		}
		return txHook(r, tx)
	})
}

// MakeUncheckedRun creates a transaction with the given attributes (or Actor
// default ones if nil) that executes the given script and is expected to use
// up to sysfee GAS for its execution. The transaction is filtered through the
// provided callback (or Actor default one, see TransactionModifier documentation
// also), so the process can be aborted and transaction can be modified before
// signing. This method is mostly useful when test invocation is already
// performed and the script and required system fee values are already known.
func (a *Actor) MakeUncheckedRun(script []byte, sysfee int64, attrs []transaction.Attribute, txHook TransactionModifier) (*transaction.Transaction, error) {
	tx, err := a.MakeUnsignedUncheckedRun(script, sysfee, attrs)
	if err != nil {
		return nil, err
	}

	if txHook == nil {
		txHook = a.opts.Modifier
	}
	err = txHook(tx)
	if err != nil {
		return nil, err
	}
	err = a.Sign(tx)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// MakeUnsignedCall creates an unsigned transaction with the given attributes
// that calls the given method of the given contract with the given parameters.
// Test-invocation is performed and is expected to end up in HALT state, the
// transaction returned has correct SystemFee and NetworkFee values.
// TransactionModifier is not applied to the result of this method, but default
// attributes are used if attrs is nil.
func (a *Actor) MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...any) (*transaction.Transaction, error) {
	r, err := a.Call(contract, method, params...)
	return a.makeUnsignedWrapper(r, err, attrs)
}

// MakeUnsignedRun creates an unsigned transaction with the given attributes
// that executes the given script. Test-invocation is performed and is expected
// to end up in HALT state, the transaction returned has correct SystemFee and
// NetworkFee values. TransactionModifier is not applied to the result of this
// method, but default attributes are used if attrs is nil.
func (a *Actor) MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error) {
	r, err := a.Run(script)
	return a.makeUnsignedWrapper(r, err, attrs)
}

func (a *Actor) makeUnsignedWrapper(r *result.Invoke, err error, attrs []transaction.Attribute) (*transaction.Transaction, error) {
	if err != nil {
		return nil, fmt.Errorf("failed to test-invoke: %w", err)
	}
	err = DefaultCheckerModifier(r, nil) // We know it doesn't care about transaction anyway.
	if err != nil {
		return nil, err
	}
	return a.MakeUnsignedUncheckedRun(r.Script, r.GasConsumed, attrs)
}

// MakeUnsignedUncheckedRun creates an unsigned transaction containing the given
// script with the system fee value and attributes. It's expected to be used when
// test invocation is already done and the script and system fee value are already
// known to be good, so it doesn't do test invocation internally. But it fills
// Signers with Actor's signers, calculates proper ValidUntilBlock and NetworkFee
// values. The resulting transaction can be changed in its Nonce, SystemFee,
// NetworkFee and ValidUntilBlock values and then be signed and sent or
// exchanged via context.ParameterContext. TransactionModifier is not applied to
// the result of this method, but default attributes are used if attrs is nil.
func (a *Actor) MakeUnsignedUncheckedRun(script []byte, sysFee int64, attrs []transaction.Attribute) (*transaction.Transaction, error) {
	var err error

	if len(script) == 0 {
		return nil, errors.New("empty script")
	}
	if sysFee < 0 {
		return nil, errors.New("negative system fee")
	}

	if attrs == nil {
		attrs = a.opts.Attributes // Might as well be nil, but it's OK.
	}
	tx := transaction.New(script, sysFee)
	tx.Signers = a.txSigners
	tx.Attributes = attrs

	tx.ValidUntilBlock, err = a.CalculateValidUntilBlock()
	if err != nil {
		return nil, fmt.Errorf("calculating validUntilBlock: %w", err)
	}

	tx.Scripts = make([]transaction.Witness, len(a.signers))
	for i := range a.signers {
		if !a.signers[i].Account.Contract.Deployed {
			tx.Scripts[i].VerificationScript = a.signers[i].Account.Contract.Script
		}
	}
	// CalculateNetworkFee doesn't call Hash or Size, only serializes the
	// transaction via Bytes, so it's safe wrt internal caching.
	tx.NetworkFee, err = a.client.CalculateNetworkFee(tx)
	if err != nil {
		return nil, fmt.Errorf("calculating network fee: %w", err)
	}

	return tx, nil
}

// CalculateValidUntilBlock returns correct ValidUntilBlock value for a new
// transaction relative to the current blockchain height. It uses "height +
// number of validators + 1" formula suggesting shorter transaction lifetime
// than the usual "height + MaxValidUntilBlockIncrement" approach. Shorter
// lifetime can be useful to control transaction acceptance wait time because
// it can't be added into a block after ValidUntilBlock.
func (a *Actor) CalculateValidUntilBlock() (uint32, error) {
	blockCount, err := a.client.GetBlockCount()
	if err != nil {
		return 0, fmt.Errorf("can't get block count: %w", err)
	}
	var vc = uint32(a.version.Protocol.ValidatorsCount)
	var bestH = uint32(0)
	for h, n := range a.version.Protocol.ValidatorsHistory { // In case it's enabled.
		if h >= bestH && h <= blockCount {
			vc = n
			bestH = h
		}
	}

	return blockCount + vc + 1, nil
}
