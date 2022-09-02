package stateroot_test

import (
	"crypto/elliptic"
	"errors"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	corestate "github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/roles"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/services/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
	r.Witness = []transaction.Witness{{
		VerificationScript: script,
		InvocationScript:   w.Bytes(),
	}}
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
		pi := accs[i].PublicKey()
		pj := accs[j].PublicKey()
		return pi.Cmp(pj) == -1
	})
	pubs := make(keys.PublicKeys, n)
	for i := range pubs {
		pubs[i] = accs[i].PublicKey()
	}
	script, err := smartcontract.CreateMajorityMultiSigRedeemScript(pubs)
	require.NoError(t, err)
	return hash.Hash160(script), pubs, accs
}

func TestStateRoot(t *testing.T) {
	bc, validator, committee := chain.NewMulti(t)
	e := neotest.NewExecutor(t, bc, validator, committee)
	designationSuperInvoker := e.NewInvoker(e.NativeHash(t, nativenames.Designation), validator, committee)
	gasValidatorInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Gas))

	h, pubs, accs := newMajorityMultisigWithGAS(t, 2)
	validatorNodes := []interface{}{pubs[0].Bytes(), pubs[1].Bytes()}
	designationSuperInvoker.Invoke(t, stackitem.Null{}, "designateAsRole",
		int64(roles.StateValidator), validatorNodes)
	updateIndex := bc.BlockHeight()

	gasValidatorInvoker.Invoke(t, true, "transfer", validator.ScriptHash(), h, 1_0000_0000, nil)

	tmpDir := t.TempDir()
	w := createAndWriteWallet(t, accs[0], filepath.Join(tmpDir, "w"), "pass")
	cfg := createStateRootConfig(w.Path(), "pass")
	srMod := bc.GetStateModule().(*corestate.Module) // Take full responsibility here.
	srv, err := stateroot.New(cfg, srMod, zaptest.NewLogger(t), bc, nil)
	require.NoError(t, err)
	require.EqualValues(t, 0, bc.GetStateModule().CurrentValidatedHeight())
	r, err := bc.GetStateModule().GetStateRoot(bc.BlockHeight())
	require.NoError(t, err)
	require.Equal(t, r.Root, bc.GetStateModule().CurrentLocalStateRoot())

	t.Run("invalid message", func(t *testing.T) {
		require.Error(t, srv.OnPayload(&payload.Extensible{Data: []byte{42}}))
		require.EqualValues(t, 0, bc.GetStateModule().CurrentValidatedHeight())
	})
	t.Run("drop zero index", func(t *testing.T) {
		r, err := bc.GetStateModule().GetStateRoot(0)
		require.NoError(t, err)
		data, err := testserdes.EncodeBinary(stateroot.NewMessage(stateroot.RootT, r))
		require.NoError(t, err)
		require.NoError(t, srv.OnPayload(&payload.Extensible{Data: data}))
		require.EqualValues(t, 0, bc.GetStateModule().CurrentValidatedHeight())
	})
	t.Run("invalid height", func(t *testing.T) {
		r, err := bc.GetStateModule().GetStateRoot(1)
		require.NoError(t, err)
		r.Index = 10
		data := testSignStateRoot(t, r, pubs, accs...)
		require.Error(t, srv.OnPayload(&payload.Extensible{Data: data}))
		require.EqualValues(t, 0, bc.GetStateModule().CurrentValidatedHeight())
	})
	t.Run("invalid signer", func(t *testing.T) {
		accInv, err := wallet.NewAccount()
		require.NoError(t, err)
		pubs := keys.PublicKeys{accInv.PublicKey()}
		require.NoError(t, accInv.ConvertMultisig(1, pubs))
		gasValidatorInvoker.Invoke(t, true, "transfer", validator.ScriptHash(), accInv.Contract.ScriptHash(), 1_0000_0000, nil)
		r, err := bc.GetStateModule().GetStateRoot(1)
		require.NoError(t, err)
		data := testSignStateRoot(t, r, pubs, accInv)
		err = srv.OnPayload(&payload.Extensible{Data: data})
		require.True(t, errors.Is(err, core.ErrWitnessHashMismatch), "got: %v", err)
		require.EqualValues(t, 0, bc.GetStateModule().CurrentValidatedHeight())
	})

	r, err = bc.GetStateModule().GetStateRoot(updateIndex + 1)
	require.NoError(t, err)
	data := testSignStateRoot(t, r, pubs, accs...)
	require.NoError(t, srv.OnPayload(&payload.Extensible{Data: data}))
	require.EqualValues(t, 2, bc.GetStateModule().CurrentValidatedHeight())

	r, err = bc.GetStateModule().GetStateRoot(updateIndex + 1)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(r.Witness))
	require.Equal(t, h, r.Witness[0].ScriptHash())
}

type memoryStore struct {
	*storage.MemoryStore
}

func (memoryStore) Close() error { return nil }

func TestStateRootInitNonZeroHeight(t *testing.T) {
	st := memoryStore{storage.NewMemoryStore()}
	h, pubs, accs := newMajorityMultisigWithGAS(t, 2)

	var root util.Uint256
	t.Run("init", func(t *testing.T) { // this is in a separate test to do proper cleanup
		bc, validator, committee := chain.NewMultiWithCustomConfigAndStore(t, nil, st, true)
		e := neotest.NewExecutor(t, bc, validator, committee)
		designationSuperInvoker := e.NewInvoker(e.NativeHash(t, nativenames.Designation), validator, committee)
		gasValidatorInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Gas))

		validatorNodes := []interface{}{pubs[0].Bytes(), pubs[1].Bytes()}
		designationSuperInvoker.Invoke(t, stackitem.Null{}, "designateAsRole",
			int64(roles.StateValidator), validatorNodes)
		gasValidatorInvoker.Invoke(t, true, "transfer", validator.ScriptHash(), h, 1_0000_0000, nil)

		tmpDir := t.TempDir()
		w := createAndWriteWallet(t, accs[0], filepath.Join(tmpDir, "w"), "pass")
		cfg := createStateRootConfig(w.Path(), "pass")
		srMod := bc.GetStateModule().(*corestate.Module) // Take full responsibility here.
		srv, err := stateroot.New(cfg, srMod, zaptest.NewLogger(t), bc, nil)
		require.NoError(t, err)
		r, err := bc.GetStateModule().GetStateRoot(2)
		require.NoError(t, err)
		data := testSignStateRoot(t, r, pubs, accs...)
		require.NoError(t, srv.OnPayload(&payload.Extensible{Data: data}))
		require.EqualValues(t, 2, bc.GetStateModule().CurrentValidatedHeight())
		root = bc.GetStateModule().CurrentLocalStateRoot()
	})

	bc2, _, _ := chain.NewMultiWithCustomConfigAndStore(t, nil, st, true)
	srv := bc2.GetStateModule()
	require.EqualValues(t, 2, srv.CurrentValidatedHeight())
	require.Equal(t, root, srv.CurrentLocalStateRoot())
}

func createAndWriteWallet(t *testing.T, acc *wallet.Account, path, password string) *wallet.Wallet {
	w, err := wallet.NewWallet(path)
	require.NoError(t, err)
	require.NoError(t, acc.Encrypt(password, w.Scrypt))
	w.AddAccount(acc)
	require.NoError(t, w.Save())
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
	tmpDir := t.TempDir()
	bc, validator, committee := chain.NewMulti(t)
	e := neotest.NewExecutor(t, bc, validator, committee)
	designationSuperInvoker := e.NewInvoker(e.NativeHash(t, nativenames.Designation), validator, committee)
	gasValidatorInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Gas))

	getDesignatedByRole := func(t *testing.T, h uint32) keys.PublicKeys {
		res, err := designationSuperInvoker.TestInvoke(t, "getDesignatedByRole", int64(noderoles.StateValidator), h)
		require.NoError(t, err)
		nodes := res.Pop().Value().([]stackitem.Item)
		pubs := make(keys.PublicKeys, len(nodes))
		for i, node := range nodes {
			pubs[i], err = keys.NewPublicKeyFromBytes(node.Value().([]byte), elliptic.P256())
			require.NoError(t, err)
		}
		return pubs
	}

	h, pubs, accs := newMajorityMultisigWithGAS(t, 2)
	w := createAndWriteWallet(t, accs[1], filepath.Join(tmpDir, "wallet2"), "two")
	cfg := createStateRootConfig(w.Path(), "two")

	var lastValidated atomic.Value
	var lastHeight atomic.Uint32
	srMod := bc.GetStateModule().(*corestate.Module) // Take full responsibility here.
	srv, err := stateroot.New(cfg, srMod, zaptest.NewLogger(t), bc, func(ep *payload.Extensible) {
		lastHeight.Store(ep.ValidBlockStart)
		lastValidated.Store(ep)
	})
	require.NoError(t, err)
	srv.Start()
	t.Cleanup(srv.Shutdown)

	validatorNodes := []interface{}{pubs[0].Bytes(), pubs[1].Bytes()}
	designationSuperInvoker.Invoke(t, stackitem.Null{}, "designateAsRole",
		int64(roles.StateValidator), validatorNodes)
	gasValidatorInvoker.Invoke(t, true, "transfer", validator.ScriptHash(), h, 1_0000_0000, nil)
	require.Eventually(t, func() bool { return lastHeight.Load() == 2 }, time.Second, time.Millisecond)
	checkVoteBroadcasted(t, bc, lastValidated.Load().(*payload.Extensible), 2, 1, getDesignatedByRole)
	e.AddNewBlock(t)
	require.Eventually(t, func() bool { return lastHeight.Load() == 3 }, time.Second, time.Millisecond)
	checkVoteBroadcasted(t, bc, lastValidated.Load().(*payload.Extensible), 3, 1, getDesignatedByRole)

	r, err := bc.GetStateModule().GetStateRoot(2)
	require.NoError(t, err)
	require.NoError(t, srv.AddSignature(2, 0, accs[0].PrivateKey().SignHashable(uint32(netmode.UnitTestNet), r)))
	require.NotNil(t, lastValidated.Load().(*payload.Extensible))

	msg := new(stateroot.Message)
	require.NoError(t, testserdes.DecodeBinary(lastValidated.Load().(*payload.Extensible).Data, msg))
	require.NotEqual(t, stateroot.RootT, msg.Type) // not a sender for this root

	r, err = bc.GetStateModule().GetStateRoot(3)
	require.NoError(t, err)
	require.Error(t, srv.AddSignature(2, 0, accs[0].PrivateKey().SignHashable(uint32(netmode.UnitTestNet), r)))
	require.NoError(t, srv.AddSignature(3, 0, accs[0].PrivateKey().SignHashable(uint32(netmode.UnitTestNet), r)))
	require.NotNil(t, lastValidated.Load().(*payload.Extensible))

	require.NoError(t, testserdes.DecodeBinary(lastValidated.Load().(*payload.Extensible).Data, msg))
	require.Equal(t, stateroot.RootT, msg.Type)

	actual := msg.Payload.(*state.MPTRoot)
	require.Equal(t, r.Index, actual.Index)
	require.Equal(t, r.Version, actual.Version)
	require.Equal(t, r.Root, actual.Root)
}

func checkVoteBroadcasted(t *testing.T, bc *core.Blockchain, p *payload.Extensible,
	height uint32, valIndex byte, getDesignatedByRole func(t *testing.T, h uint32) keys.PublicKeys) {
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

	pubs := getDesignatedByRole(t, bc.BlockHeight())
	require.True(t, len(pubs) > int(valIndex))
	require.True(t, pubs[valIndex].VerifyHashable(vote.Signature, uint32(netmode.UnitTestNet), r))
}

func TestStateroot_GetLatestStateHeight(t *testing.T) {
	bc, validators, committee := chain.NewMultiWithCustomConfig(t, func(c *config.ProtocolConfiguration) {
		c.P2PSigExtensions = true
	})
	e := neotest.NewExecutor(t, bc, validators, committee)
	basicchain.Init(t, "../../../", e)

	m := bc.GetStateModule()
	for i := uint32(0); i < bc.BlockHeight(); i++ {
		r, err := m.GetStateRoot(i)
		require.NoError(t, err)
		h, err := bc.GetStateModule().GetLatestStateHeight(r.Root)
		require.NoError(t, err, i)
		require.Equal(t, i, h)
	}
}
