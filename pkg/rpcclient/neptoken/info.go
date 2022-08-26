package neptoken

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// InfoClient is a set of RPC methods required to get all of the NEP-11/NEP-17
// token data.
type InfoClient interface {
	invoker.RPCInvoke

	GetContractStateByHash(hash util.Uint160) (*state.Contract, error)
}

// Info allows to get basic token info using RPC client.
func Info(c InfoClient, hash util.Uint160) (*wallet.Token, error) {
	cs, err := c.GetContractStateByHash(hash)
	if err != nil {
		return nil, err
	}
	var standard string
	for _, st := range cs.Manifest.SupportedStandards {
		if st == manifest.NEP17StandardName || st == manifest.NEP11StandardName {
			standard = st
			break
		}
	}
	if standard == "" {
		return nil, fmt.Errorf("contract %s is not NEP-11/NEP17", hash.StringLE())
	}
	b := New(invoker.New(c, nil), hash)
	symbol, err := b.Symbol()
	if err != nil {
		return nil, err
	}
	decimals, err := b.Decimals()
	if err != nil {
		return nil, err
	}
	return wallet.NewToken(hash, cs.Manifest.Name, symbol, int64(decimals), standard), nil
}
