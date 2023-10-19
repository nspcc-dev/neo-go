package native_test

import (
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newDesignateClient(t *testing.T) *neotest.ContractInvoker {
	return newNativeClient(t, nativenames.Designation)
}

func TestDesignate_DesignateAsRole(t *testing.T) {
	c := newDesignateClient(t)
	e := c.Executor
	designateInvoker := c.WithSigners(c.Committee)

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pubs := keys.PublicKeys{priv.PublicKey()}

	setNodesByRole(t, designateInvoker, false, 0xFF, pubs)
	setNodesByRole(t, designateInvoker, true, noderoles.Oracle, pubs)
	index := e.Chain.BlockHeight() + 1
	checkNodeRoles(t, designateInvoker, false, 0xFF, 0, nil)
	checkNodeRoles(t, designateInvoker, false, noderoles.Oracle, 100500, nil)
	checkNodeRoles(t, designateInvoker, true, noderoles.Oracle, 0, keys.PublicKeys{}) // returns an empty list
	checkNodeRoles(t, designateInvoker, true, noderoles.Oracle, index, pubs)          // returns pubs

	priv1, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pubs = keys.PublicKeys{priv1.PublicKey()}
	setNodesByRole(t, designateInvoker, true, noderoles.StateValidator, pubs)
	checkNodeRoles(t, designateInvoker, true, noderoles.StateValidator, e.Chain.BlockHeight()+1, pubs)

	t.Run("neofs", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		pubs = keys.PublicKeys{priv.PublicKey()}
		setNodesByRole(t, designateInvoker, true, noderoles.NeoFSAlphabet, pubs)
		checkNodeRoles(t, designateInvoker, true, noderoles.NeoFSAlphabet, e.Chain.BlockHeight()+1, pubs)
	})
}

type dummyOracle struct {
	updateNodes func(k keys.PublicKeys)
}

// AddRequests processes new requests.
func (o *dummyOracle) AddRequests(map[uint64]*state.OracleRequest) {
}

// RemoveRequests removes already processed requests.
func (o *dummyOracle) RemoveRequests([]uint64) {
	panic("TODO")
}

// UpdateOracleNodes updates oracle nodes.
func (o *dummyOracle) UpdateOracleNodes(k keys.PublicKeys) {
	if o.updateNodes != nil {
		o.updateNodes(k)
		return
	}
	panic("TODO")
}

// UpdateNativeContract updates oracle contract native script and hash.
func (o *dummyOracle) UpdateNativeContract([]byte, []byte, util.Uint160, int) {
}

// Start runs oracle module.
func (o *dummyOracle) Start() {
	panic("TODO")
}

// Shutdown shutdowns oracle module.
func (o *dummyOracle) Shutdown() {
	panic("TODO")
}

func TestDesignate_Cache(t *testing.T) {
	c := newDesignateClient(t)
	e := c.Executor
	designateInvoker := c.WithSigners(c.Committee)
	r := int64(noderoles.Oracle)
	var (
		updatedNodes keys.PublicKeys
		updateCalled bool
	)
	oracleServ := &dummyOracle{
		updateNodes: func(k keys.PublicKeys) {
			updatedNodes = k
			updateCalled = true
		},
	}
	privGood, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pubsGood := []any{privGood.PublicKey().Bytes()}

	privBad, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pubsBad := []any{privBad.PublicKey().Bytes()}

	// Firstly, designate good Oracle node and check that OracleService callback was called during PostPersist.
	e.Chain.SetOracle(oracleServ)
	txDesignateGood := designateInvoker.PrepareInvoke(t, "designateAsRole", r, pubsGood)
	e.AddNewBlock(t, txDesignateGood)
	e.CheckHalt(t, txDesignateGood.Hash(), stackitem.Null{})
	require.True(t, updateCalled)
	require.Equal(t, keys.PublicKeys{privGood.PublicKey()}, updatedNodes)
	updatedNodes = nil
	updateCalled = false

	// Check designated node in a separate block.
	checkNodeRoles(t, designateInvoker, true, noderoles.Oracle, e.Chain.BlockHeight()+1, keys.PublicKeys{privGood.PublicKey()})

	// Designate privBad as oracle node and abort the transaction. Designation cache changes
	// shouldn't be persisted to the contract and no notification should be sent.
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, designateInvoker.Hash, "designateAsRole", callflag.All, int64(r), pubsBad)
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	require.NoError(t, w.Err)
	script := w.Bytes()

	designateInvoker.InvokeScriptCheckFAULT(t, script, designateInvoker.Signers, "ABORT")
	require.Nil(t, updatedNodes)
	require.False(t, updateCalled)
}

func TestDesignate_GenesisRolesExtension(t *testing.T) {
	pk1, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pk2, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pubs := keys.PublicKeys{pk1.PublicKey(), pk2.PublicKey()}

	bc, acc := chain.NewSingleWithCustomConfig(t, func(blockchain *config.Blockchain) {
		blockchain.Genesis.Roles = map[noderoles.Role]keys.PublicKeys{
			noderoles.StateValidator: pubs,
		}
	})
	e := neotest.NewExecutor(t, bc, acc, acc)
	c := e.CommitteeInvoker(e.NativeHash(t, nativenames.Designation))

	// Check designated node in a separate block.
	sort.Sort(pubs)
	checkNodeRoles(t, c, true, noderoles.StateValidator, e.Chain.BlockHeight()+1, pubs)
}
