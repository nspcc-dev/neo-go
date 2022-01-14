package native_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
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
