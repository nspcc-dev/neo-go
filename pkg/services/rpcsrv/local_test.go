package rpcsrv

import (
	"context"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/gas"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestLocalClient(t *testing.T) {
	_, rpcSrv, _ := initClearServerWithCustomConfig(t, func(cfg *config.Config) {
		// No addresses configured -> RPC server listens nothing (but it
		// has MaxGasInvoke, sessions and other stuff).
		cfg.ApplicationConfiguration.RPC.BasicService.Enabled = true
		cfg.ApplicationConfiguration.RPC.BasicService.Addresses = nil
		cfg.ApplicationConfiguration.RPC.TLSConfig.Addresses = nil
	})
	// RPC server listens nothing (not exposed in any way), but it works for internal clients.
	c, err := rpcclient.NewInternal(context.TODO(), rpcSrv.RegisterLocal)
	require.NoError(t, err)
	require.NoError(t, c.Init())

	// Invokers can use this client.
	gasReader := gas.NewReader(invoker.New(c, nil))
	d, err := gasReader.Decimals()
	require.NoError(t, err)
	require.EqualValues(t, 8, d)

	// Actors can use it as well
	priv := testchain.PrivateKeyByID(0)
	acc := wallet.NewAccountFromPrivateKey(priv)
	addr := priv.PublicKey().GetScriptHash()

	act, err := actor.NewSimple(c, acc)
	require.NoError(t, err)
	gasprom := gas.New(act)
	txHash, _, err := gasprom.Transfer(addr, util.Uint160{}, big.NewInt(1000), nil)
	require.NoError(t, err)
	// No new blocks are produced here, but the tx is OK and is in the mempool.
	txes, err := c.GetRawMemPool()
	require.NoError(t, err)
	require.Equal(t, []util.Uint256{txHash}, txes)
	// Subscriptions are checked by other tests.
}
