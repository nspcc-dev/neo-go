package notary_test

import (
	"context"
	"math/big"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/gas"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/notary"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/policy"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

func ExampleActor() {
	// No error checking done at all, intentionally.
	w, _ := wallet.NewWalletFromFile("somewhere")
	defer w.Close()
	// We assume there are two accounts in the wallet --- one is a simple signature
	// account and another one is committee account. The first one will send notary
	// requests, while committee signatures need to be collected.

	// Create an RPC client.
	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// An actor for the first account.
	single, _ := actor.NewSimple(c, w.Accounts[0])

	// Transfer some GAS to the Notary contract to be able to send notary requests
	// from the first account.
	gasSingle := gas.New(single)
	txid, vub, _ := gasSingle.Transfer(single.Sender(), notary.Hash, big.NewInt(10_0000_0000), &notary.OnNEP17PaymentData{Till: 10000000})

	var depositOK bool
	// Wait for transaction to be persisted, either it gets in and we get
	// an application log with some result or it expires.
	for height, err := c.GetBlockCount(); err == nil && height <= vub; height, err = c.GetBlockCount() {
		appLog, err := c.GetApplicationLog(txid, nil)
		// We can't separate "application log missing" from other errors at the moment, see #2248.
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		if len(appLog.Executions) == 1 && appLog.Executions[0].VMState == vmstate.Halt {
			depositOK = true
		} else {
			break
		}
	}
	if !depositOK {
		panic("deposit failed")
	}

	var opts = new(notary.ActorOptions)
	// Add high priority attribute, we gonna be making committee-signed transactions anyway.
	opts.MainAttributes = []transaction.Attribute{{Type: transaction.HighPriority}}

	// Create an Actor with the simple account used for paying fees and committee
	// signature to be collected.
	multi, _ := notary.NewTunedActor(c, []actor.SignerAccount{{
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

	// Use the Policy contract to perform something requiring committee signature.
	policyContract := policy.New(multi)

	// Wrap a transaction to set storage price into a notary request. Fallback will
	// be create automatically and all appropriate attributes will be added to both
	// transactions.
	mainTx, fbTx, vub, _ := multi.Notarize(policyContract.SetStoragePriceTransaction(10))
	_ = mainTx
	_ = fbTx
	_ = vub
}
