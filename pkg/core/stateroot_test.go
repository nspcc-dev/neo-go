package core

import (
	"errors"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
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
)

func testSignStateRoot(t *testing.T, r *state.MPTRoot, pubs keys.PublicKeys, accs ...*wallet.Account) []byte {
	n := smartcontract.GetMajorityHonestNodeCount(len(accs))
	w := io.NewBufBinWriter()
	for i := 0; i < n; i++ {
		sig := accs[i].PrivateKey().SignHash(r.GetSignedHash())
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
	bc.setNodesByRole(t, true, native.RoleStateValidator, pubs)
	updateIndex := bc.BlockHeight()
	transferTokenFromMultisigAccount(t, bc, h, bc.contracts.GAS.Hash, 1_0000_0000)

	srv, err := stateroot.New(bc.GetStateModule())
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
		bc.setNodesByRole(t, true, native.RoleStateValidator, pubs)
		transferTokenFromMultisigAccount(t, bc, h, bc.contracts.GAS.Hash, 1_0000_0000)

		_, err := persistBlock(bc)
		require.NoError(t, err)
		srv, err := stateroot.New(bc.GetStateModule())
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
