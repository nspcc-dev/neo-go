package smartcontract_test

import (
	"context"
	"encoding/hex"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/gas"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neo"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

func ExampleBuilder() {
	// No error checking done at all, intentionally.
	w, _ := wallet.NewWalletFromFile("somewhere")
	defer w.Close()

	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Assuming there is one Account inside.
	a, _ := actor.NewSimple(c, w.Accounts[0])

	pKey, _ := hex.DecodeString("03d9e8b16bd9b22d3345d6d4cde31be1c3e1d161532e3d0ccecb95ece2eb58336e") // Public key.

	b := smartcontract.NewBuilder()
	// Transfer + vote in a single script with each action leaving return value on the stack.
	b.InvokeMethod(neo.Hash, "transfer", a.Sender(), util.Uint160{0xff}, 1, nil)
	b.InvokeMethod(neo.Hash, "vote", pKey)
	script, _ := b.Script()

	// Actor has an Invoker inside, so we can perform test invocation using the script.
	res, _ := a.Run(script)
	if res.State != "HALT" || len(res.Stack) != 2 {
		// The script failed completely or didn't return proper number of return values.
	}

	transferResult, _ := res.Stack[0].TryBool()
	voteResult, _ := res.Stack[1].TryBool()

	if !transferResult {
		// Transfer failed.
	}
	if !voteResult {
		// Vote failed.
	}

	b.Reset() // Copy the old script above if you need it!

	// Multiple transfers of different tokens in a single script. If any of
	// them fails whole script fails.
	b.InvokeWithAssert(neo.Hash, "transfer", a.Sender(), util.Uint160{0x70}, 1, nil)
	b.InvokeWithAssert(gas.Hash, "transfer", a.Sender(), util.Uint160{0x71}, 100000, []byte("data"))
	b.InvokeWithAssert(neo.Hash, "transfer", a.Sender(), util.Uint160{0x72}, 1, nil)
	script, _ = b.Script()

	// Now send a transaction with this script via an RPC node.
	txid, vub, _ := a.SendRun(script)
	_ = txid
	_ = vub
}
