package server

import (
	"context"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
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

func TestAddNetworkFee(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := client.New(context.Background(), httpSrv.URL, client.Options{Network: testchain.Network()})
	require.NoError(t, err)

	getAccounts := func(t *testing.T, n int) []*wallet.Account {
		accs := make([]*wallet.Account, n)
		var err error
		for i := range accs {
			accs[i], err = wallet.NewAccount()
			require.NoError(t, err)
		}
		return accs
	}

	feePerByte := chain.FeePerByte()

	t.Run("Invalid", func(t *testing.T) {
		tx := transaction.New(testchain.Network(), []byte{byte(opcode.PUSH1)}, 0)
		accs := getAccounts(t, 2)
		tx.Signers = []transaction.Signer{{
			Account: accs[0].PrivateKey().GetScriptHash(),
			Scopes:  transaction.CalledByEntry,
		}}
		require.Error(t, c.AddNetworkFee(tx, 10, accs[0], accs[1]))
	})
	t.Run("Simple", func(t *testing.T) {
		tx := transaction.New(testchain.Network(), []byte{byte(opcode.PUSH1)}, 0)
		accs := getAccounts(t, 1)
		tx.Signers = []transaction.Signer{{
			Account: accs[0].PrivateKey().GetScriptHash(),
			Scopes:  transaction.CalledByEntry,
		}}
		require.NoError(t, c.AddNetworkFee(tx, 10, accs[0]))
		require.NoError(t, accs[0].SignTx(tx))
		cFee, _ := core.CalculateNetworkFee(accs[0].Contract.Script)
		require.Equal(t, int64(io.GetVarSize(tx))*feePerByte+cFee+10, tx.NetworkFee)
	})

	t.Run("Multi", func(t *testing.T) {
		tx := transaction.New(testchain.Network(), []byte{byte(opcode.PUSH1)}, 0)
		accs := getAccounts(t, 3)
		pubs := keys.PublicKeys{accs[1].PrivateKey().PublicKey(), accs[2].PrivateKey().PublicKey()}
		require.NoError(t, accs[1].ConvertMultisig(1, pubs))
		require.NoError(t, accs[2].ConvertMultisig(1, pubs))
		tx.Signers = []transaction.Signer{
			{
				Account: accs[0].PrivateKey().GetScriptHash(),
				Scopes:  transaction.CalledByEntry,
			},
			{
				Account: hash.Hash160(accs[1].Contract.Script),
				Scopes:  transaction.Global,
			},
		}
		require.NoError(t, c.AddNetworkFee(tx, 10, accs[0], accs[1]))
		require.NoError(t, accs[0].SignTx(tx))
		require.NoError(t, accs[1].SignTx(tx))
		cFee, _ := core.CalculateNetworkFee(accs[0].Contract.Script)
		cFeeM, _ := core.CalculateNetworkFee(accs[1].Contract.Script)
		require.Equal(t, int64(io.GetVarSize(tx))*feePerByte+cFee+cFeeM+10, tx.NetworkFee)
	})
}
