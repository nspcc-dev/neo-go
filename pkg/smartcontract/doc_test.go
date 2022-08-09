package smartcontract_test

import (
	"context"
	"encoding/hex"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

func ExampleBuilder() {
	// No error checking done at all, intentionally.
	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})
	neoHash, _ := c.GetNativeContractHash("NeoToken")

	pKey, _ := hex.DecodeString("03d9e8b16bd9b22d3345d6d4cde31be1c3e1d161532e3d0ccecb95ece2eb58336e") // Public key.

	b := smartcontract.NewBuilder()
	// Single NEO "vote" call with a check
	b.InvokeWithAssert(neoHash, "vote", pKey)
	script, _ := b.Script()

	// The script can then be used to create transaction or to invoke via RPC.
	res, _ := c.InvokeScript(script, []transaction.Signer{{Account: util.Uint160{0x01, 0x02, 0x03}, Scopes: transaction.CalledByEntry}})
	if res.State != "HALT" {
		// The script failed
	}

	b.Reset() // Copy the old script above if you need it!

	w, _ := wallet.NewWalletFromFile("somewhere")
	// Assuming there is one Account inside
	a, _ := actor.NewSimple(c, w.Accounts[0])
	from := w.Accounts[0].Contract.ScriptHash() // Assuming Contract is present.

	// Multiple transfers in a single script. If any of them fail whole script fails.
	b.InvokeWithAssert(neoHash, "transfer", from, util.Uint160{0x70}, 1, nil)
	b.InvokeWithAssert(neoHash, "transfer", from, util.Uint160{0x71}, 10, []byte("data"))
	b.InvokeWithAssert(neoHash, "transfer", from, util.Uint160{0x72}, 1, nil)
	script, _ = b.Script()

	// The script can then be used to create transaction or to invoke via RPC.
	txid, vub, _ := a.SendRun(script)
	_ = txid
	_ = vub
}
