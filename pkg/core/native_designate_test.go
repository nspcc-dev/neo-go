package core

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func (bc *Blockchain) setNodesByRole(t *testing.T, ok bool, r native.Role, nodes keys.PublicKeys) {
	w := io.NewBufBinWriter()
	for _, pub := range nodes {
		emit.Bytes(w.BinWriter, pub.Bytes())
	}
	emit.Int(w.BinWriter, int64(len(nodes)))
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.Int(w.BinWriter, int64(r))
	emit.Int(w.BinWriter, 2)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.String(w.BinWriter, "designateAsRole")
	emit.AppCall(w.BinWriter, bc.contracts.Designate.Hash)
	require.NoError(t, w.Err)
	tx := transaction.New(netmode.UnitTestNet, w.Bytes(), 0)
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
	require.NoError(t, signTx(bc, tx))
	tx.Scripts = append(tx.Scripts, transaction.Witness{
		InvocationScript:   testchain.SignCommittee(tx.GetSignedPart()),
		VerificationScript: testchain.CommitteeVerificationScript(),
	})
	require.NoError(t, bc.AddBlock(bc.newBlock(tx)))

	aer, err := bc.GetAppExecResult(tx.Hash())
	require.NoError(t, err)
	if ok {
		require.Equal(t, vm.HaltState, aer.VMState)
	} else {
		require.Equal(t, vm.FaultState, aer.VMState)
	}
}

func TestDesignate_DesignateAsRoleTx(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pubs := keys.PublicKeys{priv.PublicKey()}

	bc.setNodesByRole(t, false, 0xFF, pubs)
	bc.setNodesByRole(t, true, native.RoleOracle, pubs)
}

func TestDesignate_DesignateAsRole(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	des := bc.contracts.Designate
	tx := transaction.New(netmode.UnitTestNet, []byte{}, 0)
	ic := bc.newInteropContext(trigger.OnPersist, bc.dao, nil, tx)
	ic.SpawnVM()
	ic.VM.LoadScript([]byte{byte(opcode.RET)})

	pubs, err := des.GetDesignatedByRole(bc.dao, 0xFF)
	require.True(t, errors.Is(err, native.ErrInvalidRole), "got: %v", err)

	pubs, err = des.GetDesignatedByRole(bc.dao, native.RoleOracle)
	require.NoError(t, err)
	require.Equal(t, 0, len(pubs))

	err = des.DesignateAsRole(ic, native.RoleOracle, keys.PublicKeys{})
	require.True(t, errors.Is(err, native.ErrEmptyNodeList), "got: %v", err)

	err = des.DesignateAsRole(ic, native.RoleOracle, make(keys.PublicKeys, 32+1))
	require.True(t, errors.Is(err, native.ErrLargeNodeList), "got: %v", err)

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()

	err = des.DesignateAsRole(ic, 0xFF, keys.PublicKeys{pub})
	require.True(t, errors.Is(err, native.ErrInvalidRole), "got: %v", err)

	err = des.DesignateAsRole(ic, native.RoleOracle, keys.PublicKeys{pub})
	require.True(t, errors.Is(err, native.ErrInvalidWitness), "got: %v", err)

	setSigner(tx, testchain.CommitteeScriptHash())
	err = des.DesignateAsRole(ic, native.RoleOracle, keys.PublicKeys{pub})
	require.NoError(t, err)
	require.NoError(t, des.OnPersistEnd(ic.DAO))

	pubs, err = des.GetDesignatedByRole(ic.DAO, native.RoleOracle)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub}, pubs)

	pubs, err = des.GetDesignatedByRole(ic.DAO, native.RoleStateValidator)
	require.NoError(t, err)
	require.Equal(t, 0, len(pubs))

	// Set another role.
	_, err = keys.NewPrivateKey()
	require.NoError(t, err)
	pub1 := priv.PublicKey()
	err = des.DesignateAsRole(ic, native.RoleStateValidator, keys.PublicKeys{pub1})
	require.NoError(t, err)
	require.NoError(t, des.OnPersistEnd(ic.DAO))

	pubs, err = des.GetDesignatedByRole(ic.DAO, native.RoleOracle)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub}, pubs)

	pubs, err = des.GetDesignatedByRole(ic.DAO, native.RoleStateValidator)
	require.NoError(t, err)
	require.Equal(t, keys.PublicKeys{pub1}, pubs)
}
