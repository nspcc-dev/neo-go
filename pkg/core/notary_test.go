package core

import (
	"errors"
	"fmt"
	"math/rand"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/services/notary"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var notaryModulePath = filepath.Join("..", "services", "notary")

func getTestNotary(t *testing.T, bc *Blockchain, walletPath, pass string, onTx func(tx *transaction.Transaction) error) (*wallet.Account, *notary.Notary, *mempool.Pool) {
	mainCfg := config.P2PNotary{
		Enabled: true,
		UnlockWallet: config.Wallet{
			Path:     filepath.Join(notaryModulePath, walletPath),
			Password: pass,
		},
	}
	cfg := notary.Config{
		MainCfg: mainCfg,
		Chain:   bc,
		Log:     zaptest.NewLogger(t),
	}
	mp := mempool.New(10, 1, true)
	ntr, err := notary.NewNotary(cfg, testchain.Network(), mp, onTx)
	require.NoError(t, err)

	w, err := wallet.NewWalletFromFile(path.Join(notaryModulePath, walletPath))
	require.NoError(t, err)
	require.NoError(t, w.Accounts[0].Decrypt(pass, w.Scrypt))
	return w.Accounts[0], ntr, mp
}

// dupNotaryRequest duplicates notary request by serializing/deserializing it. Use
// it to avoid data races when reusing the same payload. Normal OnNewRequest handler
// never receives the same (as in the same pointer) payload multiple times, even if
// the contents is the same it would be a separate buffer.
func dupNotaryRequest(t *testing.T, p *payload.P2PNotaryRequest) *payload.P2PNotaryRequest {
	b, err := p.Bytes()
	require.NoError(t, err)
	r, err := payload.NewP2PNotaryRequestFromBytes(b)
	require.NoError(t, err)
	return r
}

func TestNotary(t *testing.T) {
	bc := newTestChain(t)
	var (
		nonce           uint32
		nvbDiffFallback uint32 = 20
	)

	mtx := sync.RWMutex{}
	completedTxes := make(map[util.Uint256]*transaction.Transaction)
	var unluckies []*payload.P2PNotaryRequest
	var (
		finalizeWithError bool
		choosy            bool
	)
	setFinalizeWithError := func(v bool) {
		mtx.Lock()
		finalizeWithError = v
		mtx.Unlock()
	}
	setChoosy := func(v bool) {
		mtx.Lock()
		choosy = v
		mtx.Unlock()
	}
	onTransaction := func(tx *transaction.Transaction) error {
		mtx.Lock()
		defer mtx.Unlock()
		if !choosy {
			if completedTxes[tx.Hash()] != nil {
				panic("transaction was completed twice")
			}
			if finalizeWithError {
				return errors.New("error while finalizing transaction")
			}
			completedTxes[tx.Hash()] = tx
			return nil
		}
		for _, unl := range unluckies {
			if tx.Hash().Equals(unl.FallbackTransaction.Hash()) {
				return errors.New("error while finalizing transaction")
			}
		}
		completedTxes[tx.Hash()] = tx
		return nil
	}
	getCompletedTx := func(t *testing.T, waitForNonNil bool, h util.Uint256) *transaction.Transaction {
		if !waitForNonNil {
			mtx.RLock()
			defer mtx.RUnlock()
			return completedTxes[h]
		}
		var completedTx *transaction.Transaction
		require.Eventually(t, func() bool {
			mtx.RLock()
			defer mtx.RUnlock()
			completedTx = completedTxes[h]
			return completedTx != nil
		}, time.Second*3, time.Millisecond*50, errors.New("main transaction expected to be completed"))
		return completedTx
	}

	acc1, ntr1, mp1 := getTestNotary(t, bc, "./testdata/notary1.json", "one", onTransaction)
	acc2, _, _ := getTestNotary(t, bc, "./testdata/notary2.json", "two", onTransaction)
	randomAcc, err := keys.NewPrivateKey()
	require.NoError(t, err)

	bc.SetNotary(ntr1)
	bc.RegisterPostBlock(func(bc blockchainer.Blockchainer, pool *mempool.Pool, b *block.Block) {
		ntr1.PostPersist()
	})

	mp1.RunSubscriptions()
	go ntr1.Run()
	t.Cleanup(func() {
		ntr1.Stop()
		mp1.StopSubscriptions()
	})

	notaryNodes := keys.PublicKeys{acc1.PrivateKey().PublicKey(), acc2.PrivateKey().PublicKey()}
	bc.setNodesByRole(t, true, noderoles.P2PNotary, notaryNodes)

	type requester struct {
		accounts []*wallet.Account
		m        int
		typ      notary.RequestType
	}
	createFallbackTx := func(requester *wallet.Account, mainTx *transaction.Transaction, nvbIncrement ...uint32) *transaction.Transaction {
		fallback := transaction.New([]byte{byte(opcode.RET)}, 2000_0000)
		fallback.Nonce = nonce
		nonce++
		fallback.SystemFee = 1_0000_0000
		fallback.ValidUntilBlock = bc.BlockHeight() + 2*nvbDiffFallback
		fallback.Signers = []transaction.Signer{
			{
				Account: bc.GetNotaryContractScriptHash(),
				Scopes:  transaction.None,
			},
			{
				Account: requester.PrivateKey().PublicKey().GetScriptHash(),
				Scopes:  transaction.None,
			},
		}
		nvb := bc.BlockHeight() + nvbDiffFallback
		if len(nvbIncrement) != 0 {
			nvb += nvbIncrement[0]
		}
		fallback.Attributes = []transaction.Attribute{
			{
				Type:  transaction.NotaryAssistedT,
				Value: &transaction.NotaryAssisted{NKeys: 0},
			},
			{
				Type:  transaction.NotValidBeforeT,
				Value: &transaction.NotValidBefore{Height: nvb},
			},
			{
				Type:  transaction.ConflictsT,
				Value: &transaction.Conflicts{Hash: mainTx.Hash()},
			},
		}
		fallback.Scripts = []transaction.Witness{
			{
				InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64)...),
				VerificationScript: []byte{},
			},
		}
		err = requester.SignTx(testchain.Network(), fallback)
		require.NoError(t, err)
		return fallback
	}
	createMixedRequest := func(requesters []requester, NVBincrements ...uint32) []*payload.P2PNotaryRequest {
		mainTx := *transaction.New([]byte{byte(opcode.RET)}, 11000000)
		mainTx.Nonce = nonce
		nonce++
		mainTx.SystemFee = 100000000
		mainTx.ValidUntilBlock = bc.BlockHeight() + 2*nvbDiffFallback
		signers := make([]transaction.Signer, len(requesters)+1)
		var (
			nKeys               uint8
			verificationScripts [][]byte
		)
		for i := range requesters {
			var script []byte
			switch requesters[i].typ {
			case notary.Signature:
				script = requesters[i].accounts[0].PrivateKey().PublicKey().GetVerificationScript()
				nKeys++
			case notary.MultiSignature:
				pubs := make(keys.PublicKeys, len(requesters[i].accounts))
				for j, r := range requesters[i].accounts {
					pubs[j] = r.PrivateKey().PublicKey()
				}
				script, err = smartcontract.CreateMultiSigRedeemScript(requesters[i].m, pubs)
				require.NoError(t, err)
				nKeys += uint8(len(requesters[i].accounts))
			}
			signers[i] = transaction.Signer{
				Account: hash.Hash160(script),
				Scopes:  transaction.None,
			}
			verificationScripts = append(verificationScripts, script)
		}
		signers[len(signers)-1] = transaction.Signer{
			Account: bc.GetNotaryContractScriptHash(),
			Scopes:  transaction.None,
		}
		mainTx.Signers = signers
		mainTx.Attributes = []transaction.Attribute{
			{
				Type:  transaction.NotaryAssistedT,
				Value: &transaction.NotaryAssisted{NKeys: nKeys},
			},
		}
		payloads := make([]*payload.P2PNotaryRequest, nKeys)
		plIndex := 0
		// we'll collect only m signatures out of n (so only m payloads are needed), but let's create payloads for all requesters (for the next tests)
		for i, r := range requesters {
			for _, acc := range r.accounts {
				cp := mainTx
				main := &cp
				main.Scripts = make([]transaction.Witness, len(requesters))
				for j := range main.Scripts {
					main.Scripts[j].VerificationScript = verificationScripts[j]
					if i == j {
						main.Scripts[j].InvocationScript = append([]byte{byte(opcode.PUSHDATA1), 64}, acc.PrivateKey().SignHashable(uint32(testchain.Network()), main)...)
					}
				}
				main.Scripts = append(main.Scripts, transaction.Witness{}) // empty Notary witness

				_ = main.Size() // for size update test

				var fallback *transaction.Transaction
				if len(NVBincrements) == int(nKeys) {
					fallback = createFallbackTx(acc, main, NVBincrements[plIndex])
				} else {
					fallback = createFallbackTx(acc, main)
				}

				_ = fallback.Size() // for size update test

				payloads[plIndex] = &payload.P2PNotaryRequest{
					MainTransaction:     main,
					FallbackTransaction: fallback,
				}
				plIndex++
			}
		}
		return payloads
	}
	checkMainTx := func(t *testing.T, requesters []requester, requests []*payload.P2PNotaryRequest, sentCount int, shouldComplete bool) {
		nSigs := 0
		for _, r := range requesters {
			switch r.typ {
			case notary.Signature:
				nSigs++
			case notary.MultiSignature:
				nSigs += r.m
			}
		}
		nSigners := len(requesters) + 1
		if sentCount >= nSigs && shouldComplete {
			completedTx := getCompletedTx(t, true, requests[0].MainTransaction.Hash())
			require.Equal(t, nSigners, len(completedTx.Signers))
			require.Equal(t, nSigners, len(completedTx.Scripts))

			// check that tx size was updated
			require.Equal(t, io.GetVarSize(completedTx), completedTx.Size())

			for i := 0; i < len(completedTx.Scripts)-1; i++ {
				interopCtx := bc.newInteropContext(trigger.Verification, bc.dao, nil, completedTx)
				_, err := bc.verifyHashAgainstScript(completedTx.Signers[i].Account, &completedTx.Scripts[i], interopCtx, -1)
				require.NoError(t, err)
			}
			require.Equal(t, transaction.Witness{
				InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), 64}, acc1.PrivateKey().SignHashable(uint32(testchain.Network()), requests[0].MainTransaction)...),
				VerificationScript: []byte{},
			}, completedTx.Scripts[len(completedTx.Scripts)-1])
		} else {
			completedTx := getCompletedTx(t, false, requests[0].MainTransaction.Hash())
			require.Nil(t, completedTx, fmt.Errorf("main transaction shouldn't be completed: sent %d out of %d requests", sentCount, nSigs))
		}
	}
	checkFallbackTxs := func(t *testing.T, requests []*payload.P2PNotaryRequest, shouldComplete bool) {
		for i, req := range requests {
			if shouldComplete {
				completedTx := getCompletedTx(t, true, req.FallbackTransaction.Hash())
				require.Equal(t, 2, len(completedTx.Signers))
				require.Equal(t, 2, len(completedTx.Scripts))
				require.Equal(t, transaction.Witness{
					InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), 64}, acc1.PrivateKey().SignHashable(uint32(testchain.Network()), req.FallbackTransaction)...),
					VerificationScript: []byte{},
				}, completedTx.Scripts[0])

				// check that tx size was updated
				require.Equal(t, io.GetVarSize(completedTx), completedTx.Size())

				interopCtx := bc.newInteropContext(trigger.Verification, bc.dao, nil, completedTx)
				_, err := bc.verifyHashAgainstScript(completedTx.Signers[1].Account, &completedTx.Scripts[1], interopCtx, -1)
				require.NoError(t, err)
			} else {
				completedTx := getCompletedTx(t, false, req.FallbackTransaction.Hash())
				require.Nil(t, completedTx, fmt.Errorf("fallback transaction for request #%d shouldn't be completed", i))
			}
		}
	}
	checkCompleteStandardRequest := func(t *testing.T, nKeys int, shouldComplete bool, nvbIncrements ...uint32) ([]*payload.P2PNotaryRequest, []requester) {
		requesters := make([]requester, nKeys)
		for i := range requesters {
			acc, _ := wallet.NewAccount()
			requesters[i] = requester{
				accounts: []*wallet.Account{acc},
				typ:      notary.Signature,
			}
		}

		requests := createMixedRequest(requesters, nvbIncrements...)
		sendOrder := make([]int, nKeys)
		for i := range sendOrder {
			sendOrder[i] = i
		}
		rand.Shuffle(nKeys, func(i, j int) {
			sendOrder[j], sendOrder[i] = sendOrder[i], sendOrder[j]
		})
		for i := range requests {
			ntr1.OnNewRequest(requests[sendOrder[i]])
			checkMainTx(t, requesters, requests, i+1, shouldComplete)
			completedCount := len(completedTxes)

			// check that the same request won't be processed twice
			ntr1.OnNewRequest(dupNotaryRequest(t, requests[sendOrder[i]]))
			checkMainTx(t, requesters, requests, i+1, shouldComplete)
			require.Equal(t, completedCount, len(completedTxes))
		}
		return requests, requesters
	}
	checkCompleteMultisigRequest := func(t *testing.T, nSigs int, nKeys int, shouldComplete bool) ([]*payload.P2PNotaryRequest, []requester) {
		accounts := make([]*wallet.Account, nKeys)
		for i := range accounts {
			accounts[i], _ = wallet.NewAccount()
		}
		requesters := []requester{
			{
				accounts: accounts,
				m:        nSigs,
				typ:      notary.MultiSignature,
			},
		}
		requests := createMixedRequest(requesters)
		sendOrder := make([]int, nKeys)
		for i := range sendOrder {
			sendOrder[i] = i
		}
		rand.Shuffle(nKeys, func(i, j int) {
			sendOrder[j], sendOrder[i] = sendOrder[i], sendOrder[j]
		})

		var submittedRequests []*payload.P2PNotaryRequest
		// sent only nSigs (m out of n) requests - it should be enough to complete min tx
		for i := 0; i < nSigs; i++ {
			submittedRequests = append(submittedRequests, requests[sendOrder[i]])

			ntr1.OnNewRequest(requests[sendOrder[i]])
			checkMainTx(t, requesters, submittedRequests, i+1, shouldComplete)

			// check that the same request won't be processed twice
			ntr1.OnNewRequest(dupNotaryRequest(t, requests[sendOrder[i]]))
			checkMainTx(t, requesters, submittedRequests, i+1, shouldComplete)
		}

		// sent the rest (n-m) out of n requests: main tx is already collected, so only fallbacks should be applied
		completedCount := len(completedTxes)
		for i := nSigs; i < nKeys; i++ {
			submittedRequests = append(submittedRequests, requests[sendOrder[i]])

			ntr1.OnNewRequest(requests[sendOrder[i]])
			checkMainTx(t, requesters, submittedRequests, i+1, shouldComplete)
			require.Equal(t, completedCount, len(completedTxes))
		}

		return submittedRequests, requesters
	}

	checkCompleteMixedRequest := func(t *testing.T, nSigSigners int, shouldComplete bool) ([]*payload.P2PNotaryRequest, []requester) {
		requesters := make([]requester, nSigSigners)
		for i := range requesters {
			acc, _ := wallet.NewAccount()
			requesters[i] = requester{
				accounts: []*wallet.Account{acc},
				typ:      notary.Signature,
			}
		}
		multisigAccounts := make([]*wallet.Account, 3)
		for i := range multisigAccounts {
			multisigAccounts[i], _ = wallet.NewAccount()
		}

		requesters = append(requesters, requester{
			accounts: multisigAccounts,
			m:        2,
			typ:      notary.MultiSignature,
		})

		requests := createMixedRequest(requesters)
		for i := range requests {
			ntr1.OnNewRequest(requests[i])
			checkMainTx(t, requesters, requests, i+1, shouldComplete)
			completedCount := len(completedTxes)

			// check that the same request won't be processed twice
			ntr1.OnNewRequest(dupNotaryRequest(t, requests[i]))
			checkMainTx(t, requesters, requests, i+1, shouldComplete)
			require.Equal(t, completedCount, len(completedTxes))
		}
		return requests, requesters
	}

	// OnNewRequest: missing account
	ntr1.UpdateNotaryNodes(keys.PublicKeys{randomAcc.PublicKey()})
	r, _ := checkCompleteStandardRequest(t, 1, false)
	checkFallbackTxs(t, r, false)
	// set account back for the next tests
	ntr1.UpdateNotaryNodes(keys.PublicKeys{acc1.PrivateKey().PublicKey()})

	// OnNewRequest: signature request
	for _, i := range []int{1, 2, 3, 10} {
		r, _ := checkCompleteStandardRequest(t, i, true)
		checkFallbackTxs(t, r, false)
	}

	// OnNewRequest: multisignature request
	r, _ = checkCompleteMultisigRequest(t, 1, 1, true)
	checkFallbackTxs(t, r, false)
	r, _ = checkCompleteMultisigRequest(t, 1, 2, true)
	checkFallbackTxs(t, r, false)
	r, _ = checkCompleteMultisigRequest(t, 1, 3, true)
	checkFallbackTxs(t, r, false)
	r, _ = checkCompleteMultisigRequest(t, 3, 3, true)
	checkFallbackTxs(t, r, false)
	r, _ = checkCompleteMultisigRequest(t, 3, 4, true)
	checkFallbackTxs(t, r, false)
	r, _ = checkCompleteMultisigRequest(t, 3, 10, true)
	checkFallbackTxs(t, r, false)

	// OnNewRequest: mixed request
	r, _ = checkCompleteMixedRequest(t, 1, true)
	checkFallbackTxs(t, r, false)
	r, _ = checkCompleteMixedRequest(t, 2, true)
	checkFallbackTxs(t, r, false)
	r, _ = checkCompleteMixedRequest(t, 3, true)
	checkFallbackTxs(t, r, false)
	// PostPersist: missing account
	setFinalizeWithError(true)
	r, requesters := checkCompleteStandardRequest(t, 1, false)
	checkFallbackTxs(t, r, false)
	ntr1.UpdateNotaryNodes(keys.PublicKeys{randomAcc.PublicKey()})
	setFinalizeWithError(false)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, r, 1, false)
	checkFallbackTxs(t, r, false)
	// set account back for the next tests
	ntr1.UpdateNotaryNodes(keys.PublicKeys{acc1.PrivateKey().PublicKey()})

	// PostPersist: complete main transaction, signature request
	setFinalizeWithError(true)
	requests, requesters := checkCompleteStandardRequest(t, 3, false)
	// check PostPersist with finalisation error
	setFinalizeWithError(true)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	// check PostPersist without finalisation error
	setFinalizeWithError(false)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), true)

	// PostPersist: complete main transaction, multisignature account
	setFinalizeWithError(true)
	requests, requesters = checkCompleteMultisigRequest(t, 3, 4, false)
	checkFallbackTxs(t, requests, false)
	// check PostPersist with finalisation error
	setFinalizeWithError(true)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests, false)
	// check PostPersist without finalisation error
	setFinalizeWithError(false)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), true)
	checkFallbackTxs(t, requests, false)

	// PostPersist: complete fallback, signature request
	setFinalizeWithError(true)
	requests, requesters = checkCompleteStandardRequest(t, 3, false)
	checkFallbackTxs(t, requests, false)
	// make fallbacks valid
	_, err = bc.genBlocks(int(nvbDiffFallback))
	require.NoError(t, err)
	// check PostPersist for valid fallbacks with finalisation error
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests, false)
	// check PostPersist for valid fallbacks without finalisation error
	setFinalizeWithError(false)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests, true)

	// PostPersist: complete fallback, multisignature request
	nSigs, nKeys := 3, 5
	// check OnNewRequest with finalization error
	setFinalizeWithError(true)
	requests, requesters = checkCompleteMultisigRequest(t, nSigs, nKeys, false)
	checkFallbackTxs(t, requests, false)
	// make fallbacks valid
	_, err = bc.genBlocks(int(nvbDiffFallback))
	require.NoError(t, err)
	// check PostPersist for valid fallbacks with finalisation error
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests, false)
	// check PostPersist for valid fallbacks without finalisation error
	setFinalizeWithError(false)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests[:nSigs], true)
	// the rest of fallbacks should also be applied even if the main tx was already constructed by the moment they were sent
	checkFallbackTxs(t, requests[nSigs:], true)

	// PostPersist: partial fallbacks completion due to finalisation errors
	setFinalizeWithError(true)
	requests, requesters = checkCompleteStandardRequest(t, 5, false)
	checkFallbackTxs(t, requests, false)
	// make fallbacks valid
	_, err = bc.genBlocks(int(nvbDiffFallback))
	require.NoError(t, err)
	// some of fallbacks should fail finalisation
	unluckies = []*payload.P2PNotaryRequest{requests[0], requests[4]}
	lucky := requests[1:4]
	setChoosy(true)
	// check PostPersist for lucky fallbacks
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, lucky, true)
	checkFallbackTxs(t, unluckies, false)
	// reset finalisation function for unlucky fallbacks to finalise without an error
	setChoosy(false)
	setFinalizeWithError(false)
	// check PostPersist for unlucky fallbacks
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, lucky, true)
	checkFallbackTxs(t, unluckies, true)

	// PostPersist: different NVBs
	// check OnNewRequest with finalization error and different NVBs
	setFinalizeWithError(true)
	requests, requesters = checkCompleteStandardRequest(t, 5, false, 1, 2, 3, 4, 5)
	checkFallbackTxs(t, requests, false)
	// generate blocks to reach the most earlier fallback's NVB
	_, err = bc.genBlocks(int(nvbDiffFallback))
	require.NoError(t, err)
	// check PostPersist for valid fallbacks without finalisation error
	// Add block before allowing tx to finalize to exclude race condition when
	// main transaction is finalized between `finalizeWithError` restore and adding new block.
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	mtx.RLock()
	start := len(completedTxes)
	mtx.RUnlock()
	setFinalizeWithError(false)
	for i := range requests {
		if i != 0 {
			require.NoError(t, bc.AddBlock(bc.newBlock()))
		}
		require.Eventually(t, func() bool {
			mtx.RLock()
			defer mtx.RUnlock()
			return len(completedTxes)-start >= i+1
		}, time.Second*3, time.Millisecond)
		checkMainTx(t, requesters, requests, len(requests), false)
		checkFallbackTxs(t, requests[:i+1], true)
		checkFallbackTxs(t, requests[i+1:], false)
	}

	// OnRequestRemoval: missing account
	// check OnNewRequest with finalization error
	setFinalizeWithError(true)
	requests, requesters = checkCompleteStandardRequest(t, 4, false)
	checkFallbackTxs(t, requests, false)
	// make fallbacks valid and remove one fallback
	_, err = bc.genBlocks(int(nvbDiffFallback))
	require.NoError(t, err)
	ntr1.UpdateNotaryNodes(keys.PublicKeys{randomAcc.PublicKey()})
	ntr1.OnRequestRemoval(requests[3])
	// non of the fallbacks should be completed
	setFinalizeWithError(false)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests, false)
	// set account back for the next tests
	ntr1.UpdateNotaryNodes(keys.PublicKeys{acc1.PrivateKey().PublicKey()})

	// OnRequestRemoval: signature request, remove one fallback
	// check OnNewRequest with finalization error
	setFinalizeWithError(true)
	requests, requesters = checkCompleteStandardRequest(t, 4, false)
	checkFallbackTxs(t, requests, false)
	// make fallbacks valid and remove one fallback
	_, err = bc.genBlocks(int(nvbDiffFallback))
	require.NoError(t, err)
	unlucky := requests[3]
	ntr1.OnRequestRemoval(unlucky)
	// rest of the fallbacks should be completed
	setFinalizeWithError(false)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests[:3], true)
	require.Nil(t, completedTxes[unlucky.FallbackTransaction.Hash()])

	// OnRequestRemoval: signature request, remove all fallbacks
	setFinalizeWithError(true)
	requests, requesters = checkCompleteStandardRequest(t, 4, false)
	// remove all fallbacks
	_, err = bc.genBlocks(int(nvbDiffFallback))
	require.NoError(t, err)
	for i := range requests {
		ntr1.OnRequestRemoval(requests[i])
	}
	// then the whole request should be removed, i.e. there are no completed transactions
	setFinalizeWithError(false)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests, false)

	// OnRequestRemoval: signature request, remove unexisting fallback
	ntr1.OnRequestRemoval(requests[0])
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests, false)

	// OnRequestRemoval: multisignature request, remove one fallback
	nSigs, nKeys = 3, 5
	// check OnNewRequest with finalization error
	setFinalizeWithError(true)
	requests, requesters = checkCompleteMultisigRequest(t, nSigs, nKeys, false)
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests, false)
	// make fallbacks valid and remove the last fallback
	_, err = bc.genBlocks(int(nvbDiffFallback))
	require.NoError(t, err)
	unlucky = requests[nSigs-1]
	ntr1.OnRequestRemoval(unlucky)
	// then (m-1) out of n fallbacks should be completed
	setFinalizeWithError(false)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests[:nSigs-1], true)
	require.Nil(t, completedTxes[unlucky.FallbackTransaction.Hash()])
	//  the rest (n-(m-1)) out of n fallbacks should also be completed even if main tx has been collected by the moment they were sent
	checkFallbackTxs(t, requests[nSigs:], true)

	// OnRequestRemoval: multisignature request, remove all fallbacks
	setFinalizeWithError(true)
	requests, requesters = checkCompleteMultisigRequest(t, nSigs, nKeys, false)
	// make fallbacks valid and then remove all of them
	_, err = bc.genBlocks(int(nvbDiffFallback))
	require.NoError(t, err)
	for i := range requests {
		ntr1.OnRequestRemoval(requests[i])
	}
	// then the whole request should be removed, i.e. there are no completed transactions
	setFinalizeWithError(false)
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests, false)

	// // OnRequestRemoval: multisignature request, remove unexisting fallbac, i.e. there still shouldn't be any completed transactions after this
	ntr1.OnRequestRemoval(requests[0])
	require.NoError(t, bc.AddBlock(bc.newBlock()))
	checkMainTx(t, requesters, requests, len(requests), false)
	checkFallbackTxs(t, requests, false)

	// Subscriptions test
	setFinalizeWithError(false)
	requester1, _ := wallet.NewAccount()
	requester2, _ := wallet.NewAccount()
	amount := int64(100_0000_0000)
	feer := NewNotaryFeerStub(bc)
	transferTokenFromMultisigAccountCheckOK(t, bc, bc.GetNotaryContractScriptHash(), bc.contracts.GAS.Hash, amount, requester1.PrivateKey().PublicKey().GetScriptHash(), int64(bc.BlockHeight()+50))
	checkBalanceOf(t, bc, bc.contracts.Notary.Hash, int(amount))
	transferTokenFromMultisigAccountCheckOK(t, bc, bc.GetNotaryContractScriptHash(), bc.contracts.GAS.Hash, amount, requester2.PrivateKey().PublicKey().GetScriptHash(), int64(bc.BlockHeight()+50))
	checkBalanceOf(t, bc, bc.contracts.Notary.Hash, int(2*amount))
	// create request for 2 standard signatures => main tx should be completed after the second request is added to the pool
	requests = createMixedRequest([]requester{
		{
			accounts: []*wallet.Account{requester1},
			typ:      notary.Signature,
		},
		{
			accounts: []*wallet.Account{requester2},
			typ:      notary.Signature,
		},
	})
	require.NoError(t, mp1.Add(requests[0].FallbackTransaction, feer, requests[0]))
	require.NoError(t, mp1.Add(requests[1].FallbackTransaction, feer, requests[1]))
	require.Eventually(t, func() bool {
		mtx.RLock()
		defer mtx.RUnlock()
		return completedTxes[requests[0].MainTransaction.Hash()] != nil
	}, 3*time.Second, 100*time.Millisecond)
	checkFallbackTxs(t, requests, false)
}
