package invoker_test

import (
	"context"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neo"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
)

func ExampleInvoker() {
	// No error checking done at all, intentionally.
	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// A simple invoker with no signers, perfectly fine for reads from safe methods.
	inv := invoker.New(c, nil)

	// Get the NEO token supply (notice that unwrap is used to get the result).
	supply, _ := unwrap.BigInt(inv.Call(neo.Hash, "totalSupply"))
	_ = supply

	acc, _ := address.StringToUint160("NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq")
	// Get the NEO balance for account NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq.
	balance, _ := unwrap.BigInt(inv.Call(neo.Hash, "balanceOf", acc))
	_ = balance

	// Test-invoke transfer call.
	res, _ := inv.Call(neo.Hash, "transfer", acc, util.Uint160{1, 2, 3}, 1, nil)
	if res.State == vmstate.Halt.String() {
		panic("NEO is broken!") // inv has no signers and transfer requires a witness to be performed.
	} else { // nolint:revive // superfluous-else: if block ends with call to panic function, so drop this else and outdent its block (revive)
		println("ok") // this actually should fail
	}

	// A historic invoker with no signers at block 1000000.
	inv = invoker.NewHistoricAtHeight(1000000, c, nil)

	// It's the same call as above, but the data is for a state at block 1000000.
	balance, _ = unwrap.BigInt(inv.Call(neo.Hash, "balanceOf", acc))
	_ = balance

	// This invoker has a signer for NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq account with
	// CalledByEntry scope, which is sufficient for most operation. It uses current
	// state which is exactly what you need if you want to then create a transaction
	// with the same action.
	inv = invoker.New(c, []transaction.Signer{{Account: acc, Scopes: transaction.CalledByEntry}})

	// Now test invocation should be fine (if NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq has 1 NEO of course).
	res, _ = inv.Call(neo.Hash, "transfer", acc, util.Uint160{1, 2, 3}, 1, nil)
	if res.State == vmstate.Halt.String() {
		// transfer actually returns a value, so check it too.
		ok, _ := unwrap.Bool(res, nil)
		if ok {
			// OK, as expected.
			// res.Script contains the corresponding script.
			_ = res.Script
			// res.GasConsumed has an appropriate system fee required for a transaction.
			_ = res.GasConsumed
		}
	}

	// Now let's try working with iterators.
	nep11Contract := util.Uint160{1, 2, 3}

	var tokens [][]byte

	// Try doing it the right way, by traversing the iterator.
	sess, iter, err := unwrap.SessionIterator(inv.Call(nep11Contract, "tokensOf", acc))

	// The server doesn't support sessions and doesn't perform iterator expansion,
	// iterators can't be used.
	if err != nil {
		if errors.Is(err, unwrap.ErrNoSessionID) {
			// But if we expect some low number of elements, CallAndExpandIterator
			// can help us in this case. If the account has more than 10 elements,
			// some of them will be missing from the response.
			tokens, _ = unwrap.ArrayOfBytes(inv.CallAndExpandIterator(nep11Contract, "tokensOf", 10, acc))
		} else {
			panic("some error")
		}
	} else {
		items, err := inv.TraverseIterator(sess, &iter, 100)
		// Keep going until there are no more elements
		for err == nil && len(items) != 0 {
			for _, itm := range items {
				tokenID, _ := itm.TryBytes()
				tokens = append(tokens, tokenID)
			}
			items, err = inv.TraverseIterator(sess, &iter, 100)
		}
		// Let the server release the session.
		_ = inv.TerminateSession(sess)
	}
	_ = tokens
}
