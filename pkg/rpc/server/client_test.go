package server

import (
	"context"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestClient_NEP5(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := client.New(context.Background(), httpSrv.URL, client.Options{Network: netmode.UnitTestNet})
	require.NoError(t, err)

	h, err := util.Uint160DecodeStringLE(testContractHash)
	require.NoError(t, err)

	t.Run("Decimals", func(t *testing.T) {
		d, err := c.NEP5Decimals(h)
		require.NoError(t, err)
		require.EqualValues(t, 2, d)
	})
	t.Run("TotalSupply", func(t *testing.T) {
		s, err := c.NEP5TotalSupply(h)
		require.NoError(t, err)
		require.EqualValues(t, 1_000_000, s)
	})
	t.Run("Name", func(t *testing.T) {
		name, err := c.NEP5Name(h)
		require.NoError(t, err)
		require.Equal(t, "Rubl", name)
	})
	t.Run("Symbol", func(t *testing.T) {
		sym, err := c.NEP5Symbol(h)
		require.NoError(t, err)
		require.Equal(t, "RUB", sym)
	})
	t.Run("TokenInfo", func(t *testing.T) {
		tok, err := c.NEP5TokenInfo(h)
		require.NoError(t, err)
		require.Equal(t, h, tok.Hash)
		require.Equal(t, "Rubl", tok.Name)
		require.Equal(t, "RUB", tok.Symbol)
		require.EqualValues(t, 2, tok.Decimals)
	})
	t.Run("BalanceOf", func(t *testing.T) {
		acc := testchain.PrivateKeyByID(0).GetScriptHash()
		b, err := c.NEP5BalanceOf(h, acc)
		require.NoError(t, err)
		require.EqualValues(t, 877, b)
	})
}
