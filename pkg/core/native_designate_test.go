package core

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

// Technically this test belongs to the native package, but it's so deeply tied to
// the core internals that it needs to be rewritten to be moved. So let it be
// there just to remind us about the imperfect world we live in.
func TestDesignate_DesignateAsRole(t *testing.T) {
	bc := newTestChain(t)

	des := bc.contracts.Designate
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	bl := block.New(bc.config.StateRootInHeader)
	bl.Index = bc.BlockHeight() + 1
	ic := bc.newInteropContext(trigger.OnPersist, bc.dao, bl, tx)
	ic.SpawnVM()
	ic.VM.LoadScript([]byte{byte(opcode.RET)})

	_, _, err := des.GetDesignatedByRole(bc.dao, 0xFF, 255)
	require.True(t, errors.Is(err, native.ErrInvalidRole), "got: %v", err)

	pubs, index, err := des.GetDesignatedByRole(bc.dao, noderoles.Oracle, 255)
	require.NoError(t, err)
	require.Equal(t, 0, len(pubs))
	require.Equal(t, uint32(0), index)

	err = des.DesignateAsRole(ic, noderoles.Oracle, keys.PublicKeys{})
	require.True(t, errors.Is(err, native.ErrEmptyNodeList), "got: %v", err)

	err = des.DesignateAsRole(ic, noderoles.Oracle, make(keys.PublicKeys, 32+1))
	require.True(t, errors.Is(err, native.ErrLargeNodeList), "got: %v", err)

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()

	err = des.DesignateAsRole(ic, 0xFF, keys.PublicKeys{pub})
	require.True(t, errors.Is(err, native.ErrInvalidRole), "got: %v", err)

	err = des.DesignateAsRole(ic, noderoles.Oracle, keys.PublicKeys{pub})
	require.True(t, errors.Is(err, native.ErrInvalidWitness), "got: %v", err)

	setSigner(tx, testchain.CommitteeScriptHash())
	err = des.DesignateAsRole(ic, noderoles.Oracle, keys.PublicKeys{pub})
	require.NoError(t, err)

	pubs, index, err = des.GetDesignatedByRole(ic.DAO, noderoles.Oracle, bl.Index+1)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub}, pubs)
	require.Equal(t, bl.Index+1, index)

	pubs, index, err = des.GetDesignatedByRole(ic.DAO, noderoles.StateValidator, 255)
	require.NoError(t, err)
	require.Equal(t, 0, len(pubs))
	require.Equal(t, uint32(0), index)

	// Set StateValidator role.
	_, err = keys.NewPrivateKey()
	require.NoError(t, err)
	pub1 := priv.PublicKey()
	err = des.DesignateAsRole(ic, noderoles.StateValidator, keys.PublicKeys{pub1})
	require.NoError(t, err)

	pubs, index, err = des.GetDesignatedByRole(ic.DAO, noderoles.Oracle, 255)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub}, pubs)
	require.Equal(t, bl.Index+1, index)

	pubs, index, err = des.GetDesignatedByRole(ic.DAO, noderoles.StateValidator, 255)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub1}, pubs)
	require.Equal(t, bl.Index+1, index)

	// Set P2PNotary role.
	pubs, index, err = des.GetDesignatedByRole(ic.DAO, noderoles.P2PNotary, 255)
	require.NoError(t, err)
	require.Equal(t, 0, len(pubs))
	require.Equal(t, uint32(0), index)

	err = des.DesignateAsRole(ic, noderoles.P2PNotary, keys.PublicKeys{pub1})
	require.NoError(t, err)

	pubs, index, err = des.GetDesignatedByRole(ic.DAO, noderoles.P2PNotary, 255)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub1}, pubs)
	require.Equal(t, bl.Index+1, index)
}
