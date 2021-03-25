package core

import (
	"errors"
	"os"
	"path"
	"sort"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/services/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap/zaptest"
)

func testSignStateRoot(t *testing.T, r *state.MPTRoot, pubs keys.PublicKeys, accs ...*wallet.Account) []byte {
	n := smartcontract.GetMajorityHonestNodeCount(len(accs))
	w := io.NewBufBinWriter()
	for i := 0; i < n; i++ {
		sig := accs[i].PrivateKey().SignHashable(uint32(netmode.UnitTestNet), r)
		emit.Bytes(w.BinWriter, sig)
	}
	require.NoError(t, w.Err)

	script, err := smartcontract.CreateMajorityMultiSigRedeemScript(pubs.Copy())
	require.NoError(t, err)
	r.Witness = &transaction.Witness{
		VerificationScript: script,
		InvocationScript:   w.Bytes(),
	}
	data, err := testserdes.EncodeBinary(stateroot.NewMessage(stateroot.RootT, r))
	require.NoError(t, err)
	return data
}

func newMajorityMultisigWithGAS(t *testing.T, n int) (util.Uint160, keys.PublicKeys, []*wallet.Account) {
	accs := make([]*wallet.Account, n)
	for i := range accs {
		acc, err := wallet.NewAccount()
		require.NoError(t, err)
		accs[i] = acc
	}
	sort.Slice(accs, func(i, j int) bool {
		pi := accs[i].PrivateKey().PublicKey()
		pj := accs[j].PrivateKey().PublicKey()
		return pi.Cmp(pj) == -1
	})
	pubs := make(keys.PublicKeys, n)
	for i := range pubs {
		pubs[i] = accs[i].PrivateKey().PublicKey()
	}
	script, err := smartcontract.CreateMajorityMultiSigRedeemScript(pubs)
	require.NoError(t, err)
	return hash.Hash160(script), pubs, accs
}

func TestStateRoot(t *testing.T) {
	bc := newTestChain(t)

	h, pubs, accs := newMajorityMultisigWithGAS(t, 2)
	bc.setNodesByRole(t, true, noderoles.StateValidator, pubs)
	updateIndex := bc.BlockHeight()
	transferTokenFromMultisigAccount(t, bc, h, bc.contracts.GAS.Hash, 1_0000_0000)

	tmpDir := path.Join(os.TempDir(), "neogo.initsnz")
	require.NoError(t, os.Mkdir(tmpDir, os.ModePerm))
	defer os.RemoveAll(tmpDir)
	w := createAndWriteWallet(t, accs[0], path.Join(tmpDir, "w"), "pass")
	cfg := createStateRootConfig(w.Path(), "pass")
	srv, err := stateroot.New(cfg, zaptest.NewLogger(t), bc)
	require.NoError(t, err)
	require.EqualValues(t, 0, srv.CurrentValidatedHeight())
	r, err := srv.GetStateRoot(bc.BlockHeight())
	require.NoError(t, err)
	require.Equal(t, r.Root, srv.CurrentLocalStateRoot())

	t.Run("invalid message", func(t *testing.T) {
		require.Error(t, srv.OnPayload(&payload.Extensible{Data: []byte{42}}))
		require.EqualValues(t, 0, srv.CurrentValidatedHeight())
	})
	t.Run("drop zero index", func(t *testing.T) {
		r, err := srv.GetStateRoot(0)
		require.NoError(t, err)
		data, err := testserdes.EncodeBinary(stateroot.NewMessage(stateroot.RootT, r))
		require.NoError(t, err)
		require.NoError(t, srv.OnPayload(&payload.Extensible{Data: data}))
		require.EqualValues(t, 0, srv.CurrentValidatedHeight())
	})
	t.Run("invalid height", func(t *testing.T) {
		r, err := srv.GetStateRoot(1)
		require.NoError(t, err)
		r.Index = 10
		data := testSignStateRoot(t, r, pubs, accs...)
		require.Error(t, srv.OnPayload(&payload.Extensible{Data: data}))
		require.EqualValues(t, 0, srv.CurrentValidatedHeight())
	})
	t.Run("invalid signer", func(t *testing.T) {
		accInv, err := wallet.NewAccount()
		require.NoError(t, err)
		pubs := keys.PublicKeys{accInv.PrivateKey().PublicKey()}
		require.NoError(t, accInv.ConvertMultisig(1, pubs))
		transferTokenFromMultisigAccount(t, bc, accInv.Contract.ScriptHash(), bc.contracts.GAS.Hash, 1_0000_0000)
		r, err := srv.GetStateRoot(1)
		require.NoError(t, err)
		data := testSignStateRoot(t, r, pubs, accInv)
		err = srv.OnPayload(&payload.Extensible{Data: data})
		require.True(t, errors.Is(err, ErrWitnessHashMismatch), "got: %v", err)
		require.EqualValues(t, 0, srv.CurrentValidatedHeight())
	})

	r, err = srv.GetStateRoot(updateIndex + 1)
	require.NoError(t, err)
	data := testSignStateRoot(t, r, pubs, accs...)
	require.NoError(t, srv.OnPayload(&payload.Extensible{Data: data}))
	require.EqualValues(t, 2, srv.CurrentValidatedHeight())

	r, err = srv.GetStateRoot(updateIndex + 1)
	require.NoError(t, err)
	require.NotNil(t, r.Witness)
	require.Equal(t, h, r.Witness.ScriptHash())
}

func TestStateRootInitNonZeroHeight(t *testing.T) {
	st := memoryStore{storage.NewMemoryStore()}
	h, pubs, accs := newMajorityMultisigWithGAS(t, 2)

	var root util.Uint256
	t.Run("init", func(t *testing.T) { // this is in a separate test to do proper cleanup
		bc := newTestChainWithCustomCfgAndStore(t, st, nil)
		bc.setNodesByRole(t, true, noderoles.StateValidator, pubs)
		transferTokenFromMultisigAccount(t, bc, h, bc.contracts.GAS.Hash, 1_0000_0000)

		_, err := persistBlock(bc)
		require.NoError(t, err)
		tmpDir := path.Join(os.TempDir(), "neogo.initsnz")
		require.NoError(t, os.Mkdir(tmpDir, os.ModePerm))
		defer os.RemoveAll(tmpDir)
		w := createAndWriteWallet(t, accs[0], path.Join(tmpDir, "w"), "pass")
		cfg := createStateRootConfig(w.Path(), "pass")
		srv, err := stateroot.New(cfg, zaptest.NewLogger(t), bc)
		require.NoError(t, err)
		r, err := srv.GetStateRoot(2)
		require.NoError(t, err)
		data := testSignStateRoot(t, r, pubs, accs...)
		require.NoError(t, srv.OnPayload(&payload.Extensible{Data: data}))
		require.EqualValues(t, 2, srv.CurrentValidatedHeight())
		root = srv.CurrentLocalStateRoot()
	})

	bc2 := newTestChainWithCustomCfgAndStore(t, st, nil)
	srv := bc2.GetStateModule()
	require.EqualValues(t, 2, srv.CurrentValidatedHeight())
	require.Equal(t, root, srv.CurrentLocalStateRoot())
}

func createAndWriteWallet(t *testing.T, acc *wallet.Account, path, password string) *wallet.Wallet {
	w, err := wallet.NewWallet(path)
	require.NoError(t, err)
	require.NoError(t, acc.Encrypt(password))
	w.AddAccount(acc)
	require.NoError(t, w.Save())
	w.Close()
	return w
}

func createStateRootConfig(walletPath, password string) config.StateRoot {
	return config.StateRoot{
		Enabled: true,
		UnlockWallet: config.Wallet{
			Path:     walletPath,
			Password: password,
		},
	}
}

func TestStateRootFull(t *testing.T) {
	tmpDir := path.Join(os.TempDir(), "neogo.stateroot4")
	require.NoError(t, os.Mkdir(tmpDir, os.ModePerm))
	defer os.RemoveAll(tmpDir)

	bc := newTestChain(t)

	h, pubs, accs := newMajorityMultisigWithGAS(t, 2)
	w := createAndWriteWallet(t, accs[1], path.Join(tmpDir, "wallet2"), "two")
	cfg := createStateRootConfig(w.Path(), "two")
	srv, err := stateroot.New(cfg, zaptest.NewLogger(t), bc)
	require.NoError(t, err)
	srv.Run()
	t.Cleanup(srv.Shutdown)

	var lastValidated atomic.Value
	var lastHeight atomic.Uint32
	srv.SetRelayCallback(func(ep *payload.Extensible) {
		lastHeight.Store(ep.ValidBlockStart)
		lastValidated.Store(ep)
	})

	bc.setNodesByRole(t, true, noderoles.StateValidator, pubs)
	transferTokenFromMultisigAccount(t, bc, h, bc.contracts.GAS.Hash, 1_0000_0000)
	require.Eventually(t, func() bool { return lastHeight.Load() == 2 }, time.Second, time.Millisecond)
	checkVoteBroadcasted(t, bc, lastValidated.Load().(*payload.Extensible), 2, 1)
	_, err = persistBlock(bc)
	require.Eventually(t, func() bool { return lastHeight.Load() == 3 }, time.Second, time.Millisecond)
	checkVoteBroadcasted(t, bc, lastValidated.Load().(*payload.Extensible), 3, 1)

	r, err := srv.GetStateRoot(2)
	require.NoError(t, err)
	require.NoError(t, srv.AddSignature(2, 0, accs[0].PrivateKey().SignHashable(uint32(netmode.UnitTestNet), r)))
	require.NotNil(t, lastValidated.Load().(*payload.Extensible))

	msg := new(stateroot.Message)
	require.NoError(t, testserdes.DecodeBinary(lastValidated.Load().(*payload.Extensible).Data, msg))
	require.Equal(t, stateroot.RootT, msg.Type)

	actual := msg.Payload.(*state.MPTRoot)
	require.Equal(t, r.Index, actual.Index)
	require.Equal(t, r.Version, actual.Version)
	require.Equal(t, r.Root, actual.Root)
}

func checkVoteBroadcasted(t *testing.T, bc *Blockchain, p *payload.Extensible,
	height uint32, valIndex byte) {
	require.NotNil(t, p)
	m := new(stateroot.Message)
	require.NoError(t, testserdes.DecodeBinary(p.Data, m))
	require.Equal(t, stateroot.VoteT, m.Type)
	vote := m.Payload.(*stateroot.Vote)

	srv := bc.GetStateModule()
	r, err := srv.GetStateRoot(bc.BlockHeight())
	require.NoError(t, err)
	require.Equal(t, height, vote.Height)
	require.Equal(t, int32(valIndex), vote.ValidatorIndex)

	pubs, _, err := bc.contracts.Designate.GetDesignatedByRole(bc.dao, noderoles.StateValidator, bc.BlockHeight())
	require.True(t, len(pubs) > int(valIndex))
	require.True(t, pubs[valIndex].VerifyHashable(vote.Signature, uint32(netmode.UnitTestNet), r))
}
