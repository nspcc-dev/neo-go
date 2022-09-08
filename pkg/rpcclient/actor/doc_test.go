package actor_test

import (
	"context"
	"encoding/json"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neo"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/policy"
	sccontext "github.com/nspcc-dev/neo-go/pkg/smartcontract/context"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

func ExampleActor() {
	// No error checking done at all, intentionally.
	w, _ := wallet.NewWalletFromFile("somewhere")
	defer w.Close()

	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Create a simple CalledByEntry-scoped actor (assuming there are accounts
	// inside the wallet).
	a, _ := actor.NewSimple(c, w.Accounts[0])

	customContract := util.Uint160{9, 8, 7}
	// Actor has an Invoker inside, so we can perform test invocations, it will
	// have a signer with the first wallet account and CalledByEntry scope.
	res, _ := a.Call(customContract, "method", 1, 2, 3)
	if res.State != vmstate.Halt.String() {
		// The call failed.
	}
	// All of the side-effects in res can be analyzed.

	// Now we want to send the same invocation in a transaction, but we already
	// have the script and a proper system fee for it, therefore SendUncheckedRun
	// can be used.
	txid, vub, _ := a.SendUncheckedRun(res.Script, res.GasConsumed, nil, nil)
	_ = txid
	_ = vub
	// You need to wait for it to persist and then check the on-chain result of it.

	// Now we want to send some transaction, but give it a priority by increasing
	// its network fee, this can be done with Tuned APIs.
	txid, vub, _ = a.SendTunedCall(customContract, "method", nil, func(r *result.Invoke, t *transaction.Transaction) error {
		// This code is run after the test-invocation done by *Call methods.
		// Reuse the default function to check for HALT execution state.
		err := actor.DefaultCheckerModifier(r, t)
		if err != nil {
			return err
		}
		// Some additional checks can be performed right here, but we only
		// want to raise the network fee by ~20%.
		t.NetworkFee += (t.NetworkFee / 5)
		return nil
	}, 1, 2, 3)
	_ = txid
	_ = vub

	// Actor can be used for higher-level wrappers as well, if we want to interact with
	// NEO then [neo] package can accept our Actor and allow to easily use NEO methods.
	neoContract := neo.New(a)
	balance, _ := neoContract.BalanceOf(a.Sender())
	_ = balance

	// Now suppose the second wallet account is a committee account. We want to
	// create and sign transactions for committee, but use the first account as
	// a sender (because committee account has no GAS). We at the same time want
	// to make all transactions using this actor high-priority ones, because
	// committee can use this attribute.

	// Get the default options to have CheckerModifier/Modifier set up correctly.
	opts := actor.NewDefaultOptions()
	// And override attributes.
	opts.Attributes = []transaction.Attribute{{Type: transaction.HighPriority}}

	// Create an Actor.
	a, _ = actor.NewTuned(c, []actor.SignerAccount{{
		// Sender, regular account with None scope.
		Signer: transaction.Signer{
			Account: w.Accounts[0].ScriptHash(),
			Scopes:  transaction.None,
		},
		Account: w.Accounts[0],
	}, {
		// Commmitee.
		Signer: transaction.Signer{
			Account: w.Accounts[1].ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: w.Accounts[1],
	}}, opts)

	// Use policy contract wrapper to simplify things. All changes in the
	// Policy contract are made by the committee.
	policyContract := policy.New(a)

	// Create a transaction to set storage price, it'll be high-priority and have two
	// signers from above. Committee is a multisignature account, so we can't sign/send
	// it right away, w.Accounts[1] has only one public key. Therefore, we need to
	// create a partially signed transaction and save it, then collect other signatures
	// and send.
	tx, _ := policyContract.SetStoragePriceUnsigned(10)

	net := a.GetNetwork()
	scCtx := sccontext.NewParameterContext(sccontext.TransactionType, net, tx)
	sign := w.Accounts[0].SignHashable(net, tx)
	_ = scCtx.AddSignature(w.Accounts[0].ScriptHash(), w.Accounts[0].Contract, w.Accounts[0].PublicKey(), sign)

	sign = w.Accounts[1].SignHashable(net, tx)
	_ = scCtx.AddSignature(w.Accounts[1].ScriptHash(), w.Accounts[1].Contract, w.Accounts[1].PublicKey(), sign)

	data, _ := json.Marshal(scCtx)
	_ = os.WriteFile("tx.json", data, 0644)

	// Signature collection is out of scope, usually it's manual for cases like this.
}
