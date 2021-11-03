package chain

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/stretchr/testify/require"
)

// TestNewMulti checks that transaction and block is signed correctly for multi-node setup.
func TestNewMulti(t *testing.T) {
	bc, vAcc, cAcc := NewMulti(t)
	e := neotest.NewExecutor(t, bc, vAcc, cAcc)

	require.NotEqual(t, vAcc.ScriptHash(), cAcc.ScriptHash())

	const amount = int64(10_0000_0000)

	c := e.CommitteeInvoker(bc.UtilityTokenHash()).WithSigners(vAcc)
	c.Invoke(t, true, "transfer", e.Validator.ScriptHash(), e.Committee.ScriptHash(), amount, nil)
}
