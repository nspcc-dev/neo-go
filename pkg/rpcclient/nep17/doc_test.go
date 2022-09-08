package nep17_test

import (
	"context"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

func ExampleTokenReader() {
	// No error checking done at all, intentionally.
	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Safe methods are reachable with just an invoker, no need for an account there.
	inv := invoker.New(c, nil)

	// NEP-17 contract hash.
	nep17Hash := util.Uint160{9, 8, 7}

	// And a reader interface.
	n17 := nep17.NewReader(inv, nep17Hash)

	// Get the metadata. Even though these methods are implemented in neptoken package,
	// they're available for NEP-17 wrappers.
	symbol, _ := n17.Symbol()
	supply, _ := n17.TotalSupply()
	_ = symbol
	_ = supply

	// Account hash we're interested in.
	accHash, _ := address.StringToUint160("NdypBhqkz2CMMnwxBgvoC9X2XjKF5axgKo")

	// Get account balance.
	balance, _ := n17.BalanceOf(accHash)
	_ = balance
}

func ExampleToken() {
	// No error checking done at all, intentionally.
	w, _ := wallet.NewWalletFromFile("somewhere")
	defer w.Close()

	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Create a simple CalledByEntry-scoped actor (assuming there is an account
	// inside the wallet).
	a, _ := actor.NewSimple(c, w.Accounts[0])

	// NEP-17 contract hash.
	nep17Hash := util.Uint160{9, 8, 7}

	// Create a complete NEP-17 contract representation.
	n17 := nep17.New(a, nep17Hash)

	tgtAcc, _ := address.StringToUint160("NdypBhqkz2CMMnwxBgvoC9X2XjKF5axgKo")

	// Send a transaction that transfers one token to another account.
	txid, vub, _ := n17.Transfer(a.Sender(), tgtAcc, big.NewInt(1), nil)
	_ = txid
	_ = vub
}
