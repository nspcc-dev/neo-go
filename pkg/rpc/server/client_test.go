package server

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestClient_NEP17(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := client.New(context.Background(), httpSrv.URL, client.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	h, err := util.Uint160DecodeStringLE(testContractHash)
	require.NoError(t, err)

	t.Run("Decimals", func(t *testing.T) {
		d, err := c.NEP17Decimals(h)
		require.NoError(t, err)
		require.EqualValues(t, 2, d)
	})
	t.Run("TotalSupply", func(t *testing.T) {
		s, err := c.NEP17TotalSupply(h)
		require.NoError(t, err)
		require.EqualValues(t, 1_000_000, s)
	})
	t.Run("Symbol", func(t *testing.T) {
		sym, err := c.NEP17Symbol(h)
		require.NoError(t, err)
		require.Equal(t, "RUB", sym)
	})
	t.Run("TokenInfo", func(t *testing.T) {
		tok, err := c.NEP17TokenInfo(h)
		require.NoError(t, err)
		require.Equal(t, h, tok.Hash)
		require.Equal(t, "Rubl", tok.Name)
		require.Equal(t, "RUB", tok.Symbol)
		require.EqualValues(t, 2, tok.Decimals)
	})
	t.Run("BalanceOf", func(t *testing.T) {
		acc := testchain.PrivateKeyByID(0).GetScriptHash()
		b, err := c.NEP17BalanceOf(h, acc)
		require.NoError(t, err)
		require.EqualValues(t, 877, b)
	})
}

func TestAddNetworkFee(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := client.New(context.Background(), httpSrv.URL, client.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

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
		cFee, _ := fee.Calculate(chain.GetBaseExecFee(), accs[0].Contract.Script)
		require.Equal(t, int64(io.GetVarSize(tx))*feePerByte+cFee+10, tx.NetworkFee)
	})

	t.Run("Multi", func(t *testing.T) {
		tx := transaction.New(testchain.Network(), []byte{byte(opcode.PUSH1)}, 0)
		accs := getAccounts(t, 4)
		pubs := keys.PublicKeys{accs[1].PrivateKey().PublicKey(), accs[2].PrivateKey().PublicKey(), accs[3].PrivateKey().PublicKey()}
		require.NoError(t, accs[1].ConvertMultisig(2, pubs))
		require.NoError(t, accs[2].ConvertMultisig(2, pubs))
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
		require.NoError(t, accs[2].SignTx(tx))
		cFee, _ := fee.Calculate(chain.GetBaseExecFee(), accs[0].Contract.Script)
		cFeeM, _ := fee.Calculate(chain.GetBaseExecFee(), accs[1].Contract.Script)
		require.Equal(t, int64(io.GetVarSize(tx))*feePerByte+cFee+cFeeM+10, tx.NetworkFee)
	})
	t.Run("Contract", func(t *testing.T) {
		tx := transaction.New(testchain.Network(), []byte{byte(opcode.PUSH1)}, 0)
		priv := testchain.PrivateKeyByID(0)
		acc1 := wallet.NewAccountFromPrivateKey(priv)
		acc1.Contract.Deployed = true
		acc1.Contract.Script, _ = hex.DecodeString(verifyContractAVM)
		h, _ := util.Uint160DecodeStringLE(verifyContractHash)
		tx.ValidUntilBlock = chain.BlockHeight() + 10

		t.Run("Valid", func(t *testing.T) {
			acc0 := wallet.NewAccountFromPrivateKey(priv)
			tx.Signers = []transaction.Signer{
				{
					Account: acc0.PrivateKey().GetScriptHash(),
					Scopes:  transaction.CalledByEntry,
				},
				{
					Account: h,
					Scopes:  transaction.Global,
				},
			}
			require.NoError(t, c.AddNetworkFee(tx, 10, acc0, acc1))
			require.NoError(t, acc0.SignTx(tx))
			tx.Scripts = append(tx.Scripts, transaction.Witness{})
			require.NoError(t, chain.VerifyTx(tx))
		})
		t.Run("Invalid", func(t *testing.T) {
			acc0, err := wallet.NewAccount()
			require.NoError(t, err)
			tx.Signers = []transaction.Signer{
				{
					Account: acc0.PrivateKey().GetScriptHash(),
					Scopes:  transaction.CalledByEntry,
				},
				{
					Account: h,
					Scopes:  transaction.Global,
				},
			}
			require.Error(t, c.AddNetworkFee(tx, 10, acc0, acc1))
		})
		t.Run("InvalidContract", func(t *testing.T) {
			acc0 := wallet.NewAccountFromPrivateKey(priv)
			tx.Signers = []transaction.Signer{
				{
					Account: acc0.PrivateKey().GetScriptHash(),
					Scopes:  transaction.CalledByEntry,
				},
				{
					Account: util.Uint160{},
					Scopes:  transaction.Global,
				},
			}
			require.Error(t, c.AddNetworkFee(tx, 10, acc0, acc1))
		})
	})
}

func TestSignAndPushInvocationTx(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := client.New(context.Background(), httpSrv.URL, client.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	priv := testchain.PrivateKey(0)
	acc := wallet.NewAccountFromPrivateKey(priv)
	h, err := c.SignAndPushInvocationTx([]byte{byte(opcode.PUSH1)}, acc, 30, 0, []transaction.Signer{{
		Account: priv.GetScriptHash(),
		Scopes:  transaction.CalledByEntry,
	}})
	require.NoError(t, err)

	mp := chain.GetMemPool()
	tx, ok := mp.TryGetValue(h)
	require.True(t, ok)
	require.Equal(t, h, tx.Hash())
	require.EqualValues(t, 30, tx.SystemFee)
}

func TestPing(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()

	c, err := client.New(context.Background(), httpSrv.URL, client.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	require.NoError(t, c.Ping())
	require.NoError(t, rpcSrv.Shutdown())
	httpSrv.Close()
	require.Error(t, c.Ping())
}

func TestCreateTxFromScript(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := client.New(context.Background(), httpSrv.URL, client.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	priv := testchain.PrivateKey(0)
	acc := wallet.NewAccountFromPrivateKey(priv)
	t.Run("NoSystemFee", func(t *testing.T) {
		tx, err := c.CreateTxFromScript([]byte{byte(opcode.PUSH1)}, acc, -1, 10)
		require.NoError(t, err)
		require.True(t, tx.ValidUntilBlock > chain.BlockHeight())
		require.EqualValues(t, 30, tx.SystemFee) // PUSH1
		require.True(t, len(tx.Signers) == 1)
		require.Equal(t, acc.PrivateKey().GetScriptHash(), tx.Signers[0].Account)
	})
	t.Run("ProvideSystemFee", func(t *testing.T) {
		tx, err := c.CreateTxFromScript([]byte{byte(opcode.PUSH1)}, acc, 123, 10)
		require.NoError(t, err)
		require.True(t, tx.ValidUntilBlock > chain.BlockHeight())
		require.EqualValues(t, 123, tx.SystemFee)
		require.True(t, len(tx.Signers) == 1)
		require.Equal(t, acc.PrivateKey().GetScriptHash(), tx.Signers[0].Account)
	})
}

func TestCreateNEP17TransferTx(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := client.New(context.Background(), httpSrv.URL, client.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	priv := testchain.PrivateKeyByID(0)
	acc := wallet.NewAccountFromPrivateKey(priv)

	gasContractHash, err := c.GetNativeContractHash(nativenames.Gas)
	require.NoError(t, err)

	tx, err := c.CreateNEP17TransferTx(acc, util.Uint160{}, gasContractHash, 1000, 0)
	require.NoError(t, err)
	require.NoError(t, acc.SignTx(tx))
	require.NoError(t, chain.VerifyTx(tx))
	v := chain.GetTestVM(trigger.Application, tx, nil)
	v.LoadScriptWithFlags(tx.Script, callflag.All)
	require.NoError(t, v.Run())
}

func TestInvokeVerify(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := client.New(context.Background(), httpSrv.URL, client.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	contract, err := util.Uint160DecodeStringLE(verifyContractHash)
	require.NoError(t, err)

	t.Run("positive, with signer", func(t *testing.T) {
		res, err := c.InvokeContractVerify(contract, smartcontract.Params{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}})
		require.NoError(t, err)
		require.Equal(t, "HALT", res.State)
		require.Equal(t, 1, len(res.Stack))
		require.True(t, res.Stack[0].Value().(bool))
	})

	t.Run("positive, with signer and witness", func(t *testing.T) {
		res, err := c.InvokeContractVerify(contract, smartcontract.Params{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}}, transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH1), byte(opcode.RET)}})
		require.NoError(t, err)
		require.Equal(t, "HALT", res.State)
		require.Equal(t, 1, len(res.Stack))
		require.True(t, res.Stack[0].Value().(bool))
	})

	t.Run("error, invalid witness number", func(t *testing.T) {
		_, err := c.InvokeContractVerify(contract, smartcontract.Params{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}}, transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH1), byte(opcode.RET)}}, transaction.Witness{InvocationScript: []byte{byte(opcode.RET)}})
		require.Error(t, err)
	})

	t.Run("false", func(t *testing.T) {
		res, err := c.InvokeContractVerify(contract, smartcontract.Params{}, []transaction.Signer{{Account: util.Uint160{}}})
		require.NoError(t, err)
		require.Equal(t, "HALT", res.State)
		require.Equal(t, 1, len(res.Stack))
		require.False(t, res.Stack[0].Value().(bool))
	})
}
