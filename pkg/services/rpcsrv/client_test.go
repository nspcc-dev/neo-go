package rpcsrv

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/gas"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/management"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neo"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep11"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neptoken"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nns"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/notary"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/oracle"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/policy"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/rolemgmt"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_NEP17(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	h, err := util.Uint160DecodeStringLE(testContractHash)
	require.NoError(t, err)
	rub := nep17.NewReader(invoker.New(c, nil), h)

	t.Run("Decimals", func(t *testing.T) {
		d, err := rub.Decimals()
		require.NoError(t, err)
		require.EqualValues(t, 2, d)
	})
	t.Run("TotalSupply", func(t *testing.T) {
		s, err := rub.TotalSupply()
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(1_000_000), s)
	})
	t.Run("Symbol", func(t *testing.T) {
		sym, err := rub.Symbol()
		require.NoError(t, err)
		require.Equal(t, "RUB", sym)
	})
	t.Run("TokenInfo", func(t *testing.T) {
		tok, err := neptoken.Info(c, h)
		require.NoError(t, err)
		require.Equal(t, h, tok.Hash)
		require.Equal(t, "Rubl", tok.Name)
		require.Equal(t, "RUB", tok.Symbol)
		require.EqualValues(t, 2, tok.Decimals)
	})
	t.Run("BalanceOf", func(t *testing.T) {
		acc := testchain.PrivateKeyByID(0).GetScriptHash()
		b, err := rub.BalanceOf(acc)
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(877), b)
	})
}

func TestClientRoleManagement(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	act, err := actor.New(c, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: testchain.CommitteeScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: &wallet.Account{
			Address: testchain.CommitteeAddress(),
			Contract: &wallet.Contract{
				Script: testchain.CommitteeVerificationScript(),
			},
		},
	}})
	require.NoError(t, err)

	height, err := c.GetBlockCount()
	require.NoError(t, err)

	rm := rolemgmt.New(act)
	ks, err := rm.GetDesignatedByRole(noderoles.Oracle, height)
	require.NoError(t, err)
	require.Equal(t, 0, len(ks))

	testKeys := keys.PublicKeys{
		testchain.PrivateKeyByID(0).PublicKey(),
		testchain.PrivateKeyByID(1).PublicKey(),
		testchain.PrivateKeyByID(2).PublicKey(),
		testchain.PrivateKeyByID(3).PublicKey(),
	}

	tx, err := rm.DesignateAsRoleUnsigned(noderoles.Oracle, testKeys)
	require.NoError(t, err)

	tx.Scripts[0].InvocationScript = testchain.SignCommittee(tx)
	bl := testchain.NewBlock(t, chain, 1, 0, tx)
	_, err = c.SubmitBlock(*bl)
	require.NoError(t, err)

	sort.Sort(testKeys)
	ks, err = rm.GetDesignatedByRole(noderoles.Oracle, height+1)
	require.NoError(t, err)
	require.Equal(t, testKeys, ks)
}

func TestClientPolicyContract(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	polizei := policy.NewReader(invoker.New(c, nil))

	val, err := polizei.GetExecFeeFactor()
	require.NoError(t, err)
	require.Equal(t, int64(30), val)

	val, err = polizei.GetFeePerByte()
	require.NoError(t, err)
	require.Equal(t, int64(1000), val)

	val, err = polizei.GetStoragePrice()
	require.NoError(t, err)
	require.Equal(t, int64(100000), val)

	ret, err := polizei.IsBlocked(util.Uint160{})
	require.NoError(t, err)
	require.False(t, ret)

	act, err := actor.New(c, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: testchain.CommitteeScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: &wallet.Account{
			Address: testchain.CommitteeAddress(),
			Contract: &wallet.Contract{
				Script: testchain.CommitteeVerificationScript(),
			},
		},
	}})
	require.NoError(t, err)

	polis := policy.New(act)

	txexec, err := polis.SetExecFeeFactorUnsigned(100)
	require.NoError(t, err)

	txnetfee, err := polis.SetFeePerByteUnsigned(500)
	require.NoError(t, err)

	txstorage, err := polis.SetStoragePriceUnsigned(100500)
	require.NoError(t, err)

	txblock, err := polis.BlockAccountUnsigned(util.Uint160{1, 2, 3})
	require.NoError(t, err)

	for _, tx := range []*transaction.Transaction{txblock, txstorage, txnetfee, txexec} {
		tx.Scripts[0].InvocationScript = testchain.SignCommittee(tx)
	}

	bl := testchain.NewBlock(t, chain, 1, 0, txblock, txstorage, txnetfee, txexec)
	_, err = c.SubmitBlock(*bl)
	require.NoError(t, err)

	val, err = polizei.GetExecFeeFactor()
	require.NoError(t, err)
	require.Equal(t, int64(100), val)

	val, err = polizei.GetFeePerByte()
	require.NoError(t, err)
	require.Equal(t, int64(500), val)

	val, err = polizei.GetStoragePrice()
	require.NoError(t, err)
	require.Equal(t, int64(100500), val)

	ret, err = polizei.IsBlocked(util.Uint160{1, 2, 3})
	require.NoError(t, err)
	require.True(t, ret)
}

func TestClientManagementContract(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	manReader := management.NewReader(invoker.New(c, nil))

	fee, err := manReader.GetMinimumDeploymentFee()
	require.NoError(t, err)
	require.Equal(t, big.NewInt(10*1_0000_0000), fee)

	cs1, err := manReader.GetContract(gas.Hash)
	require.NoError(t, err)
	cs2, err := c.GetContractStateByHash(gas.Hash)
	require.NoError(t, err)
	require.Equal(t, cs2, cs1)
	cs1, err = manReader.GetContractByID(-6)
	require.NoError(t, err)
	require.Equal(t, cs2, cs1)

	ret, err := manReader.HasMethod(gas.Hash, "transfer", 4)
	require.NoError(t, err)
	require.True(t, ret)

	act, err := actor.New(c, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: testchain.CommitteeScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: &wallet.Account{
			Address: testchain.CommitteeAddress(),
			Contract: &wallet.Contract{
				Script: testchain.CommitteeVerificationScript(),
			},
		},
	}})
	require.NoError(t, err)

	ids, err := manReader.GetContractHashesExpanded(10)
	require.NoError(t, err)
	ctrs := make([]management.IDHash, 0)
	for i, s := range []string{testContractHash, verifyContractHash, verifyWithArgsContractHash, nnsContractHash, nfsoContractHash, storageContractHash} {
		h, err := util.Uint160DecodeStringLE(s)
		require.NoError(t, err)
		ctrs = append(ctrs, management.IDHash{ID: int32(i) + 1, Hash: h})
	}
	require.Equal(t, ctrs, ids)

	iter, err := manReader.GetContractHashes()
	require.NoError(t, err)
	ids, err = iter.Next(3)
	require.NoError(t, err)
	require.Equal(t, ctrs[:3], ids)
	ids, err = iter.Next(10)
	require.NoError(t, err)
	require.Equal(t, ctrs[3:], ids)

	man := management.New(act)

	txfee, err := man.SetMinimumDeploymentFeeUnsigned(big.NewInt(1 * 1_0000_0000))
	require.NoError(t, err)
	txdepl, err := man.DeployUnsigned(&cs1.NEF, &cs1.Manifest, nil) // Redeploy from a different account.
	require.NoError(t, err)

	for _, tx := range []*transaction.Transaction{txfee, txdepl} {
		tx.Scripts[0].InvocationScript = testchain.SignCommittee(tx)
	}

	bl := testchain.NewBlock(t, chain, 1, 0, txfee, txdepl)
	_, err = c.SubmitBlock(*bl)
	require.NoError(t, err)

	fee, err = manReader.GetMinimumDeploymentFee()
	require.NoError(t, err)
	require.Equal(t, big.NewInt(1_0000_0000), fee)

	appLog, err := c.GetApplicationLog(txdepl.Hash(), nil)
	require.NoError(t, err)
	require.Equal(t, vmstate.Halt, appLog.Executions[0].VMState)
	require.Equal(t, 1, len(appLog.Executions[0].Events))
}

func TestClientNEOContract(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	neoR := neo.NewReader(invoker.New(c, nil))

	sym, err := neoR.Symbol()
	require.NoError(t, err)
	require.Equal(t, "NEO", sym)

	dec, err := neoR.Decimals()
	require.NoError(t, err)
	require.Equal(t, 0, dec)

	ts, err := neoR.TotalSupply()
	require.NoError(t, err)
	require.Equal(t, big.NewInt(1_0000_0000), ts)

	comm, err := neoR.GetCommittee()
	require.NoError(t, err)
	commScript, err := smartcontract.CreateMajorityMultiSigRedeemScript(comm)
	require.NoError(t, err)
	require.Equal(t, testchain.CommitteeScriptHash(), hash.Hash160(commScript))

	vals, err := neoR.GetNextBlockValidators()
	require.NoError(t, err)
	valsScript, err := smartcontract.CreateDefaultMultiSigRedeemScript(vals)
	require.NoError(t, err)
	require.Equal(t, testchain.MultisigScriptHash(), hash.Hash160(valsScript))

	gpb, err := neoR.GetGasPerBlock()
	require.NoError(t, err)
	require.Equal(t, int64(5_0000_0000), gpb)

	regP, err := neoR.GetRegisterPrice()
	require.NoError(t, err)
	require.Equal(t, int64(1000_0000_0000), regP)

	acc0 := testchain.PrivateKey(0).PublicKey().GetScriptHash()
	uncl, err := neoR.UnclaimedGas(acc0, chain.BlockHeight()+1)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(10000), uncl)

	accState, err := neoR.GetAccountState(acc0)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(1000), &accState.Balance)
	require.Equal(t, uint32(4), accState.BalanceHeight)

	cands, err := neoR.GetCandidates()
	require.NoError(t, err)
	require.Equal(t, 0, len(cands)) // No registrations.

	cands, err = neoR.GetAllCandidatesExpanded(100)
	require.NoError(t, err)
	require.Equal(t, 0, len(cands)) // No registrations.

	iter, err := neoR.GetAllCandidates()
	require.NoError(t, err)
	cands, err = iter.Next(10)
	require.NoError(t, err)
	require.Equal(t, 0, len(cands)) // No registrations.
	require.NoError(t, iter.Terminate())

	act, err := actor.New(c, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: testchain.CommitteeScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: &wallet.Account{
			Address: testchain.CommitteeAddress(),
			Contract: &wallet.Contract{
				Script: testchain.CommitteeVerificationScript(),
			},
		},
	}})

	require.NoError(t, err)

	neoC := neo.New(act)

	txgpb, err := neoC.SetGasPerBlockUnsigned(10 * 1_0000_0000)
	require.NoError(t, err)
	txregp, err := neoC.SetRegisterPriceUnsigned(1_0000)
	require.NoError(t, err)

	for _, tx := range []*transaction.Transaction{txgpb, txregp} {
		tx.Scripts[0].InvocationScript = testchain.SignCommittee(tx)
	}

	bl := testchain.NewBlock(t, chain, 1, 0, txgpb, txregp)
	_, err = c.SubmitBlock(*bl)
	require.NoError(t, err)

	gpb, err = neoR.GetGasPerBlock()
	require.NoError(t, err)
	require.Equal(t, int64(10_0000_0000), gpb)

	regP, err = neoR.GetRegisterPrice()
	require.NoError(t, err)
	require.Equal(t, int64(10000), regP)

	act0, err := actor.NewSimple(c, wallet.NewAccountFromPrivateKey(testchain.PrivateKey(0)))
	require.NoError(t, err)
	neo0 := neo.New(act0)

	txreg, err := neo0.RegisterCandidateTransaction(testchain.PrivateKey(0).PublicKey())
	require.NoError(t, err)
	bl = testchain.NewBlock(t, chain, 1, 0, txreg)
	_, err = c.SubmitBlock(*bl)
	require.NoError(t, err)

	txvote, err := neo0.VoteTransaction(acc0, testchain.PrivateKey(0).PublicKey())
	require.NoError(t, err)
	bl = testchain.NewBlock(t, chain, 1, 0, txvote)
	_, err = c.SubmitBlock(*bl)
	require.NoError(t, err)

	txunreg, err := neo0.UnregisterCandidateTransaction(testchain.PrivateKey(0).PublicKey())
	require.NoError(t, err)
	bl = testchain.NewBlock(t, chain, 1, 0, txunreg)
	_, err = c.SubmitBlock(*bl)
	require.NoError(t, err)
}

func TestClientNotary(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	notaReader := notary.NewReader(invoker.New(c, nil))

	priv0 := testchain.PrivateKeyByID(0)
	priv0Hash := priv0.PublicKey().GetScriptHash()
	bal, err := notaReader.BalanceOf(priv0Hash)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(10_0000_0000), bal)

	expir, err := notaReader.ExpirationOf(priv0Hash)
	require.NoError(t, err)
	require.Equal(t, uint32(1007), expir)

	maxNVBd, err := notaReader.GetMaxNotValidBeforeDelta()
	require.NoError(t, err)
	require.Equal(t, uint32(140), maxNVBd)

	feePerKey, err := notaReader.GetNotaryServiceFeePerKey()
	require.NoError(t, err)
	require.Equal(t, int64(1000_0000), feePerKey)

	commAct, err := actor.New(c, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: testchain.CommitteeScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: &wallet.Account{
			Address: testchain.CommitteeAddress(),
			Contract: &wallet.Contract{
				Script: testchain.CommitteeVerificationScript(),
			},
		},
	}})
	require.NoError(t, err)
	notaComm := notary.New(commAct)

	txNVB, err := notaComm.SetMaxNotValidBeforeDeltaUnsigned(210)
	require.NoError(t, err)
	txFee, err := notaComm.SetNotaryServiceFeePerKeyUnsigned(500_0000)
	require.NoError(t, err)

	txNVB.Scripts[0].InvocationScript = testchain.SignCommittee(txNVB)
	txFee.Scripts[0].InvocationScript = testchain.SignCommittee(txFee)
	bl := testchain.NewBlock(t, chain, 1, 0, txNVB, txFee)
	_, err = c.SubmitBlock(*bl)
	require.NoError(t, err)

	maxNVBd, err = notaReader.GetMaxNotValidBeforeDelta()
	require.NoError(t, err)
	require.Equal(t, uint32(210), maxNVBd)

	feePerKey, err = notaReader.GetNotaryServiceFeePerKey()
	require.NoError(t, err)
	require.Equal(t, int64(500_0000), feePerKey)

	privAct, err := actor.New(c, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: priv0Hash,
			Scopes:  transaction.CalledByEntry,
		},
		Account: wallet.NewAccountFromPrivateKey(priv0),
	}})
	require.NoError(t, err)
	notaPriv := notary.New(privAct)

	txLock, err := notaPriv.LockDepositUntilTransaction(priv0Hash, 1111)
	require.NoError(t, err)

	bl = testchain.NewBlock(t, chain, 1, 0, txLock)
	_, err = c.SubmitBlock(*bl)
	require.NoError(t, err)

	expir, err = notaReader.ExpirationOf(priv0Hash)
	require.NoError(t, err)
	require.Equal(t, uint32(1111), expir)

	_, err = notaPriv.WithdrawTransaction(priv0Hash, priv0Hash)
	require.Error(t, err) // Can't be withdrawn until 1111.
}

func TestAddNetworkFeeCalculateNetworkFee(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()
	const extraFee = 10
	var nonce uint32

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
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
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		accs := getAccounts(t, 2)
		tx.Signers = []transaction.Signer{{
			Account: accs[0].PrivateKey().GetScriptHash(),
			Scopes:  transaction.CalledByEntry,
		}}
		require.Error(t, c.AddNetworkFee(tx, extraFee, accs[0], accs[1])) //nolint:staticcheck // SA1019: c.AddNetworkFee is deprecated
	})
	t.Run("Simple", func(t *testing.T) {
		acc0 := wallet.NewAccountFromPrivateKey(testchain.PrivateKeyByID(0))
		check := func(t *testing.T, extraFee int64) {
			tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
			tx.ValidUntilBlock = 25
			tx.Signers = []transaction.Signer{{
				Account: acc0.PrivateKey().GetScriptHash(),
				Scopes:  transaction.CalledByEntry,
			}}
			tx.Nonce = nonce
			nonce++

			tx.Scripts = []transaction.Witness{
				{VerificationScript: acc0.GetVerificationScript()},
			}
			actualCalculatedNetFee, err := c.CalculateNetworkFee(tx)
			require.NoError(t, err)

			tx.Scripts = nil
			require.NoError(t, c.AddNetworkFee(tx, extraFee, acc0)) //nolint:staticcheck // SA1019: c.AddNetworkFee is deprecated
			actual := tx.NetworkFee

			require.NoError(t, acc0.SignTx(testchain.Network(), tx))
			cFee, _ := fee.Calculate(chain.GetBaseExecFee(), acc0.Contract.Script)
			expected := int64(io.GetVarSize(tx))*feePerByte + cFee + extraFee

			require.Equal(t, expected, actual)
			require.Equal(t, expected, actualCalculatedNetFee+extraFee)
			err = chain.VerifyTx(tx)
			if extraFee < 0 {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}

		t.Run("with extra fee", func(t *testing.T) {
			// check that calculated network fee with extra value is enough
			check(t, extraFee)
		})
		t.Run("without extra fee", func(t *testing.T) {
			// check that calculated network fee without extra value is enough
			check(t, 0)
		})
		t.Run("exactFee-1", func(t *testing.T) {
			// check that we don't add unexpected extra GAS
			check(t, -1)
		})
	})

	t.Run("Multi", func(t *testing.T) {
		acc0 := wallet.NewAccountFromPrivateKey(testchain.PrivateKeyByID(0))
		acc1 := wallet.NewAccountFromPrivateKey(testchain.PrivateKeyByID(0))
		err = acc1.ConvertMultisig(3, keys.PublicKeys{
			testchain.PrivateKeyByID(0).PublicKey(),
			testchain.PrivateKeyByID(1).PublicKey(),
			testchain.PrivateKeyByID(2).PublicKey(),
			testchain.PrivateKeyByID(3).PublicKey(),
		})
		require.NoError(t, err)
		check := func(t *testing.T, extraFee int64) {
			tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
			tx.ValidUntilBlock = 25
			tx.Signers = []transaction.Signer{
				{
					Account: acc0.PrivateKey().GetScriptHash(),
					Scopes:  transaction.CalledByEntry,
				},
				{
					Account: hash.Hash160(acc1.Contract.Script),
					Scopes:  transaction.Global,
				},
			}
			tx.Nonce = nonce
			nonce++

			tx.Scripts = []transaction.Witness{
				{VerificationScript: acc0.GetVerificationScript()},
				{VerificationScript: acc1.GetVerificationScript()},
			}
			actualCalculatedNetFee, err := c.CalculateNetworkFee(tx)
			require.NoError(t, err)

			tx.Scripts = nil

			require.NoError(t, c.AddNetworkFee(tx, extraFee, acc0, acc1)) //nolint:staticcheck // SA1019: c.AddNetworkFee is deprecated
			actual := tx.NetworkFee

			require.NoError(t, acc0.SignTx(testchain.Network(), tx))
			tx.Scripts = append(tx.Scripts, transaction.Witness{
				InvocationScript:   testchain.Sign(tx),
				VerificationScript: acc1.Contract.Script,
			})
			cFee, _ := fee.Calculate(chain.GetBaseExecFee(), acc0.Contract.Script)
			cFeeM, _ := fee.Calculate(chain.GetBaseExecFee(), acc1.Contract.Script)
			expected := int64(io.GetVarSize(tx))*feePerByte + cFee + cFeeM + extraFee

			require.Equal(t, expected, actual)
			require.Equal(t, expected, actualCalculatedNetFee+extraFee)
			err = chain.VerifyTx(tx)
			if extraFee < 0 {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}

		t.Run("with extra fee", func(t *testing.T) {
			// check that calculated network fee with extra value is enough
			check(t, extraFee)
		})
		t.Run("without extra fee", func(t *testing.T) {
			// check that calculated network fee without extra value is enough
			check(t, 0)
		})
		t.Run("exactFee-1", func(t *testing.T) {
			// check that we don't add unexpected extra GAS
			check(t, -1)
		})
	})
	t.Run("Contract", func(t *testing.T) {
		h, err := util.Uint160DecodeStringLE(verifyContractHash)
		require.NoError(t, err)
		priv := testchain.PrivateKeyByID(0)
		acc0 := wallet.NewAccountFromPrivateKey(priv)
		acc1 := wallet.NewAccountFromPrivateKey(priv) // contract account
		acc1.Contract.Deployed = true
		acc1.Contract.Script, err = base64.StdEncoding.DecodeString(verifyContractAVM)
		require.NoError(t, err)

		newTx := func(t *testing.T) *transaction.Transaction {
			tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
			tx.ValidUntilBlock = chain.BlockHeight() + 10
			return tx
		}

		t.Run("Valid", func(t *testing.T) {
			check := func(t *testing.T, extraFee int64) {
				tx := newTx(t)
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
				// we need to fill standard verification scripts to use CalculateNetworkFee.
				tx.Scripts = []transaction.Witness{
					{VerificationScript: acc0.GetVerificationScript()},
					{},
				}
				actual, err := c.CalculateNetworkFee(tx)
				require.NoError(t, err)
				tx.Scripts = nil

				require.NoError(t, c.AddNetworkFee(tx, extraFee, acc0, acc1)) //nolint:staticcheck // SA1019: c.AddNetworkFee is deprecated
				require.NoError(t, acc0.SignTx(testchain.Network(), tx))
				tx.Scripts = append(tx.Scripts, transaction.Witness{})
				require.Equal(t, tx.NetworkFee, actual+extraFee)
				err = chain.VerifyTx(tx)
				if extraFee < 0 {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			}

			t.Run("with extra fee", func(t *testing.T) {
				// check that calculated network fee with extra value is enough
				check(t, extraFee)
			})
			t.Run("without extra fee", func(t *testing.T) {
				// check that calculated network fee without extra value is enough
				check(t, 0)
			})
			t.Run("exactFee-1", func(t *testing.T) {
				// check that we don't add unexpected extra GAS
				check(t, -1)
			})
		})
		t.Run("Invalid", func(t *testing.T) {
			tx := newTx(t)
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
			require.Error(t, c.AddNetworkFee(tx, 10, acc0, acc1)) //nolint:staticcheck // SA1019: c.AddNetworkFee is deprecated
		})
		t.Run("InvalidContract", func(t *testing.T) {
			tx := newTx(t)
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
			require.Error(t, c.AddNetworkFee(tx, 10, acc0, acc1)) //nolint:staticcheck // SA1019: c.AddNetworkFee is deprecated
		})
	})
}

func TestCalculateNetworkFee(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()
	const extraFee = 10

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	t.Run("ContractWithArgs", func(t *testing.T) {
		check := func(t *testing.T, extraFee int64) {
			h, err := util.Uint160DecodeStringLE(verifyWithArgsContractHash)
			require.NoError(t, err)
			priv := testchain.PrivateKeyByID(0)
			acc0 := wallet.NewAccountFromPrivateKey(priv)
			tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
			require.NoError(t, err)
			tx.ValidUntilBlock = chain.BlockHeight() + 10
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

			bw := io.NewBufBinWriter()
			emit.Bool(bw.BinWriter, false)
			emit.Int(bw.BinWriter, int64(4))
			emit.String(bw.BinWriter, "good_string") // contract's `verify` return `true` with this string
			require.NoError(t, bw.Err)
			contractInv := bw.Bytes()
			// we need to fill standard verification scripts to use CalculateNetworkFee.
			tx.Scripts = []transaction.Witness{
				{VerificationScript: acc0.GetVerificationScript()},
				{InvocationScript: contractInv},
			}
			tx.NetworkFee, err = c.CalculateNetworkFee(tx)
			require.NoError(t, err)
			tx.NetworkFee += extraFee
			tx.Scripts = nil

			require.NoError(t, acc0.SignTx(testchain.Network(), tx))
			tx.Scripts = append(tx.Scripts, transaction.Witness{InvocationScript: contractInv})
			err = chain.VerifyTx(tx)
			if extraFee < 0 {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}

		t.Run("with extra fee", func(t *testing.T) {
			// check that calculated network fee with extra value is enough
			check(t, extraFee)
		})
		t.Run("without extra fee", func(t *testing.T) {
			// check that calculated network fee without extra value is enough
			check(t, 0)
		})
		t.Run("exactFee-1", func(t *testing.T) {
			// check that we don't add unexpected extra GAS
			check(t, -1)
		})
	})
}
func TestSignAndPushInvocationTx(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	priv0 := testchain.PrivateKeyByID(0)
	acc0 := wallet.NewAccountFromPrivateKey(priv0)

	verifyWithoutParamsCtr, err := util.Uint160DecodeStringLE(verifyContractHash)
	require.NoError(t, err)
	acc1 := &wallet.Account{
		Address: address.Uint160ToString(verifyWithoutParamsCtr),
		Contract: &wallet.Contract{
			Parameters: []wallet.ContractParam{},
			Deployed:   true,
		},
		Default: false,
	}

	verifyWithParamsCtr, err := util.Uint160DecodeStringLE(verifyWithArgsContractHash)
	require.NoError(t, err)
	acc2 := &wallet.Account{
		Address: address.Uint160ToString(verifyWithParamsCtr),
		Contract: &wallet.Contract{
			Parameters: []wallet.ContractParam{
				{Name: "argString", Type: smartcontract.StringType},
				{Name: "argInt", Type: smartcontract.IntegerType},
				{Name: "argBool", Type: smartcontract.BoolType},
			},
			Deployed: true,
		},
		Default: false,
	}

	priv3 := testchain.PrivateKeyByID(3)
	acc3 := wallet.NewAccountFromPrivateKey(priv3)

	check := func(t *testing.T, h util.Uint256) {
		mp := chain.GetMemPool()
		tx, ok := mp.TryGetValue(h)
		require.True(t, ok)
		require.Equal(t, h, tx.Hash())
		require.EqualValues(t, 30, tx.SystemFee)
	}

	t.Run("good", func(t *testing.T) {
		t.Run("signer0: sig", func(t *testing.T) {
			h, err := c.SignAndPushInvocationTx([]byte{byte(opcode.PUSH1)}, acc0, 30, 0, []rpcclient.SignerAccount{ //nolint:staticcheck // SA1019: c.SignAndPushInvocationTx is deprecated
				{
					Signer: transaction.Signer{
						Account: priv0.GetScriptHash(),
						Scopes:  transaction.CalledByEntry,
					},
					Account: acc0,
				},
			})
			require.NoError(t, err)
			check(t, h)
		})
		t.Run("signer0: sig; signer1: sig", func(t *testing.T) {
			h, err := c.SignAndPushInvocationTx([]byte{byte(opcode.PUSH1)}, acc0, 30, 0, []rpcclient.SignerAccount{ //nolint:staticcheck // SA1019: c.SignAndPushInvocationTx is deprecated
				{
					Signer: transaction.Signer{
						Account: priv0.GetScriptHash(),
						Scopes:  transaction.CalledByEntry,
					},
					Account: acc0,
				},
				{
					Signer: transaction.Signer{
						Account: priv3.GetScriptHash(),
						Scopes:  transaction.CalledByEntry,
					},
					Account: acc3,
				},
			})
			require.NoError(t, err)
			check(t, h)
		})
		t.Run("signer0: sig; signer1: contract-based paramless", func(t *testing.T) {
			h, err := c.SignAndPushInvocationTx([]byte{byte(opcode.PUSH1)}, acc0, 30, 0, []rpcclient.SignerAccount{ //nolint:staticcheck // SA1019: c.SignAndPushInvocationTx is deprecated
				{
					Signer: transaction.Signer{
						Account: priv0.GetScriptHash(),
						Scopes:  transaction.CalledByEntry,
					},
					Account: acc0,
				},
				{
					Signer: transaction.Signer{
						Account: verifyWithoutParamsCtr,
						Scopes:  transaction.CalledByEntry,
					},
					Account: acc1,
				},
			})
			require.NoError(t, err)
			check(t, h)
		})
	})
	t.Run("error", func(t *testing.T) {
		t.Run("signer0: sig; signer1: contract-based with params", func(t *testing.T) {
			_, err := c.SignAndPushInvocationTx([]byte{byte(opcode.PUSH1)}, acc0, 30, 0, []rpcclient.SignerAccount{ //nolint:staticcheck // SA1019: c.SignAndPushInvocationTx is deprecated
				{
					Signer: transaction.Signer{
						Account: priv0.GetScriptHash(),
						Scopes:  transaction.CalledByEntry,
					},
					Account: acc0,
				},
				{
					Signer: transaction.Signer{
						Account: verifyWithParamsCtr,
						Scopes:  transaction.CalledByEntry,
					},
					Account: acc2,
				},
			})
			require.Error(t, err)
		})
		t.Run("signer0: sig; signer1: locked sig", func(t *testing.T) {
			pk, err := keys.NewPrivateKey()
			require.NoError(t, err)
			acc4 := &wallet.Account{
				Address: address.Uint160ToString(pk.GetScriptHash()),
				Contract: &wallet.Contract{
					Script:     pk.PublicKey().GetVerificationScript(),
					Parameters: []wallet.ContractParam{{Name: "parameter0", Type: smartcontract.SignatureType}},
				},
			}
			_, err = c.SignAndPushInvocationTx([]byte{byte(opcode.PUSH1)}, acc0, 30, 0, []rpcclient.SignerAccount{ //nolint:staticcheck // SA1019: c.SignAndPushInvocationTx is deprecated
				{
					Signer: transaction.Signer{
						Account: priv0.GetScriptHash(),
						Scopes:  transaction.CalledByEntry,
					},
					Account: acc0,
				},
				{
					Signer: transaction.Signer{
						Account: util.Uint160{1, 2, 3},
						Scopes:  transaction.CalledByEntry,
					},
					Account: acc4,
				},
			})
			require.Error(t, err)
		})
	})
}

func TestNotaryActor(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChainAndServices(t, false, true, false)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)

	sender := testchain.PrivateKeyByID(0) // owner of the deposit in testchain
	acc := wallet.NewAccountFromPrivateKey(sender)

	comm, err := c.GetCommittee()
	require.NoError(t, err)

	multiAcc := &wallet.Account{}
	*multiAcc = *acc
	require.NoError(t, multiAcc.ConvertMultisig(smartcontract.GetMajorityHonestNodeCount(len(comm)), comm))

	nact, err := notary.NewActor(c, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: multiAcc.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: multiAcc,
	}}, acc)
	require.NoError(t, err)
	neoW := neo.New(nact)
	_, _, _, err = nact.Notarize(neoW.SetRegisterPriceTransaction(1_0000_0000))
	require.NoError(t, err)
}

func TestSignAndPushP2PNotaryRequest(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChainAndServices(t, false, true, false)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	acc, err := wallet.NewAccount()
	require.NoError(t, err)

	t.Run("client wasn't initialized", func(t *testing.T) {
		_, err := c.SignAndPushP2PNotaryRequest(transaction.New([]byte{byte(opcode.RET)}, 123), []byte{byte(opcode.RET)}, -1, 0, 100, acc) //nolint:staticcheck // SA1019: c.SignAndPushP2PNotaryRequest is deprecated
		require.NotNil(t, err)
	})

	require.NoError(t, c.Init())
	t.Run("bad fallback script", func(t *testing.T) {
		_, err := c.SignAndPushP2PNotaryRequest(nil, []byte{byte(opcode.ASSERT)}, -1, 0, 0, acc) //nolint:staticcheck // SA1019: c.SignAndPushP2PNotaryRequest is deprecated
		require.NotNil(t, err)
	})

	t.Run("too large fallbackValidFor", func(t *testing.T) {
		_, err := c.SignAndPushP2PNotaryRequest(nil, []byte{byte(opcode.RET)}, -1, 0, 141, acc) //nolint:staticcheck // SA1019: c.SignAndPushP2PNotaryRequest is deprecated
		require.NotNil(t, err)
	})

	t.Run("good", func(t *testing.T) {
		sender := testchain.PrivateKeyByID(0) // owner of the deposit in testchain
		acc := wallet.NewAccountFromPrivateKey(sender)
		expected := transaction.Transaction{
			Attributes:      []transaction.Attribute{{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 1}}},
			Script:          []byte{byte(opcode.RET)},
			ValidUntilBlock: chain.BlockHeight() + 5,
			Signers:         []transaction.Signer{{Account: util.Uint160{1, 5, 9}}},
			Scripts: []transaction.Witness{{
				InvocationScript:   []byte{1, 4, 7},
				VerificationScript: []byte{3, 6, 9},
			}},
		}
		mainTx := expected
		_ = expected.Hash()
		req, err := c.SignAndPushP2PNotaryRequest(&mainTx, []byte{byte(opcode.RET)}, -1, 0, 6, acc) //nolint:staticcheck // SA1019: c.SignAndPushP2PNotaryRequest is deprecated
		require.NoError(t, err)

		// check that request was correctly completed
		require.Equal(t, expected, *req.MainTransaction) // main tx should be the same
		require.ElementsMatch(t, []transaction.Attribute{
			{
				Type:  transaction.NotaryAssistedT,
				Value: &transaction.NotaryAssisted{NKeys: 0},
			},
			{
				Type:  transaction.NotValidBeforeT,
				Value: &transaction.NotValidBefore{Height: chain.BlockHeight()},
			},
			{
				Type:  transaction.ConflictsT,
				Value: &transaction.Conflicts{Hash: mainTx.Hash()},
			},
		}, req.FallbackTransaction.Attributes)
		require.Equal(t, []transaction.Signer{
			{Account: chain.GetNotaryContractScriptHash()},
			{Account: acc.PrivateKey().GetScriptHash()},
		}, req.FallbackTransaction.Signers)

		// it shouldn't be an error to add completed fallback to the chain
		w, err := wallet.NewWalletFromFile(notaryPath)
		require.NoError(t, err)
		ntr := w.Accounts[0]
		err = ntr.Decrypt(notaryPass, w.Scrypt)
		require.NoError(t, err)
		req.FallbackTransaction.Scripts[0] = transaction.Witness{
			InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, ntr.PrivateKey().SignHashable(uint32(testchain.Network()), req.FallbackTransaction)...),
			VerificationScript: []byte{},
		}
		b := testchain.NewBlock(t, chain, 1, 0, req.FallbackTransaction)
		require.NoError(t, chain.AddBlock(b))
		appLogs, err := chain.GetAppExecResults(req.FallbackTransaction.Hash(), trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 1, len(appLogs))
		appLog := appLogs[0]
		require.Equal(t, vmstate.Halt, appLog.VMState)
		require.Equal(t, appLog.GasConsumed, req.FallbackTransaction.SystemFee)
	})
}

func TestCalculateNotaryFee(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)

	t.Run("client not initialized", func(t *testing.T) {
		_, err := c.CalculateNotaryFee(0) //nolint:staticcheck // SA1019: c.CalculateNotaryFee is deprecated
		require.NoError(t, err)           // Do not require client initialisation for this.
	})
}

func TestPing(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	require.NoError(t, c.Ping())
	rpcSrv.Shutdown()
	httpSrv.Close()
	require.Error(t, c.Ping())
}

func TestCreateTxFromScript(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	priv := testchain.PrivateKey(0)
	acc := wallet.NewAccountFromPrivateKey(priv)
	t.Run("NoSystemFee", func(t *testing.T) {
		tx, err := c.CreateTxFromScript([]byte{byte(opcode.PUSH1)}, acc, -1, 10, nil) //nolint:staticcheck // SA1019: c.CreateTxFromScript is deprecated
		require.NoError(t, err)
		require.True(t, tx.ValidUntilBlock > chain.BlockHeight())
		require.EqualValues(t, 30, tx.SystemFee) // PUSH1
		require.True(t, len(tx.Signers) == 1)
		require.Equal(t, acc.PrivateKey().GetScriptHash(), tx.Signers[0].Account)
	})
	t.Run("ProvideSystemFee", func(t *testing.T) {
		tx, err := c.CreateTxFromScript([]byte{byte(opcode.PUSH1)}, acc, 123, 10, nil) //nolint:staticcheck // SA1019: c.CreateTxFromScript is deprecated
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

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	priv := testchain.PrivateKeyByID(0)
	acc := wallet.NewAccountFromPrivateKey(priv)
	addr := priv.PublicKey().GetScriptHash()

	t.Run("default scope", func(t *testing.T) {
		act, err := actor.NewSimple(c, acc)
		require.NoError(t, err)
		gasprom := gas.New(act)
		tx, err := gasprom.TransferUnsigned(addr, util.Uint160{}, big.NewInt(1000), nil)
		require.NoError(t, err)
		require.NoError(t, acc.SignTx(testchain.Network(), tx))
		require.NoError(t, chain.VerifyTx(tx))
		ic, err := chain.GetTestVM(trigger.Application, tx, nil)
		require.NoError(t, err)
		ic.VM.LoadScriptWithFlags(tx.Script, callflag.All)
		require.NoError(t, ic.VM.Run())
	})
	t.Run("default scope, multitransfer", func(t *testing.T) {
		act, err := actor.NewSimple(c, acc)
		require.NoError(t, err)
		gazprom := gas.New(act)
		tx, err := gazprom.MultiTransferTransaction([]nep17.TransferParameters{
			{From: addr, To: util.Uint160{3, 2, 1}, Amount: big.NewInt(1000), Data: nil},
			{From: addr, To: util.Uint160{1, 2, 3}, Amount: big.NewInt(1000), Data: nil},
		})
		require.NoError(t, err)
		require.NoError(t, chain.VerifyTx(tx))
		ic, err := chain.GetTestVM(trigger.Application, tx, nil)
		require.NoError(t, err)
		ic.VM.LoadScriptWithFlags(tx.Script, callflag.All)
		require.NoError(t, ic.VM.Run())
		require.Equal(t, 2, len(ic.Notifications))
	})
	t.Run("none scope", func(t *testing.T) {
		act, err := actor.New(c, []actor.SignerAccount{{
			Signer: transaction.Signer{
				Account: addr,
				Scopes:  transaction.None,
			},
			Account: acc,
		}})
		require.NoError(t, err)
		gasprom := gas.New(act)
		_, err = gasprom.TransferUnsigned(addr, util.Uint160{}, big.NewInt(1000), nil)
		require.Error(t, err)
	})
	t.Run("customcontracts scope", func(t *testing.T) {
		act, err := actor.New(c, []actor.SignerAccount{{
			Signer: transaction.Signer{
				Account:          priv.PublicKey().GetScriptHash(),
				Scopes:           transaction.CustomContracts,
				AllowedContracts: []util.Uint160{gas.Hash},
			},
			Account: acc,
		}})
		require.NoError(t, err)
		gasprom := gas.New(act)
		tx, err := gasprom.TransferUnsigned(addr, util.Uint160{}, big.NewInt(1000), nil)
		require.NoError(t, err)
		require.NoError(t, acc.SignTx(testchain.Network(), tx))
		require.NoError(t, chain.VerifyTx(tx))
		ic, err := chain.GetTestVM(trigger.Application, tx, nil)
		require.NoError(t, err)
		ic.VM.LoadScriptWithFlags(tx.Script, callflag.All)
		require.NoError(t, ic.VM.Run())
	})
}

func TestInvokeVerify(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	contract, err := util.Uint160DecodeStringLE(verifyContractHash)
	require.NoError(t, err)

	t.Run("positive, with signer", func(t *testing.T) {
		res, err := c.InvokeContractVerify(contract, []smartcontract.Parameter{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}})
		require.NoError(t, err)
		require.Equal(t, "HALT", res.State)
		require.Equal(t, 1, len(res.Stack))
		require.True(t, res.Stack[0].Value().(bool))
	})

	t.Run("positive, historic, by height, with signer", func(t *testing.T) {
		h := chain.BlockHeight() - 1
		res, err := c.InvokeContractVerifyAtHeight(h, contract, []smartcontract.Parameter{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}})
		require.NoError(t, err)
		require.Equal(t, "HALT", res.State)
		require.Equal(t, 1, len(res.Stack))
		require.True(t, res.Stack[0].Value().(bool))
	})

	t.Run("positive, historic, by block, with signer", func(t *testing.T) {
		res, err := c.InvokeContractVerifyWithState(chain.GetHeaderHash(chain.BlockHeight()-1), contract, []smartcontract.Parameter{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}})
		require.NoError(t, err)
		require.Equal(t, "HALT", res.State)
		require.Equal(t, 1, len(res.Stack))
		require.True(t, res.Stack[0].Value().(bool))
	})

	t.Run("positive, historic, by stateroot, with signer", func(t *testing.T) {
		h := chain.BlockHeight() - 1
		sr, err := chain.GetStateModule().GetStateRoot(h)
		require.NoError(t, err)
		res, err := c.InvokeContractVerifyWithState(sr.Root, contract, []smartcontract.Parameter{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}})
		require.NoError(t, err)
		require.Equal(t, "HALT", res.State)
		require.Equal(t, 1, len(res.Stack))
		require.True(t, res.Stack[0].Value().(bool))
	})

	t.Run("bad, historic, by hash: contract not found", func(t *testing.T) {
		var h uint32 = 1
		_, err = c.InvokeContractVerifyAtHeight(h, contract, []smartcontract.Parameter{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}})
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), core.ErrUnknownVerificationContract.Error())) // contract wasn't deployed at block #1 yet
	})

	t.Run("bad, historic, by block: contract not found", func(t *testing.T) {
		_, err = c.InvokeContractVerifyWithState(chain.GetHeaderHash(1), contract, []smartcontract.Parameter{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}})
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), core.ErrUnknownVerificationContract.Error())) // contract wasn't deployed at block #1 yet
	})

	t.Run("bad, historic, by stateroot: contract not found", func(t *testing.T) {
		var h uint32 = 1
		sr, err := chain.GetStateModule().GetStateRoot(h)
		require.NoError(t, err)
		_, err = c.InvokeContractVerifyWithState(sr.Root, contract, []smartcontract.Parameter{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}})
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), core.ErrUnknownVerificationContract.Error())) // contract wasn't deployed at block #1 yet
	})

	t.Run("positive, with signer and witness", func(t *testing.T) {
		res, err := c.InvokeContractVerify(contract, []smartcontract.Parameter{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}}, transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH1), byte(opcode.RET)}})
		require.NoError(t, err)
		require.Equal(t, "HALT", res.State)
		require.Equal(t, 1, len(res.Stack))
		require.True(t, res.Stack[0].Value().(bool))
	})

	t.Run("error, invalid witness number", func(t *testing.T) {
		_, err := c.InvokeContractVerify(contract, []smartcontract.Parameter{}, []transaction.Signer{{Account: testchain.PrivateKeyByID(0).PublicKey().GetScriptHash()}}, transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH1), byte(opcode.RET)}}, transaction.Witness{InvocationScript: []byte{byte(opcode.RET)}})
		require.Error(t, err)
	})

	t.Run("false", func(t *testing.T) {
		res, err := c.InvokeContractVerify(contract, []smartcontract.Parameter{}, []transaction.Signer{{Account: util.Uint160{}}})
		require.NoError(t, err)
		require.Equal(t, "HALT", res.State)
		require.Equal(t, 1, len(res.Stack))
		require.False(t, res.Stack[0].Value().(bool))
	})
}

func TestClient_GetNativeContracts(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	cs, err := c.GetNativeContracts()
	require.NoError(t, err)
	require.Equal(t, chain.GetNatives(), cs)
}

func TestClient_NEP11_ND(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	h, err := util.Uint160DecodeStringLE(nnsContractHash)
	require.NoError(t, err)
	priv0 := testchain.PrivateKeyByID(0)
	act, err := actor.NewSimple(c, wallet.NewAccountFromPrivateKey(priv0))
	require.NoError(t, err)
	n11 := nep11.NewNonDivisible(act, h)
	acc := priv0.GetScriptHash()

	t.Run("Decimals", func(t *testing.T) {
		d, err := n11.Decimals()
		require.NoError(t, err)
		require.EqualValues(t, 0, d) // non-divisible
	})
	t.Run("TotalSupply", func(t *testing.T) {
		s, err := n11.TotalSupply()
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(1), s) // the only `neo.com` of acc0
	})
	t.Run("Symbol", func(t *testing.T) {
		sym, err := n11.Symbol()
		require.NoError(t, err)
		require.Equal(t, "NNS", sym)
	})
	t.Run("TokenInfo", func(t *testing.T) {
		tok, err := neptoken.Info(c, h)
		require.NoError(t, err)
		require.Equal(t, &wallet.Token{
			Name:     "NameService",
			Hash:     h,
			Decimals: 0,
			Symbol:   "NNS",
			Standard: manifest.NEP11StandardName,
		}, tok)
	})
	t.Run("BalanceOf", func(t *testing.T) {
		b, err := n11.BalanceOf(acc)
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(1), b)
	})
	t.Run("OwnerOf", func(t *testing.T) {
		b, err := n11.OwnerOf([]byte("neo.com"))
		require.NoError(t, err)
		require.EqualValues(t, acc, b)
	})
	t.Run("Tokens", func(t *testing.T) {
		iter, err := n11.Tokens()
		require.NoError(t, err)
		items, err := iter.Next(config.DefaultMaxIteratorResultItems)
		require.NoError(t, err)
		require.Equal(t, 1, len(items))
		require.Equal(t, [][]byte{[]byte("neo.com")}, items)
		require.NoError(t, iter.Terminate())
	})
	t.Run("TokensExpanded", func(t *testing.T) {
		items, err := n11.TokensExpanded(config.DefaultMaxIteratorResultItems)
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("neo.com")}, items)
	})
	t.Run("Properties", func(t *testing.T) {
		p, err := n11.Properties([]byte("neo.com"))
		require.NoError(t, err)
		blockRegisterDomain, err := chain.GetBlock(chain.GetHeaderHash(14)) // `neo.com` domain was registered in 14th block
		require.NoError(t, err)
		require.Equal(t, 1, len(blockRegisterDomain.Transactions))
		expected := stackitem.NewMap()
		expected.Add(stackitem.Make([]byte("name")), stackitem.Make([]byte("neo.com")))
		expected.Add(stackitem.Make([]byte("expiration")), stackitem.Make(blockRegisterDomain.Timestamp+365*24*3600*1000)) // expiration formula
		expected.Add(stackitem.Make([]byte("admin")), stackitem.Null{})
		require.EqualValues(t, expected, p)
	})
	t.Run("Transfer", func(t *testing.T) {
		_, _, err := n11.Transfer(testchain.PrivateKeyByID(1).GetScriptHash(), []byte("neo.com"), nil)
		require.NoError(t, err)
	})
}

func TestClient_NEP11_D(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	pkey0 := testchain.PrivateKeyByID(0)
	priv0 := pkey0.GetScriptHash()
	priv1 := testchain.PrivateKeyByID(1).GetScriptHash()
	token1ID, err := hex.DecodeString(nfsoToken1ID)
	require.NoError(t, err)

	act, err := actor.NewSimple(c, wallet.NewAccountFromPrivateKey(pkey0))
	require.NoError(t, err)
	n11 := nep11.NewDivisible(act, nfsoHash)

	t.Run("Decimals", func(t *testing.T) {
		d, err := n11.Decimals()
		require.NoError(t, err)
		require.EqualValues(t, 2, d) // Divisible.
	})
	t.Run("TotalSupply", func(t *testing.T) {
		s, err := n11.TotalSupply()
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(1), s) // the only NFSO of acc0
	})
	t.Run("Symbol", func(t *testing.T) {
		sym, err := n11.Symbol()
		require.NoError(t, err)
		require.Equal(t, "NFSO", sym)
	})
	t.Run("TokenInfo", func(t *testing.T) {
		tok, err := neptoken.Info(c, nfsoHash)
		require.NoError(t, err)
		require.Equal(t, &wallet.Token{
			Name:     "NeoFS Object NFT",
			Hash:     nfsoHash,
			Decimals: 2,
			Symbol:   "NFSO",
			Standard: manifest.NEP11StandardName,
		}, tok)
	})
	t.Run("BalanceOf", func(t *testing.T) {
		b, err := n11.BalanceOf(priv0)
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(80), b)
	})
	t.Run("BalanceOfD", func(t *testing.T) {
		b, err := n11.BalanceOfD(priv0, token1ID)
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(80), b)
	})
	t.Run("OwnerOf", func(t *testing.T) {
		iter, err := n11.OwnerOf(token1ID)
		require.NoError(t, err)
		items, err := iter.Next(config.DefaultMaxIteratorResultItems)
		require.NoError(t, err)
		require.Equal(t, 2, len(items))
		require.Equal(t, []util.Uint160{priv1, priv0}, items)
		require.NoError(t, iter.Terminate())
	})
	t.Run("OwnerOfExpanded", func(t *testing.T) {
		b, err := n11.OwnerOfExpanded(token1ID, config.DefaultMaxIteratorResultItems)
		require.NoError(t, err)
		require.Equal(t, []util.Uint160{priv1, priv0}, b)
	})
	t.Run("Properties", func(t *testing.T) {
		p, err := n11.Properties(token1ID)
		require.NoError(t, err)
		expected := stackitem.NewMap()
		expected.Add(stackitem.Make([]byte("name")), stackitem.NewBuffer([]byte("NeoFS Object "+base64.StdEncoding.EncodeToString(token1ID))))
		expected.Add(stackitem.Make([]byte("containerID")), stackitem.Make([]byte(base64.StdEncoding.EncodeToString(nfsoToken1ContainerID.BytesBE()))))
		expected.Add(stackitem.Make([]byte("objectID")), stackitem.Make([]byte(base64.StdEncoding.EncodeToString(nfsoToken1ObjectID.BytesBE()))))
		require.EqualValues(t, expected, p)
	})
	t.Run("Transfer", func(t *testing.T) {
		_, _, err := n11.TransferD(priv0, priv1, big.NewInt(20), token1ID, nil)
		require.NoError(t, err)
	})
}

func TestClient_NNS(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())
	nnc := nns.NewReader(invoker.New(c, nil), nnsHash)

	t.Run("IsAvailable, false", func(t *testing.T) {
		b, err := nnc.IsAvailable("neo.com")
		require.NoError(t, err)
		require.Equal(t, false, b)
	})
	t.Run("IsAvailable, true", func(t *testing.T) {
		b, err := nnc.IsAvailable("neogo.com")
		require.NoError(t, err)
		require.Equal(t, true, b)
	})
	t.Run("Resolve, good", func(t *testing.T) {
		b, err := nnc.Resolve("neo.com", nns.A)
		require.NoError(t, err)
		require.Equal(t, "1.2.3.4", b)
	})
	t.Run("Resolve, bad", func(t *testing.T) {
		_, err := nnc.Resolve("neogo.com", nns.A)
		require.Error(t, err)
	})
	t.Run("Resolve, CNAME", func(t *testing.T) {
		_, err := nnc.Resolve("neogo.com", nns.CNAME)
		require.Error(t, err)
	})
	t.Run("GetAllRecords, good", func(t *testing.T) {
		iter, err := nnc.GetAllRecords("neo.com")
		require.NoError(t, err)
		arr, err := iter.Next(config.DefaultMaxIteratorResultItems)
		require.NoError(t, err)
		require.Equal(t, 1, len(arr))
		require.Equal(t, nns.RecordState{
			Name: "neo.com",
			Type: nns.A,
			Data: "1.2.3.4",
		}, arr[0])
	})
	t.Run("GetAllRecordsExpanded, good", func(t *testing.T) {
		rss, err := nnc.GetAllRecordsExpanded("neo.com", 42)
		require.NoError(t, err)
		require.Equal(t, []nns.RecordState{
			{
				Name: "neo.com",
				Type: nns.A,
				Data: "1.2.3.4",
			},
		}, rss)
	})
	t.Run("GetAllRecords, bad", func(t *testing.T) {
		_, err := nnc.GetAllRecords("neopython.com")
		require.Error(t, err)
	})
	t.Run("GetAllRecordsExpanded, bad", func(t *testing.T) {
		_, err := nnc.GetAllRecordsExpanded("neopython.com", 7)
		require.Error(t, err)
	})
}

func TestClient_IteratorSessions(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	storageHash, err := util.Uint160DecodeStringLE(storageContractHash)
	require.NoError(t, err)

	// storageItemsCount is the amount of storage items stored in Storage contract, it's hard-coded in the contract code.
	const storageItemsCount = 255
	expected := make([][]byte, storageItemsCount)
	for i := 0; i < storageItemsCount; i++ {
		expected[i] = stackitem.NewBigInteger(big.NewInt(int64(i))).Bytes()
	}
	sort.Slice(expected, func(i, j int) bool {
		if len(expected[i]) != len(expected[j]) {
			return len(expected[i]) < len(expected[j])
		}
		return bytes.Compare(expected[i], expected[j]) < 0
	})

	prepareSession := func(t *testing.T) (uuid.UUID, uuid.UUID) {
		res, err := c.InvokeFunction(storageHash, "iterateOverValues", []smartcontract.Parameter{}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, res.Session)
		require.Equal(t, 1, len(res.Stack))
		require.Equal(t, stackitem.InteropT, res.Stack[0].Type())
		iterator, ok := res.Stack[0].Value().(result.Iterator)
		require.True(t, ok)
		require.NotEmpty(t, iterator.ID)
		return res.Session, *iterator.ID
	}
	t.Run("traverse with max constraint", func(t *testing.T) {
		sID, iID := prepareSession(t)
		check := func(t *testing.T, start, end int) {
			max := end - start
			set, err := c.TraverseIterator(sID, iID, max)
			require.NoError(t, err)
			require.Equal(t, max, len(set))
			for i := 0; i < max; i++ {
				// According to the Storage contract code.
				require.Equal(t, expected[start+i], set[i].Value().([]byte), start+i)
			}
		}
		check(t, 0, 30)
		check(t, 30, 48)
		check(t, 48, 49)
		check(t, 49, 49+config.DefaultMaxIteratorResultItems)
		check(t, 49+config.DefaultMaxIteratorResultItems, 49+2*config.DefaultMaxIteratorResultItems-1)
		check(t, 49+2*config.DefaultMaxIteratorResultItems-1, 255)

		// Iterator ends on 255-th element, so no more elements should be returned.
		set, err := c.TraverseIterator(sID, iID, config.DefaultMaxIteratorResultItems)
		require.NoError(t, err)
		require.Equal(t, 0, len(set))
	})

	t.Run("traverse, request more than exists", func(t *testing.T) {
		sID, iID := prepareSession(t)
		for i := 0; i < storageItemsCount/config.DefaultMaxIteratorResultItems; i++ {
			set, err := c.TraverseIterator(sID, iID, config.DefaultMaxIteratorResultItems)
			require.NoError(t, err)
			require.Equal(t, config.DefaultMaxIteratorResultItems, len(set))
		}

		// Request more items than left untraversed.
		set, err := c.TraverseIterator(sID, iID, config.DefaultMaxIteratorResultItems)
		require.NoError(t, err)
		require.Equal(t, storageItemsCount%config.DefaultMaxIteratorResultItems, len(set))
	})

	t.Run("traverse, no max constraint", func(t *testing.T) {
		sID, iID := prepareSession(t)

		set, err := c.TraverseIterator(sID, iID, -1)
		require.NoError(t, err)
		require.Equal(t, config.DefaultMaxIteratorResultItems, len(set))
	})

	t.Run("traverse, concurrent access", func(t *testing.T) {
		sID, iID := prepareSession(t)
		wg := sync.WaitGroup{}
		wg.Add(storageItemsCount)
		check := func(t *testing.T) {
			set, err := c.TraverseIterator(sID, iID, 1)
			require.NoError(t, err)
			require.Equal(t, 1, len(set))
			wg.Done()
		}
		for i := 0; i < storageItemsCount; i++ {
			go check(t)
		}
		wg.Wait()
	})

	t.Run("terminate session", func(t *testing.T) {
		t.Run("manually", func(t *testing.T) {
			sID, iID := prepareSession(t)

			// Check session is created.
			set, err := c.TraverseIterator(sID, iID, 1)
			require.NoError(t, err)
			require.Equal(t, 1, len(set))

			ok, err := c.TerminateSession(sID)
			require.NoError(t, err)
			require.True(t, ok)

			ok, err = c.TerminateSession(sID)
			require.NoError(t, err)
			require.False(t, ok) // session has already been terminated.
		})
		t.Run("automatically", func(t *testing.T) {
			sID, iID := prepareSession(t)

			// Check session is created.
			set, err := c.TraverseIterator(sID, iID, 1)
			require.NoError(t, err)
			require.Equal(t, 1, len(set))

			require.Eventually(t, func() bool {
				rpcSrv.sessionsLock.Lock()
				defer rpcSrv.sessionsLock.Unlock()

				_, ok := rpcSrv.sessions[sID.String()]
				return !ok
			}, time.Duration(rpcSrv.config.SessionExpirationTime)*time.Second*3,
				// Sessions list is updated once per SessionExpirationTime, thus, no need to ask for update more frequently than
				// sessions cleaning occurs.
				time.Duration(rpcSrv.config.SessionExpirationTime)*time.Second/4)

			ok, err := c.TerminateSession(sID)
			require.NoError(t, err)
			require.False(t, ok) // session has already been terminated.
		})
	})
}

func TestClient_GetNotaryServiceFeePerKey(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	var defaultNotaryServiceFeePerKey int64 = 1000_0000
	actual, err := c.GetNotaryServiceFeePerKey() //nolint:staticcheck // SA1019: c.GetNotaryServiceFeePerKey is deprecated
	require.NoError(t, err)
	require.Equal(t, defaultNotaryServiceFeePerKey, actual)
}

func TestClient_States(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	stateheight, err := c.GetStateHeight()
	assert.NoError(t, err)
	assert.Equal(t, chain.BlockHeight(), stateheight.Local)

	stateroot, err := c.GetStateRootByHeight(stateheight.Local)
	assert.NoError(t, err)

	t.Run("proof", func(t *testing.T) {
		policy, err := chain.GetNativeContractScriptHash(nativenames.Policy)
		assert.NoError(t, err)
		proof, err := c.GetProof(stateroot.Root, policy, []byte{19}) // storagePrice key in policy contract
		assert.NoError(t, err)
		value, err := c.VerifyProof(stateroot.Root, proof)
		assert.NoError(t, err)
		assert.Equal(t, big.NewInt(native.DefaultStoragePrice), bigint.FromBytes(value))
	})
}

func TestClientOracle(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	oraRe := oracle.NewReader(invoker.New(c, nil))

	var defaultOracleRequestPrice = big.NewInt(5000_0000)
	actual, err := oraRe.GetPrice()
	require.NoError(t, err)
	require.Equal(t, defaultOracleRequestPrice, actual)

	act, err := actor.New(c, []actor.SignerAccount{{
		Signer: transaction.Signer{
			Account: testchain.CommitteeScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: &wallet.Account{
			Address: testchain.CommitteeAddress(),
			Contract: &wallet.Contract{
				Script: testchain.CommitteeVerificationScript(),
			},
		},
	}})
	require.NoError(t, err)

	ora := oracle.New(act)

	newPrice := big.NewInt(1_0000_0000)
	tx, err := ora.SetPriceUnsigned(newPrice)
	require.NoError(t, err)

	tx.Scripts[0].InvocationScript = testchain.SignCommittee(tx)
	bl := testchain.NewBlock(t, chain, 1, 0, tx)
	_, err = c.SubmitBlock(*bl)
	require.NoError(t, err)

	actual, err = ora.GetPrice()
	require.NoError(t, err)
	require.Equal(t, newPrice, actual)
}

func TestClient_InvokeAndPackIteratorResults(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	require.NoError(t, c.Init())

	// storageItemsCount is the amount of storage items stored in Storage contract, it's hard-coded in the contract code.
	const storageItemsCount = 255
	expected := make([][]byte, storageItemsCount)
	for i := 0; i < storageItemsCount; i++ {
		expected[i] = stackitem.NewBigInteger(big.NewInt(int64(i))).Bytes()
	}
	sort.Slice(expected, func(i, j int) bool {
		if len(expected[i]) != len(expected[j]) {
			return len(expected[i]) < len(expected[j])
		}
		return bytes.Compare(expected[i], expected[j]) < 0
	})
	storageHash, err := util.Uint160DecodeStringLE(storageContractHash)
	require.NoError(t, err)

	t.Run("default max items constraint", func(t *testing.T) {
		res, err := c.InvokeAndPackIteratorResults(storageHash, "iterateOverValues", []smartcontract.Parameter{}, nil) //nolint:staticcheck // SA1019: c.InvokeAndPackIteratorResults is deprecated
		require.NoError(t, err)
		require.Equal(t, vmstate.Halt.String(), res.State)
		require.Equal(t, 1, len(res.Stack))
		require.Equal(t, stackitem.ArrayT, res.Stack[0].Type())
		arr, ok := res.Stack[0].Value().([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, config.DefaultMaxIteratorResultItems, len(arr))

		for i := range arr {
			require.Equal(t, stackitem.ByteArrayT, arr[i].Type())
			require.Equal(t, expected[i], arr[i].Value().([]byte))
		}
	})
	t.Run("custom max items constraint", func(t *testing.T) {
		max := 123
		res, err := c.InvokeAndPackIteratorResults(storageHash, "iterateOverValues", []smartcontract.Parameter{}, nil, max) //nolint:staticcheck // SA1019: c.InvokeAndPackIteratorResults is deprecated
		require.NoError(t, err)
		require.Equal(t, vmstate.Halt.String(), res.State)
		require.Equal(t, 1, len(res.Stack))
		require.Equal(t, stackitem.ArrayT, res.Stack[0].Type())
		arr, ok := res.Stack[0].Value().([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, max, len(arr))

		for i := range arr {
			require.Equal(t, stackitem.ByteArrayT, arr[i].Type())
			require.Equal(t, expected[i], arr[i].Value().([]byte))
		}
	})
}

func TestClient_Iterator_SessionConfigVariations(t *testing.T) {
	var expected [][]byte
	storageHash, err := util.Uint160DecodeStringLE(storageContractHash)
	require.NoError(t, err)
	// storageItemsCount is the amount of storage items stored in Storage contract, it's hard-coded in the contract code.
	const storageItemsCount = 255

	checkSessionEnabled := func(t *testing.T, c *rpcclient.Client) {
		// We expect Iterator with designated ID to be presented on stack. It should be possible to retrieve its values via `traverseiterator` call.
		res, err := c.InvokeFunction(storageHash, "iterateOverValues", []smartcontract.Parameter{}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, res.Session)
		require.Equal(t, 1, len(res.Stack))
		require.Equal(t, stackitem.InteropT, res.Stack[0].Type())
		iterator, ok := res.Stack[0].Value().(result.Iterator)
		require.True(t, ok)
		require.NotEmpty(t, iterator.ID)
		require.Empty(t, iterator.Values)
		max := 84
		actual, err := c.TraverseIterator(res.Session, *iterator.ID, max)
		require.NoError(t, err)
		require.Equal(t, max, len(actual))
		for i := 0; i < max; i++ {
			// According to the Storage contract code.
			require.Equal(t, expected[i], actual[i].Value().([]byte), i)
		}
	}
	t.Run("default sessions enabled", func(t *testing.T) {
		chain, rpcSrv, httpSrv := initClearServerWithServices(t, false, false, false)
		defer chain.Close()
		defer rpcSrv.Shutdown()
		for _, b := range getTestBlocks(t) {
			require.NoError(t, chain.AddBlock(b))
		}

		c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
		require.NoError(t, err)
		require.NoError(t, c.Init())

		// Fill in expected stackitems set during the first test.
		expected = make([][]byte, storageItemsCount)
		for i := 0; i < storageItemsCount; i++ {
			expected[i] = stackitem.NewBigInteger(big.NewInt(int64(i))).Bytes()
		}
		sort.Slice(expected, func(i, j int) bool {
			if len(expected[i]) != len(expected[j]) {
				return len(expected[i]) < len(expected[j])
			}
			return bytes.Compare(expected[i], expected[j]) < 0
		})
		checkSessionEnabled(t, c)
	})
	t.Run("MPT-based sessions enables", func(t *testing.T) {
		// Prepare MPT-enabled RPC server.
		chain, orc, cfg, logger := getUnitTestChainWithCustomConfig(t, false, false, func(cfg *config.Config) {
			cfg.ApplicationConfiguration.RPC.SessionEnabled = true
			cfg.ApplicationConfiguration.RPC.SessionBackedByMPT = true
		})
		serverConfig, err := network.NewServerConfig(cfg)
		require.NoError(t, err)
		serverConfig.UserAgent = fmt.Sprintf(config.UserAgentFormat, "0.98.6-test")
		serverConfig.Addresses = []config.AnnounceableAddress{{Address: ":0"}}
		server, err := network.NewServer(serverConfig, chain, chain.GetStateSyncModule(), logger)
		require.NoError(t, err)
		errCh := make(chan error, 2)
		rpcSrv := New(chain, cfg.ApplicationConfiguration.RPC, server, orc, logger, errCh)
		rpcSrv.Start()
		handler := http.HandlerFunc(rpcSrv.handleHTTPRequest)
		httpSrv := httptest.NewServer(handler)
		defer chain.Close()
		defer rpcSrv.Shutdown()
		for _, b := range getTestBlocks(t) {
			require.NoError(t, chain.AddBlock(b))
		}

		c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
		require.NoError(t, err)
		require.NoError(t, c.Init())

		checkSessionEnabled(t, c)
	})
	t.Run("sessions disabled", func(t *testing.T) {
		chain, rpcSrv, httpSrv := initClearServerWithServices(t, false, false, true)
		defer chain.Close()
		defer rpcSrv.Shutdown()
		for _, b := range getTestBlocks(t) {
			require.NoError(t, chain.AddBlock(b))
		}

		c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
		require.NoError(t, err)
		require.NoError(t, c.Init())

		// We expect unpacked iterator values to be present on stack under InteropInterface cover.
		res, err := c.InvokeFunction(storageHash, "iterateOverValues", []smartcontract.Parameter{}, nil)
		require.NoError(t, err)
		require.Empty(t, res.Session)
		require.Equal(t, 1, len(res.Stack))
		require.Equal(t, stackitem.InteropT, res.Stack[0].Type())
		iterator, ok := res.Stack[0].Value().(result.Iterator)
		require.True(t, ok)
		require.Empty(t, iterator.ID)
		require.NotEmpty(t, iterator.Values)
		require.True(t, iterator.Truncated)
		require.Equal(t, rpcSrv.config.MaxIteratorResultItems, len(iterator.Values))
		for i := 0; i < rpcSrv.config.MaxIteratorResultItems; i++ {
			// According to the Storage contract code.
			require.Equal(t, expected[i], iterator.Values[i].Value().([]byte), i)
		}
	})
}

func TestClient_Wait(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	acc, err := wallet.NewAccount()
	require.NoError(t, err)
	act, err := actor.New(c, []actor.SignerAccount{
		{
			Signer: transaction.Signer{
				Account: acc.ScriptHash(),
			},
			Account: acc,
		},
	})
	require.NoError(t, err)

	b, err := chain.GetBlock(chain.GetHeaderHash(1))
	require.NoError(t, err)
	require.True(t, len(b.Transactions) > 0)

	check := func(t *testing.T, h util.Uint256, vub uint32, errExpected bool) {
		rcvr := make(chan struct{})
		go func() {
			aer, err := act.Wait(h, vub, nil)
			if errExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, h, aer.Container)
			}
			rcvr <- struct{}{}
		}()
	waitloop:
		for {
			select {
			case <-rcvr:
				break waitloop
			case <-time.NewTimer(chain.GetConfig().TimePerBlock).C:
				t.Fatal("transaction failed to be awaited")
			}
		}
	}

	// Wait for transaction that has been persisted and VUB block has been persisted.
	check(t, b.Transactions[0].Hash(), chain.BlockHeight()-1, false)
	// Wait for transaction that has been persisted and VUB block hasn't yet been persisted.
	check(t, b.Transactions[0].Hash(), chain.BlockHeight()+1, false)
	// Wait for transaction that hasn't been persisted and VUB block has been persisted.
	check(t, util.Uint256{1, 2, 3}, chain.BlockHeight()-1, true)
}

func mkSubsClient(t *testing.T, rpcSrv *Server, httpSrv *httptest.Server, local bool) *rpcclient.WSClient {
	var (
		c   *rpcclient.WSClient
		err error
		icl *rpcclient.Internal
	)
	if local {
		icl, err = rpcclient.NewInternal(context.Background(), rpcSrv.RegisterLocal)
	} else {
		url := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"
		c, err = rpcclient.NewWS(context.Background(), url, rpcclient.Options{})
	}
	require.NoError(t, err)
	if local {
		c = &icl.WSClient
	}
	require.NoError(t, c.Init())
	return c
}

func runWSAndLocal(t *testing.T, test func(*testing.T, bool)) {
	t.Run("ws", func(t *testing.T) {
		test(t, false)
	})
	t.Run("local", func(t *testing.T) {
		test(t, true)
	})
}

func TestSubClientWait(t *testing.T) {
	runWSAndLocal(t, testSubClientWait)
}

func testSubClientWait(t *testing.T, local bool) {
	chain, rpcSrv, httpSrv := initClearServerWithServices(t, false, false, true)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c := mkSubsClient(t, rpcSrv, httpSrv, local)
	acc, err := wallet.NewAccount()
	require.NoError(t, err)
	act, err := actor.New(c, []actor.SignerAccount{
		{
			Signer: transaction.Signer{
				Account: acc.ScriptHash(),
			},
			Account: acc,
		},
	})
	require.NoError(t, err)

	rcvr := make(chan *state.AppExecResult)
	check := func(t *testing.T, b *block.Block, h util.Uint256, vub uint32) {
		go func() {
			aer, err := act.Wait(h, vub, nil)
			require.NoError(t, err, b.Index)
			rcvr <- aer
		}()
		go func() {
			// Wait until client is properly subscribed. The real node won't behave like this,
			// but the real node has the subsequent blocks to be added that will trigger client's
			// waitloops to finish anyway (and the test has only single block, thus, use it careful).
			require.Eventually(t, func() bool {
				rpcSrv.subsLock.Lock()
				defer rpcSrv.subsLock.Unlock()
				if len(rpcSrv.subscribers) == 1 { // single client
					for s := range rpcSrv.subscribers {
						var count int
						for _, f := range s.feeds {
							if f.event != neorpc.InvalidEventID {
								count++
							}
						}
						return count == 2 // subscription for blocks + AERs
					}
				}
				return false
			}, time.Second, 100*time.Millisecond)
			require.NoError(t, chain.AddBlock(b))
		}()
	waitloop:
		for {
			select {
			case aer := <-rcvr:
				require.Equal(t, h, aer.Container)
				require.Equal(t, trigger.Application, aer.Trigger)
				if h.StringLE() == faultedTxHashLE {
					require.Equal(t, vmstate.Fault, aer.VMState)
				} else {
					require.Equal(t, vmstate.Halt, aer.VMState)
				}
				break waitloop
			case <-time.NewTimer(chain.GetConfig().TimePerBlock).C:
				t.Fatalf("transaction from block %d failed to be awaited: deadline exceeded", b.Index)
			}
		}
		// Wait for server/client to properly unsubscribe. In real life subsequent awaiter
		// requests may be run concurrently, and it's OK, but it's important for the test
		// not to run subscription requests in parallel because block addition is bounded to
		// the number of subscribers.
		require.Eventually(t, func() bool {
			rpcSrv.subsLock.Lock()
			defer rpcSrv.subsLock.Unlock()
			if len(rpcSrv.subscribers) != 1 {
				return false
			}
			for s := range rpcSrv.subscribers {
				for _, f := range s.feeds {
					if f.event != neorpc.InvalidEventID {
						return false
					}
				}
			}
			return true
		}, time.Second, 100*time.Millisecond)
	}

	var faultedChecked bool
	for _, b := range getTestBlocks(t) {
		if len(b.Transactions) > 0 {
			tx := b.Transactions[0]
			check(t, b, tx.Hash(), tx.ValidUntilBlock)
			if tx.Hash().StringLE() == faultedTxHashLE {
				faultedChecked = true
			}
		} else {
			require.NoError(t, chain.AddBlock(b))
		}
	}
	require.True(t, faultedChecked, "FAULTed transaction wasn't checked")
}

func TestSubClientWaitWithLateSubscription(t *testing.T) {
	runWSAndLocal(t, testSubClientWaitWithLateSubscription)
}

func testSubClientWaitWithLateSubscription(t *testing.T, local bool) {
	chain, rpcSrv, httpSrv := initClearServerWithServices(t, false, false, true)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c := mkSubsClient(t, rpcSrv, httpSrv, local)
	acc, err := wallet.NewAccount()
	require.NoError(t, err)
	act, err := actor.New(c, []actor.SignerAccount{
		{
			Signer: transaction.Signer{
				Account: acc.ScriptHash(),
			},
			Account: acc,
		},
	})
	require.NoError(t, err)

	// Firstly, accept the block.
	blocks := getTestBlocks(t)
	b1 := blocks[0]
	tx := b1.Transactions[0]
	require.NoError(t, chain.AddBlock(b1))

	// After that, wait and get the result immediately.
	aer, err := act.Wait(tx.Hash(), tx.ValidUntilBlock, nil)
	require.NoError(t, err)
	require.Equal(t, tx.Hash(), aer.Container)
	require.Equal(t, trigger.Application, aer.Trigger)
	require.Equal(t, vmstate.Halt, aer.VMState)
}

func TestWSClientHandshakeError(t *testing.T) {
	chain, rpcSrv, httpSrv := initClearServerWithCustomConfig(t, func(cfg *config.Config) {
		cfg.ApplicationConfiguration.RPC.MaxWebSocketClients = -1
	})
	defer chain.Close()
	defer rpcSrv.Shutdown()

	url := "ws" + strings.TrimPrefix(httpSrv.URL, "http") + "/ws"
	_, err := rpcclient.NewWS(context.Background(), url, rpcclient.Options{})
	require.ErrorContains(t, err, "websocket users limit reached")
}

func TestSubClientWaitWithMissedEvent(t *testing.T) {
	runWSAndLocal(t, testSubClientWaitWithMissedEvent)
}

func testSubClientWaitWithMissedEvent(t *testing.T, local bool) {
	chain, rpcSrv, httpSrv := initClearServerWithServices(t, false, false, true)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c := mkSubsClient(t, rpcSrv, httpSrv, local)
	acc, err := wallet.NewAccount()
	require.NoError(t, err)
	act, err := actor.New(c, []actor.SignerAccount{
		{
			Signer: transaction.Signer{
				Account: acc.ScriptHash(),
			},
			Account: acc,
		},
	})
	require.NoError(t, err)

	blocks := getTestBlocks(t)
	b1 := blocks[0]
	tx := b1.Transactions[0]

	rcvr := make(chan *state.AppExecResult)
	go func() {
		aer, err := act.Wait(tx.Hash(), tx.ValidUntilBlock, nil)
		require.NoError(t, err)
		rcvr <- aer
	}()

	// Wait until client is properly subscribed. The real node won't behave like this,
	// but the real node has the subsequent blocks to be added that will trigger client's
	// waitloops to finish anyway (and the test has only single block, thus, use it careful).
	require.Eventually(t, func() bool {
		rpcSrv.subsLock.Lock()
		defer rpcSrv.subsLock.Unlock()
		return len(rpcSrv.subscribers) == 1
	}, time.Second, 100*time.Millisecond)

	rpcSrv.subsLock.Lock()
	// Suppress normal event delivery.
	for s := range rpcSrv.subscribers {
		s.overflown.Store(true)
	}
	rpcSrv.subsLock.Unlock()

	// Accept the next block, but subscriber will get no events because it's overflown.
	require.NoError(t, chain.AddBlock(b1))

	overNotification := neorpc.Notification{
		JSONRPC: neorpc.JSONRPCVersion,
		Event:   neorpc.MissedEventID,
		Payload: make([]any, 0),
	}
	overEvent, err := json.Marshal(overNotification)
	require.NoError(t, err)
	overflowMsg, err := websocket.NewPreparedMessage(websocket.TextMessage, overEvent)
	require.NoError(t, err)
	rpcSrv.subsLock.Lock()
	// Deliver overflow message -> triggers subscriber to retry with polling waiter.
	for s := range rpcSrv.subscribers {
		s.writer <- intEvent{overflowMsg, &overNotification}
	}
	rpcSrv.subsLock.Unlock()

	// Wait for the result.
waitloop:
	for {
		select {
		case aer := <-rcvr:
			require.Equal(t, tx.Hash(), aer.Container)
			require.Equal(t, trigger.Application, aer.Trigger)
			require.Equal(t, vmstate.Halt, aer.VMState)
			break waitloop
		case <-time.NewTimer(chain.GetConfig().TimePerBlock).C:
			t.Fatal("transaction failed to be awaited")
		}
	}
}

// TestWSClient_SubscriptionsCompat is aimed to test both deprecated and relevant
// subscriptions API with filtered and non-filtered subscriptions from the WSClient
// user side.
func TestWSClient_SubscriptionsCompat(t *testing.T) {
	chain, rpcSrv, httpSrv := initClearServerWithServices(t, false, false, true)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c := mkSubsClient(t, rpcSrv, httpSrv, false)
	blocks := getTestBlocks(t)
	bCount := uint32(0)

	getData := func(t *testing.T) (*block.Block, int, util.Uint160, string, string) {
		b1 := blocks[bCount]
		primary := int(b1.PrimaryIndex)
		tx := b1.Transactions[0]
		sender := tx.Sender()
		ntfName := "Transfer"
		st := vmstate.Halt.String()
		bCount++
		return b1, primary, sender, ntfName, st
	}
	checkDeprecated := func(t *testing.T, filtered bool) {
		var (
			bID, txID, ntfID, aerID string
			err                     error
		)
		b, primary, sender, ntfName, st := getData(t)
		if filtered {
			bID, err = c.SubscribeForNewBlocks(&primary) //nolint:staticcheck // SA1019: c.SubscribeForNewBlocks is deprecated
			require.NoError(t, err)
			txID, err = c.SubscribeForNewTransactions(&sender, nil) //nolint:staticcheck // SA1019: c.SubscribeForNewTransactions is deprecated
			require.NoError(t, err)
			ntfID, err = c.SubscribeForExecutionNotifications(nil, &ntfName) //nolint:staticcheck // SA1019: c.SubscribeForExecutionNotifications is deprecated
			require.NoError(t, err)
			aerID, err = c.SubscribeForTransactionExecutions(&st) //nolint:staticcheck // SA1019: c.SubscribeForTransactionExecutions is deprecated
			require.NoError(t, err)
		} else {
			bID, err = c.SubscribeForNewBlocks(nil) //nolint:staticcheck // SA1019: c.SubscribeForNewBlocks is deprecated
			require.NoError(t, err)
			txID, err = c.SubscribeForNewTransactions(nil, nil) //nolint:staticcheck // SA1019: c.SubscribeForNewTransactions is deprecated
			require.NoError(t, err)
			ntfID, err = c.SubscribeForExecutionNotifications(nil, nil) //nolint:staticcheck // SA1019: c.SubscribeForExecutionNotifications is deprecated
			require.NoError(t, err)
			aerID, err = c.SubscribeForTransactionExecutions(nil) //nolint:staticcheck // SA1019: c.SubscribeForTransactionExecutions is deprecated
			require.NoError(t, err)
		}

		var (
			lock     sync.RWMutex
			received byte
			exitCh   = make(chan struct{})
		)
		go func() {
		dispatcher:
			for {
				select {
				case ntf := <-c.Notifications: //nolint:staticcheck // SA1019: c.Notifications is deprecated
					lock.Lock()
					switch ntf.Type {
					case neorpc.BlockEventID:
						received |= 1
					case neorpc.TransactionEventID:
						received |= 1 << 1
					case neorpc.NotificationEventID:
						received |= 1 << 2
					case neorpc.ExecutionEventID:
						received |= 1 << 3
					}
					lock.Unlock()
				case <-exitCh:
					break dispatcher
				}
			}
		drainLoop:
			for {
				select {
				case <-c.Notifications: //nolint:staticcheck // SA1019: c.Notifications is deprecated
				default:
					break drainLoop
				}
			}
		}()

		// Accept the next block and wait for events.
		require.NoError(t, chain.AddBlock(b))
		assert.Eventually(t, func() bool {
			lock.RLock()
			defer lock.RUnlock()

			return received == 1<<4-1
		}, time.Second, 100*time.Millisecond)

		require.NoError(t, c.Unsubscribe(bID))
		require.NoError(t, c.Unsubscribe(txID))
		require.NoError(t, c.Unsubscribe(ntfID))
		require.NoError(t, c.Unsubscribe(aerID))
		exitCh <- struct{}{}
	}
	t.Run("deprecated, filtered", func(t *testing.T) {
		checkDeprecated(t, true)
	})
	t.Run("deprecated, non-filtered", func(t *testing.T) {
		checkDeprecated(t, false)
	})

	checkRelevant := func(t *testing.T, filtered bool) {
		b, primary, sender, ntfName, st := getData(t)
		var (
			bID, txID, ntfID, aerID string
			blockCh                 = make(chan *block.Block)
			txCh                    = make(chan *transaction.Transaction)
			ntfCh                   = make(chan *state.ContainedNotificationEvent)
			aerCh                   = make(chan *state.AppExecResult)
			bFlt                    *neorpc.BlockFilter
			txFlt                   *neorpc.TxFilter
			ntfFlt                  *neorpc.NotificationFilter
			aerFlt                  *neorpc.ExecutionFilter
			err                     error
		)
		if filtered {
			bFlt = &neorpc.BlockFilter{Primary: &primary}
			txFlt = &neorpc.TxFilter{Sender: &sender}
			ntfFlt = &neorpc.NotificationFilter{Name: &ntfName}
			aerFlt = &neorpc.ExecutionFilter{State: &st}
		}
		bID, err = c.ReceiveBlocks(bFlt, blockCh)
		require.NoError(t, err)
		txID, err = c.ReceiveTransactions(txFlt, txCh)
		require.NoError(t, err)
		ntfID, err = c.ReceiveExecutionNotifications(ntfFlt, ntfCh)
		require.NoError(t, err)
		aerID, err = c.ReceiveExecutions(aerFlt, aerCh)
		require.NoError(t, err)

		var (
			lock     sync.RWMutex
			received byte
			exitCh   = make(chan struct{})
		)
		go func() {
		dispatcher:
			for {
				select {
				case <-blockCh:
					lock.Lock()
					received |= 1
					lock.Unlock()
				case <-txCh:
					lock.Lock()
					received |= 1 << 1
					lock.Unlock()
				case <-ntfCh:
					lock.Lock()
					received |= 1 << 2
					lock.Unlock()
				case <-aerCh:
					lock.Lock()
					received |= 1 << 3
					lock.Unlock()
				case <-exitCh:
					break dispatcher
				}
			}
		drainLoop:
			for {
				select {
				case <-blockCh:
				case <-txCh:
				case <-ntfCh:
				case <-aerCh:
				default:
					break drainLoop
				}
			}
			close(blockCh)
			close(txCh)
			close(ntfCh)
			close(aerCh)
		}()

		// Accept the next block and wait for events.
		require.NoError(t, chain.AddBlock(b))
		assert.Eventually(t, func() bool {
			lock.RLock()
			defer lock.RUnlock()

			return received == 1<<4-1
		}, time.Second, 100*time.Millisecond)

		require.NoError(t, c.Unsubscribe(bID))
		require.NoError(t, c.Unsubscribe(txID))
		require.NoError(t, c.Unsubscribe(ntfID))
		require.NoError(t, c.Unsubscribe(aerID))
		exitCh <- struct{}{}
	}
	t.Run("relevant, filtered", func(t *testing.T) {
		checkRelevant(t, true)
	})
	t.Run("relevant, non-filtered", func(t *testing.T) {
		checkRelevant(t, false)
	})
}

func TestActor_CallWithNilParam(t *testing.T) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	c, err := rpcclient.New(context.Background(), httpSrv.URL, rpcclient.Options{})
	require.NoError(t, err)
	acc, err := wallet.NewAccount()
	require.NoError(t, err)
	act, err := actor.New(c, []actor.SignerAccount{
		{
			Signer: transaction.Signer{
				Account: acc.ScriptHash(),
			},
			Account: acc,
		},
	})
	require.NoError(t, err)

	rubles, err := chain.GetContractScriptHash(basicchain.RublesContractID)
	require.NoError(t, err)

	// We don't have a suitable contract, thus use Rubles with simple put method,
	// it should fail at the moment of conversion Null value to ByteString (not earlier,
	// and that's the point of the test!).
	res, err := act.Call(rubles, "putValue", "123", (*util.Uint160)(nil))
	require.NoError(t, err)

	require.True(t, strings.Contains(res.FaultException, "invalid conversion: Null/ByteString"), res.FaultException)
}
