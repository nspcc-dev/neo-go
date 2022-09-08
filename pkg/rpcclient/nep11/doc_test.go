package nep11_test

import (
	"context"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep11"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

func ExampleNonDivisibleReader() {
	// No error checking done at all, intentionally.
	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Safe methods are reachable with just an invoker, no need for an account there.
	inv := invoker.New(c, nil)

	// NEP-11 contract hash.
	nep11Hash := util.Uint160{9, 8, 7}

	// Most of the time contracts are non-divisible, create a reader for nep11Hash.
	n11 := nep11.NewNonDivisibleReader(inv, nep11Hash)

	// Get the metadata. Even though these methods are implemented in neptoken package,
	// they're available for NEP-11 wrappers.
	symbol, _ := n11.Symbol()
	supply, _ := n11.TotalSupply()
	_ = symbol
	_ = supply

	// Account hash we're interested in.
	accHash, _ := address.StringToUint160("NdypBhqkz2CMMnwxBgvoC9X2XjKF5axgKo")

	// Get account balance.
	balance, _ := n11.BalanceOf(accHash)
	if balance.Sign() > 0 {
		// There are some tokens there, let's look at them.
		tokIter, _ := n11.TokensOf(accHash)

		for toks, err := tokIter.Next(10); err == nil && len(toks) > 0; toks, err = tokIter.Next(10) {
			for i := range toks {
				// We know the owner of the token, but let's check internal contract consistency.
				owner, _ := n11.OwnerOf(toks[i])
				if !owner.Equals(accHash) {
					panic("NEP-11 contract is broken!")
				}
			}
		}
	}
}

func ExampleDivisibleReader() {
	// No error checking done at all, intentionally.
	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Safe methods are reachable with just an invoker, no need for an account there.
	inv := invoker.New(c, nil)

	// NEP-11 contract hash.
	nep11Hash := util.Uint160{9, 8, 7}

	// Divisible contract are more rare, but we can handle them too.
	n11 := nep11.NewDivisibleReader(inv, nep11Hash)

	// Get the metadata. Even though these methods are implemented in neptoken package,
	// they're available for NEP-11 wrappers.
	symbol, _ := n11.Symbol()
	supply, _ := n11.TotalSupply()
	_ = symbol
	_ = supply

	// Account hash we're interested in.
	accHash, _ := address.StringToUint160("NdypBhqkz2CMMnwxBgvoC9X2XjKF5axgKo")

	// Get account balance.
	balance, _ := n11.BalanceOf(accHash)
	if balance.Sign() > 0 && balance.Cmp(big.NewInt(10)) < 0 {
		// We know we have a low number of tokens, so we can use a simple API to get them.
		toks, _ := n11.TokensOfExpanded(accHash, 10)

		// We can build a list of all owners of account's tokens.
		var owners = make([]util.Uint160, 0)
		for i := range toks {
			ownIter, _ := n11.OwnerOf(toks[i])
			for ows, err := ownIter.Next(10); err == nil && len(ows) > 0; ows, err = ownIter.Next(10) {
				// Notice that it includes accHash too.
				owners = append(owners, ows...)
			}
		}
		// The list can be sorted/deduplicated if needed.
		_ = owners
	}
}

func ExampleNonDivisible() {
	// No error checking done at all, intentionally.
	w, _ := wallet.NewWalletFromFile("somewhere")
	defer w.Close()

	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Create a simple CalledByEntry-scoped actor (assuming there is an account
	// inside the wallet).
	a, _ := actor.NewSimple(c, w.Accounts[0])

	// NEP-11 contract hash.
	nep11Hash := util.Uint160{9, 8, 7}

	// Create a complete non-divisible contract representation.
	n11 := nep11.NewNonDivisible(a, nep11Hash)

	tgtAcc, _ := address.StringToUint160("NdypBhqkz2CMMnwxBgvoC9X2XjKF5axgKo")

	// Let's tranfer all of account's tokens to some other account.
	tokIter, _ := n11.TokensOf(a.Sender())
	for toks, err := tokIter.Next(10); err == nil && len(toks) > 0; toks, err = tokIter.Next(10) {
		for i := range toks {
			// This creates a transaction for every token, but you can
			// create a script that will move multiple tokens in one
			// transaction with Builder from smartcontract package.
			txid, vub, _ := n11.Transfer(tgtAcc, toks[i], nil)
			_ = txid
			_ = vub
		}
	}
}

func ExampleDivisible() {
	// No error checking done at all, intentionally.
	w, _ := wallet.NewWalletFromFile("somewhere")
	defer w.Close()

	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Create a simple CalledByEntry-scoped actor (assuming there is an account
	// inside the wallet).
	a, _ := actor.NewSimple(c, w.Accounts[0])

	// NEP-11 contract hash.
	nep11Hash := util.Uint160{9, 8, 7}

	// Create a complete divisible contract representation.
	n11 := nep11.NewDivisible(a, nep11Hash)

	tgtAcc, _ := address.StringToUint160("NdypBhqkz2CMMnwxBgvoC9X2XjKF5axgKo")

	// Let's tranfer all of account's tokens to some other account.
	tokIter, _ := n11.TokensOf(a.Sender())
	for toks, err := tokIter.Next(10); err == nil && len(toks) > 0; toks, err = tokIter.Next(10) {
		for i := range toks {
			// It's a divisible token, so balance data is required in general case.
			balance, _ := n11.BalanceOfD(a.Sender(), toks[i])

			// This creates a transaction for every token, but you can
			// create a script that will move multiple tokens in one
			// transaction with Builder from smartcontract package.
			txid, vub, _ := n11.TransferD(a.Sender(), tgtAcc, balance, toks[i], nil)
			_ = txid
			_ = vub
		}
	}
}
