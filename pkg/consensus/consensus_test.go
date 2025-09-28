package consensus

import (
	"testing"
	"time"

	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	coreb "github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	npayload "github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewService(t *testing.T) {
	srv := newTestService(t)
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 100000)
	tx.ValidUntilBlock = 1
	addSender(t, tx)
	signTx(t, srv.Chain, tx)
	require.NoError(t, srv.Chain.PoolTx(tx))

	var txx []dbft.Transaction[util.Uint256]
	require.NotPanics(t, func() { txx = srv.getVerifiedTx() })
	require.Len(t, txx, 1)
	require.Equal(t, tx, txx[0])
}

func TestNewWatchingService(t *testing.T) {
	bc := newTestChain(t, false)
	srv, err := NewService(Config{
		Logger:                zaptest.NewLogger(t),
		Broadcast:             func(*npayload.Extensible) {},
		Chain:                 bc,
		BlockQueue:            testBlockQueuer{bc: bc},
		ProtocolConfiguration: bc.GetConfig().ProtocolConfiguration,
		RequestTx:             func(...util.Uint256) {},
		StopTxFlow:            func() {},
		// No wallet provided.
	})
	require.NoError(t, err)

	require.NotPanics(t, srv.Start)
	require.NotPanics(t, srv.Shutdown)
}

func collectBlock(t *testing.T, bc *core.Blockchain, srv *service) {
	h := bc.BlockHeight()
	srv.dbft.OnTimeout(srv.dbft.BlockIndex, 0) // Collect and add block to the chain.
	header, err := bc.GetHeader(bc.GetHeaderHash(h + 1))
	require.NoError(t, err)
	srv.dbft.Reset(header.Timestamp * nsInMs) // Init consensus manually at the next height, as we don't run the consensus service.
}

func initServiceNextConsensus(t *testing.T, newAcc *wallet.Account, offset uint32) (*service, *wallet.Account) {
	acc, err := wallet.NewAccountFromWIF(testchain.WIF(testchain.IDToOrder(0)))
	require.NoError(t, err)
	priv := acc.PrivateKey()
	require.NoError(t, acc.ConvertMultisig(1, keys.PublicKeys{priv.PublicKey()}))

	bc := newSingleTestChain(t)
	newPriv := newAcc.PrivateKey()

	// Transfer funds to new validator.
	b := smartcontract.NewBuilder()
	b.InvokeWithAssert(bc.GoverningTokenHash(), "transfer",
		acc.Contract.ScriptHash().BytesBE(), newPriv.GetScriptHash().BytesBE(), int64(native.NEOTotalSupply), nil)

	b.InvokeWithAssert(bc.UtilityTokenHash(), "transfer",
		acc.Contract.ScriptHash().BytesBE(), newPriv.GetScriptHash().BytesBE(), int64(10000_000_000_000), nil)
	script, err := b.Script()
	require.NoError(t, err)

	tx := transaction.New(script, 21_000_000)
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	tx.NetworkFee = 10_000_000
	tx.Signers = []transaction.Signer{{Scopes: transaction.Global, Account: acc.Contract.ScriptHash()}}
	require.NoError(t, acc.SignTx(netmode.UnitTestNet, tx))
	require.NoError(t, bc.PoolTx(tx))

	srv := newTestServiceWithChain(t, bc)
	h := bc.BlockHeight()
	srv.dbft.Start(0)
	header, err := bc.GetHeader(bc.GetHeaderHash(h + 1))
	require.NoError(t, err)
	srv.dbft.Reset(header.Timestamp * nsInMs) // Init consensus manually at the next height, as we don't run the consensus service.

	// Register new candidate.
	b.Reset()
	b.InvokeWithAssert(bc.GoverningTokenHash(), "registerCandidate", newPriv.PublicKey().Bytes())
	script, err = b.Script()
	require.NoError(t, err)

	tx = transaction.New(script, 1001_00000000)
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	tx.NetworkFee = 20_000_000
	tx.Signers = []transaction.Signer{{Scopes: transaction.Global, Account: newPriv.GetScriptHash()}}
	require.NoError(t, newAcc.SignTx(netmode.UnitTestNet, tx))

	require.NoError(t, bc.PoolTx(tx))
	collectBlock(t, bc, srv)

	cfg := bc.GetConfig()
	for i := srv.dbft.BlockIndex; !cfg.ShouldUpdateCommitteeAt(i + offset); i++ {
		collectBlock(t, bc, srv)
	}

	// Vote for new candidate.
	b.Reset()
	b.InvokeWithAssert(bc.GoverningTokenHash(), "vote",
		newPriv.GetScriptHash(), newPriv.PublicKey().Bytes())
	script, err = b.Script()
	require.NoError(t, err)

	tx = transaction.New(script, 20_000_000)
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	tx.NetworkFee = 20_000_000
	tx.Signers = []transaction.Signer{{Scopes: transaction.Global, Account: newPriv.GetScriptHash()}}
	require.NoError(t, newAcc.SignTx(netmode.UnitTestNet, tx))

	require.NoError(t, bc.PoolTx(tx))
	collectBlock(t, bc, srv)

	return srv, acc
}

func TestService_NextConsensus(t *testing.T) {
	newAcc, err := wallet.NewAccount()
	require.NoError(t, err)
	script, err := smartcontract.CreateMajorityMultiSigRedeemScript(keys.PublicKeys{newAcc.PublicKey()})
	require.NoError(t, err)

	checkNextConsensus := func(t *testing.T, bc *core.Blockchain, height uint32, h util.Uint160) {
		hdrHash := bc.GetHeaderHash(height)
		hdr, err := bc.GetHeader(hdrHash)
		require.NoError(t, err)
		require.Equal(t, h, hdr.NextConsensus)
	}

	t.Run("vote 1 block before update", func(t *testing.T) { // voting occurs every block in SingleTestChain
		srv, acc := initServiceNextConsensus(t, newAcc, 1)
		bc := srv.Chain.(*core.Blockchain)

		height := bc.BlockHeight()
		checkNextConsensus(t, bc, height, acc.Contract.ScriptHash())
		// Reset     <- we are here, update NextConsensus
		// OnPersist <- update committee
		// Block     <-

		collectBlock(t, bc, srv)
		checkNextConsensus(t, bc, height+1, hash.Hash160(script))
	})
	/*
		t.Run("vote 2 blocks before update", func(t *testing.T) {
			srv, acc := initServiceNextConsensus(t, newAcc, 2)
			bc := srv.Chain.(*core.Blockchain)
			defer bc.Close()

			height := bc.BlockHeight()
			checkNextConsensus(t, bc, height, acc.Contract.ScriptHash())
			// Reset     <- we are here
			// OnPersist <- nothing to do
			// Block     <-
			//
			// Reset     <- update next consensus
			// OnPersist <- update committee
			// Block     <-
			srv.dbft.OnTimeout(timer.HV{Height: srv.dbft.BlockIndex})
			checkNextConsensus(t, bc, height+1, acc.Contract.ScriptHash())

			srv.dbft.OnTimeout(timer.HV{Height: srv.dbft.BlockIndex})
			checkNextConsensus(t, bc, height+2, hash.Hash160(script))
		})
	*/
}

func TestService_GetVerified(t *testing.T) {
	srv := newTestService(t)
	srv.dbft.Start(0)
	var txs []*transaction.Transaction
	for i := range 4 {
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 100000)
		tx.Nonce = 123 + uint32(i)
		tx.ValidUntilBlock = 1
		txs = append(txs, tx)
	}
	addSender(t, txs...)
	signTx(t, srv.Chain, txs...)
	require.NoError(t, srv.Chain.PoolTx(txs[3]))

	hashes := []util.Uint256{txs[0].Hash(), txs[1].Hash(), txs[2].Hash()}

	// Everyone sends a message.
	for i := range 4 {
		p := new(Payload)
		// One PrepareRequest and three ChangeViews.
		if i == 1 {
			p.message.Type = messageType(dbft.PrepareRequestType)
			p.payload = &prepareRequest{prevHash: srv.Chain.CurrentBlockHash(), transactionHashes: hashes}
		} else {
			p.message.Type = messageType(dbft.ChangeViewType)
			p.payload = &changeView{newViewNumber: 1, timestamp: uint64(time.Now().UnixNano() / nsInMs)}
		}
		p.BlockIndex = 1
		p.message.ValidatorIndex = byte(i)

		priv, _ := getTestValidator(i)
		require.NoError(t, p.Sign(priv))

		// Skip srv.OnPayload, because the service is not really started.
		srv.dbft.OnReceive(p)
	}
	require.Equal(t, uint8(1), srv.dbft.ViewNumber)
	require.Equal(t, hashes, srv.lastProposal)

	t.Run("new transactions will be proposed in case of failure", func(t *testing.T) {
		txx := srv.getVerifiedTx()
		require.Equal(t, 1, len(txx), "there is only 1 tx in mempool")
		require.Equal(t, txs[3], txx[0])
	})

	t.Run("more than half of the last proposal will be reused", func(t *testing.T) {
		for _, tx := range txs[:2] {
			require.NoError(t, srv.Chain.PoolTx(tx))
		}

		txx := srv.getVerifiedTx()
		require.Contains(t, txx, txs[0])
		require.Contains(t, txx, txs[1])
		require.NotContains(t, txx, txs[2])
	})
}

func TestService_ValidatePayload(t *testing.T) {
	srv := newTestService(t)
	priv, _ := getTestValidator(1)
	p := new(Payload)
	p.Sender = priv.GetScriptHash()
	p.payload = &prepareRequest{}

	t.Run("invalid validator index", func(t *testing.T) {
		p.message.ValidatorIndex = 11
		require.NoError(t, p.Sign(priv))

		var ok bool
		require.NotPanics(t, func() { ok = srv.validatePayload(p) })
		require.False(t, ok)
	})

	t.Run("wrong validator index", func(t *testing.T) {
		p.message.ValidatorIndex = 2
		require.NoError(t, p.Sign(priv))
		require.False(t, srv.validatePayload(p))
	})

	t.Run("invalid sender", func(t *testing.T) {
		p.message.ValidatorIndex = 1
		p.Sender = util.Uint160{}
		require.NoError(t, p.Sign(priv))
		require.False(t, srv.validatePayload(p))
	})

	t.Run("normal case", func(t *testing.T) {
		p.message.ValidatorIndex = 1
		p.Sender = priv.GetScriptHash()
		require.NoError(t, p.Sign(priv))
		require.True(t, srv.validatePayload(p))
	})
}

func TestService_getTx(t *testing.T) {
	srv := newTestService(t)

	t.Run("transaction in mempool", func(t *testing.T) {
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = 1234
		tx.ValidUntilBlock = 1
		addSender(t, tx)
		signTx(t, srv.Chain, tx)
		h := tx.Hash()

		require.Equal(t, nil, srv.getTx(h))

		require.NoError(t, srv.Chain.PoolTx(tx))

		got := srv.getTx(h)
		require.NotNil(t, got)
		require.Equal(t, h, got.Hash())
	})

	t.Run("transaction in local cache", func(t *testing.T) {
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = 4321
		tx.ValidUntilBlock = 1
		h := tx.Hash()

		require.Equal(t, nil, srv.getTx(h))

		srv.txx.Add(tx)

		got := srv.getTx(h)
		require.NotNil(t, got)
		require.Equal(t, h, got.Hash())
	})
}

func TestService_PrepareRequest(t *testing.T) {
	srv := newTestServiceWithState(t, true)
	srv.dbft.Start(0)

	priv, _ := getTestValidator(1)
	p := new(Payload)
	p.message.ValidatorIndex = 1

	prevHash := srv.Chain.CurrentBlockHash()

	checkRequest := func(t *testing.T, expectedErr error, req *prepareRequest) {
		p.payload = req
		require.NoError(t, p.Sign(priv))
		err := srv.verifyRequest(p)
		if expectedErr == nil {
			require.NoError(t, err)
			return
		}
		require.ErrorIs(t, err, expectedErr)
	}

	checkRequest(t, errInvalidVersion, &prepareRequest{version: 0xFF, prevHash: prevHash})
	checkRequest(t, errInvalidPrevHash, &prepareRequest{prevHash: random.Uint256()})
	checkRequest(t, errInvalidStateRoot, &prepareRequest{
		stateRootEnabled: true,
		prevHash:         prevHash,
	})

	sr, err := srv.Chain.GetStateRoot(srv.dbft.BlockIndex - 1)
	require.NoError(t, err)

	checkRequest(t, errInvalidTransactionsCount, &prepareRequest{stateRootEnabled: true,
		prevHash:          prevHash,
		stateRoot:         sr.Root,
		transactionHashes: make([]util.Uint256, srv.ProtocolConfiguration.MaxTransactionsPerBlock+1),
	})

	checkRequest(t, nil, &prepareRequest{
		stateRootEnabled: true,
		prevHash:         prevHash,
		stateRoot:        sr.Root,
	})
}

func TestService_OnPayload(t *testing.T) {
	srv := newTestService(t)
	// This test directly reads things from srv.messages that normally
	// is read by internal goroutine started with Start(). So let's
	// pretend we really did start already.
	srv.started.Store(true)

	priv, _ := getTestValidator(1)
	p := new(Payload)
	p.message.ValidatorIndex = 1
	p.payload = &prepareRequest{}
	p.encodeData()

	// sender is invalid
	require.NoError(t, srv.OnPayload(&p.Extensible))
	shouldNotReceive(t, srv.messages)

	p = new(Payload)
	p.message.ValidatorIndex = 1
	p.Sender = priv.GetScriptHash()
	p.payload = &prepareRequest{}
	require.NoError(t, p.Sign(priv))
	require.NoError(t, srv.OnPayload(&p.Extensible))
	shouldReceive(t, srv.messages)
}

func TestVerifyBlock(t *testing.T) {
	srv := newTestService(t)

	bc := srv.Chain.(*core.Blockchain)
	srv.lastTimestamp = 1
	t.Run("good empty", func(t *testing.T) {
		b := testchain.NewBlock(t, bc, 1, 0)
		require.True(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("good pooled tx", func(t *testing.T) {
		tx := transaction.New([]byte{byte(opcode.RET)}, 100000)
		tx.ValidUntilBlock = 1
		addSender(t, tx)
		signTx(t, srv.Chain, tx)
		require.NoError(t, srv.Chain.PoolTx(tx))
		b := testchain.NewBlock(t, bc, 1, 0, tx)
		require.True(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("good non-pooled tx", func(t *testing.T) {
		tx := transaction.New([]byte{byte(opcode.RET)}, 100000)
		tx.ValidUntilBlock = 1
		addSender(t, tx)
		signTx(t, srv.Chain, tx)
		b := testchain.NewBlock(t, bc, 1, 0, tx)
		require.True(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("good conflicting tx", func(t *testing.T) {
		initGAS := srv.Chain.GetConfig().InitialGASSupply
		tx1 := transaction.New([]byte{byte(opcode.RET)}, 100000)
		tx1.NetworkFee = int64(initGAS)/2 + 1
		tx1.ValidUntilBlock = 1
		addSender(t, tx1)
		signTx(t, srv.Chain, tx1)
		tx2 := transaction.New([]byte{byte(opcode.RET)}, 100000)
		tx2.NetworkFee = int64(initGAS)/2 + 1
		tx2.ValidUntilBlock = 1
		addSender(t, tx2)
		signTx(t, srv.Chain, tx2)
		require.NoError(t, srv.Chain.PoolTx(tx1))
		require.Error(t, srv.Chain.PoolTx(tx2))
		b := testchain.NewBlock(t, bc, 1, 0, tx2)
		require.True(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("bad old", func(t *testing.T) {
		b := testchain.NewBlock(t, bc, 1, 0)
		b.Index = srv.Chain.BlockHeight()
		require.False(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("bad big size", func(t *testing.T) {
		script := make([]byte, int(srv.ProtocolConfiguration.MaxBlockSize))
		script[0] = byte(opcode.RET)
		tx := transaction.New(script, 100000)
		tx.ValidUntilBlock = 1
		addSender(t, tx)
		signTx(t, srv.Chain, tx)
		b := testchain.NewBlock(t, bc, 1, 0, tx)
		require.False(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("bad timestamp", func(t *testing.T) {
		b := testchain.NewBlock(t, bc, 1, 0)
		b.Timestamp = srv.lastTimestamp - 1
		require.False(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("bad tx", func(t *testing.T) {
		tx := transaction.New([]byte{byte(opcode.RET)}, 100000)
		tx.ValidUntilBlock = 1
		addSender(t, tx)
		signTx(t, srv.Chain, tx)
		tx.Scripts[0].InvocationScript[16] = ^tx.Scripts[0].InvocationScript[16]
		b := testchain.NewBlock(t, bc, 1, 0, tx)
		require.False(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("bad big sys fee", func(t *testing.T) {
		txes := make([]*transaction.Transaction, 2)
		for i := range txes {
			txes[i] = transaction.New([]byte{byte(opcode.RET)}, srv.ProtocolConfiguration.MaxBlockSystemFee/2+1)
			txes[i].ValidUntilBlock = 1
			addSender(t, txes[i])
			signTx(t, srv.Chain, txes[i])
		}
		b := testchain.NewBlock(t, bc, 1, 0, txes...)
		require.False(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
}

func shouldReceive(t *testing.T, ch chan Payload) {
	select {
	case <-ch:
	default:
		require.Fail(t, "missing expected message")
	}
}

func shouldNotReceive(t *testing.T, ch chan Payload) {
	select {
	case <-ch:
		require.Fail(t, "unexpected message receive")
	default:
	}
}

func newTestServiceWithState(t *testing.T, stateRootInHeader bool) *service {
	return newTestServiceWithChain(t, newTestChain(t, stateRootInHeader))
}

func newTestService(t *testing.T) *service {
	return newTestServiceWithState(t, false)
}

func newTestServiceWithChain(t *testing.T, bc *core.Blockchain) *service {
	srv, err := NewService(Config{
		Logger:                zaptest.NewLogger(t),
		Broadcast:             func(*npayload.Extensible) {},
		Chain:                 bc,
		BlockQueue:            testBlockQueuer{bc: bc},
		ProtocolConfiguration: bc.GetConfig().ProtocolConfiguration,
		RequestTx:             func(...util.Uint256) {},
		StopTxFlow:            func() {},
		Wallet: config.Wallet{
			Path:     "./testdata/wallet1.json",
			Password: "one",
		},
	})
	require.NoError(t, err)

	return srv.(*service)
}

type testBlockQueuer struct {
	bc *core.Blockchain
}

var _ = BlockQueuer(testBlockQueuer{})

// PutBlock implements BlockQueuer interface.
func (bq testBlockQueuer) Put(b *coreb.Block) error {
	return bq.bc.AddBlock(b)
}

func getTestValidator(i int) (*keys.PrivateKey, *keys.PublicKey) {
	key := testchain.PrivateKey(i)
	return key, key.PublicKey()
}

func newSingleTestChain(t *testing.T) *core.Blockchain {
	configPath := "../../config/protocol.unit_testnet.single.yml"
	cfg, err := config.LoadFile(configPath)
	require.NoError(t, err, "could not load config")

	chain, err := core.NewBlockchain(storage.NewMemoryStore(), cfg.Blockchain(), zaptest.NewLogger(t))
	require.NoError(t, err, "could not create chain")

	go chain.Run()
	t.Cleanup(chain.Close)
	return chain
}

func newTestChain(t *testing.T, stateRootInHeader bool) *core.Blockchain {
	unitTestNetCfg, err := config.Load("../../config", netmode.UnitTestNet)
	require.NoError(t, err)
	unitTestNetCfg.ProtocolConfiguration.StateRootInHeader = stateRootInHeader

	chain, err := core.NewBlockchain(storage.NewMemoryStore(), unitTestNetCfg.Blockchain(), zaptest.NewLogger(t))
	require.NoError(t, err)

	go chain.Run()
	t.Cleanup(chain.Close)
	return chain
}

var neoOwner = testchain.MultisigScriptHash()

func addSender(t *testing.T, txs ...*transaction.Transaction) {
	for _, tx := range txs {
		tx.Signers = []transaction.Signer{
			{
				Account: neoOwner,
			},
		}
	}
}

func signTx(t *testing.T, bc Ledger, txs ...*transaction.Transaction) {
	validators := make([]*keys.PublicKey, 4)
	privNetKeys := make([]*keys.PrivateKey, 4)
	for i := range 4 {
		privNetKeys[i] = testchain.PrivateKey(i)
		validators[i] = privNetKeys[i].PublicKey()
	}
	privNetKeys = privNetKeys[:3]
	rawScript, err := smartcontract.CreateMultiSigRedeemScript(3, validators)
	require.NoError(t, err)
	for _, tx := range txs {
		size := io.GetVarSize(tx)
		netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), rawScript)
		tx.NetworkFee += +netFee
		size += sizeDelta
		tx.NetworkFee += int64(size)*bc.FeePerByte() + bc.CalculateAttributesFee(tx)

		buf := io.NewBufBinWriter()
		for _, key := range privNetKeys {
			signature := key.SignHashable(uint32(testchain.Network()), tx)
			emit.Bytes(buf.BinWriter, signature)
		}

		tx.Scripts = []transaction.Witness{{
			InvocationScript:   buf.Bytes(),
			VerificationScript: rawScript,
		}}
	}
}
