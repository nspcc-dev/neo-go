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
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func (bc *Blockchain) setNodesByRole(t *testing.T, ok bool, r noderoles.Role, nodes keys.PublicKeys) {
	w := io.NewBufBinWriter()
	for _, pub := range nodes {
		emit.Bytes(w.BinWriter, pub.Bytes())
	}
	emit.Int(w.BinWriter, int64(len(nodes)))
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.Int(w.BinWriter, int64(r))
	emit.Int(w.BinWriter, 2)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.AppCallNoArgs(w.BinWriter, bc.contracts.Designate.Hash, "designateAsRole", callflag.All)
	require.NoError(t, w.Err)
	tx := transaction.New(w.Bytes(), 0)
	tx.NetworkFee = 10_000_000
	tx.SystemFee = 10_000_000
	tx.ValidUntilBlock = 100
	tx.Signers = []transaction.Signer{
		{
			Account: testchain.MultisigScriptHash(),
			Scopes:  transaction.None,
		},
		{
			Account: testchain.CommitteeScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
	}
	require.NoError(t, testchain.SignTx(bc, tx))
	tx.Scripts = append(tx.Scripts, transaction.Witness{
		InvocationScript:   testchain.SignCommittee(tx),
		VerificationScript: testchain.CommitteeVerificationScript(),
	})
	require.NoError(t, bc.AddBlock(bc.newBlock(tx)))

	aer, err := bc.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 1, len(aer))
	if ok {
		require.Equal(t, vm.HaltState, aer[0].VMState)
		require.Equal(t, 1, len(aer[0].Events))

		ev := aer[0].Events[0]
		require.Equal(t, bc.contracts.Designate.Hash, ev.ScriptHash)
		require.Equal(t, native.DesignationEventName, ev.Name)
		require.Equal(t, []stackitem.Item{
			stackitem.Make(int64(r)),
			stackitem.Make(bc.BlockHeight()),
		}, ev.Item.Value().([]stackitem.Item))
	} else {
		require.Equal(t, vm.FaultState, aer[0].VMState)
		require.Equal(t, 0, len(aer[0].Events))
	}
}

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
