package nep24_test

import (
	"context"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep24"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

func ExampleRoyaltyReader() {
	// No error checking done at all, intentionally.
	c, _ := rpcclient.New(context.Background(), "url", rpcclient.Options{})

	// Safe methods are reachable with just an invoker, no need for an account there.
	inv := invoker.New(c, nil)

	// NEP-24 contract hash.
	nep24Hash := util.Uint160{9, 8, 7}

	// And a reader interface.
	n24 := nep24.NewRoyaltyReader(inv, nep24Hash)

	// Get the royalty information for a token.
	tokenID := []byte("someTokenID")
	royaltyToken := util.Uint160{1, 2, 3}
	salePrice := big.NewInt(1000)
	royaltyInfo, _ := n24.RoyaltyInfo(tokenID, royaltyToken, salePrice)
	_ = royaltyInfo
}
