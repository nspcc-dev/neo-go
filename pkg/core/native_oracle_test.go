package core

import (
	"errors"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

// getTestContractState returns test contract which uses oracles.
func getOracleContractState(h util.Uint160) *state.Contract {
	w := io.NewBufBinWriter()
	emit.Int(w.BinWriter, 5)
	emit.Opcode(w.BinWriter, opcode.PACK)
	emit.String(w.BinWriter, "request")
	emit.Bytes(w.BinWriter, h.BytesBE())
	emit.Syscall(w.BinWriter, interopnames.SystemContractCall)
	emit.Opcode(w.BinWriter, opcode.RET)

	// `handle` method aborts if len(userData) == 2
	offset := w.Len()
	emit.Opcode(w.BinWriter, opcode.OVER)
	emit.Opcode(w.BinWriter, opcode.SIZE)
	emit.Int(w.BinWriter, 2)
	emit.Instruction(w.BinWriter, opcode.JMPNE, []byte{3})
	emit.Opcode(w.BinWriter, opcode.ABORT)
	emit.Int(w.BinWriter, 4) // url, userData, code, result
	emit.Opcode(w.BinWriter, opcode.PACK)
	emit.Syscall(w.BinWriter, interopnames.SystemBinarySerialize)
	emit.String(w.BinWriter, "lastOracleResponse")
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGetContext)
	emit.Syscall(w.BinWriter, interopnames.SystemStoragePut)
	emit.Opcode(w.BinWriter, opcode.RET)

	m := manifest.NewManifest(h)
	m.Features = smartcontract.HasStorage
	m.ABI.Methods = []manifest.Method{
		{
			Name:   "requestURL",
			Offset: 0,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("url", smartcontract.StringType),
				manifest.NewParameter("filter", smartcontract.StringType),
				manifest.NewParameter("callback", smartcontract.StringType),
				manifest.NewParameter("userData", smartcontract.AnyType),
				manifest.NewParameter("gasForResponse", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "handle",
			Offset: offset,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("url", smartcontract.StringType),
				manifest.NewParameter("userData", smartcontract.AnyType),
				manifest.NewParameter("code", smartcontract.IntegerType),
				manifest.NewParameter("result", smartcontract.ByteArrayType),
			},
			ReturnType: smartcontract.VoidType,
		},
	}

	perm := manifest.NewPermission(manifest.PermissionHash, h)
	perm.Methods.Add("request")
	m.Permissions = append(m.Permissions, *perm)

	return &state.Contract{
		Script:   w.Bytes(),
		Manifest: *m,
		ID:       42,
	}
}

func putOracleRequest(t *testing.T, h util.Uint160, bc *Blockchain,
	url, filter string, userData []byte, gas int64) util.Uint256 {
	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, h, "requestURL",
		url, filter, "handle", userData, gas)
	require.NoError(t, w.Err)

	gas += 50_000_000 + 5_000_000 // request + contract call with args
	tx := transaction.New(netmode.UnitTestNet, w.Bytes(), gas)
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	tx.NetworkFee = 1_000_000
	setSigner(tx, testchain.MultisigScriptHash())
	require.NoError(t, signTx(bc, tx))
	require.NoError(t, bc.AddBlock(bc.newBlock(tx)))
	return tx.Hash()
}

func TestOracle_Request(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	orc := bc.contracts.Oracle
	cs := getOracleContractState(orc.Hash)
	require.NoError(t, bc.dao.PutContractState(cs))

	gasForResponse := int64(2000_1234)
	userData := []byte("custom info")
	txHash := putOracleRequest(t, cs.ScriptHash(), bc, "url", "flt", userData, gasForResponse)

	req, err := orc.GetRequestInternal(bc.dao, 1)
	require.NotNil(t, req)
	require.NoError(t, err)
	require.Equal(t, txHash, req.OriginalTxID)
	require.Equal(t, "url", req.URL)
	require.Equal(t, "flt", *req.Filter)
	require.Equal(t, cs.ScriptHash(), req.CallbackContract)
	require.Equal(t, "handle", req.CallbackMethod)
	require.Equal(t, uint64(gasForResponse), req.GasForResponse)

	idList, err := orc.GetIDListInternal(bc.dao, "url")
	require.NoError(t, err)
	require.Equal(t, &native.IDList{1}, idList)

	// Finish.
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()

	tx := transaction.New(netmode.UnitTestNet, []byte{}, 0)
	setSigner(tx, testchain.CommitteeScriptHash())
	ic := bc.newInteropContext(trigger.Application, bc.dao, nil, tx)
	ic.SpawnVM()
	ic.VM.LoadScript([]byte{byte(opcode.RET)})
	err = orc.SetOracleNodes(ic, keys.PublicKeys{pub})
	require.NoError(t, err)
	orc.OnPersistEnd(ic.DAO)

	tx = transaction.New(netmode.UnitTestNet, native.GetOracleResponseScript(), 0)
	ic.Tx = tx
	ic.Block = bc.newBlock(tx)

	err = orc.FinishInternal(ic)
	require.True(t, errors.Is(err, native.ErrResponseNotFound), "got: %v", err)

	resp := &transaction.OracleResponse{
		ID:     13,
		Code:   transaction.Success,
		Result: []byte{4, 8, 15, 16, 23, 42},
	}
	tx.Attributes = []transaction.Attribute{{
		Type:  transaction.OracleResponseT,
		Value: resp,
	}}
	err = orc.FinishInternal(ic)
	require.True(t, errors.Is(err, native.ErrRequestNotFound), "got: %v", err)

	// We need to ensure that callback is called thus, executing full script is necessary.
	resp.ID = 1
	ic.VM.LoadScriptWithFlags(tx.Script, smartcontract.All)
	require.NoError(t, ic.VM.Run())

	si := ic.DAO.GetStorageItem(cs.ID, []byte("lastOracleResponse"))
	require.NotNil(t, si)
	item, err := stackitem.DeserializeItem(si.Value)
	require.NoError(t, err)
	arr, ok := item.Value().([]stackitem.Item)
	require.True(t, ok)
	require.Equal(t, []byte("url"), arr[0].Value())
	require.Equal(t, userData, arr[1].Value())
	require.Equal(t, big.NewInt(int64(resp.Code)), arr[2].Value())
	require.Equal(t, resp.Result, arr[3].Value())

	// Check that processed request is removed during `postPersist`.
	_, err = orc.GetRequestInternal(ic.DAO, 1)
	require.NoError(t, err)

	require.NoError(t, orc.PostPersist(ic))
	_, err = orc.GetRequestInternal(ic.DAO, 1)
	require.Error(t, err)

	t.Run("ErrorOnFinish", func(t *testing.T) {
		const reqID = 2

		putOracleRequest(t, cs.ScriptHash(), bc, "url", "flt", []byte{1, 2}, gasForResponse)
		_, err := orc.GetRequestInternal(bc.dao, reqID) // ensure ID is 2
		require.NoError(t, err)

		tx = transaction.New(netmode.UnitTestNet, native.GetOracleResponseScript(), 0)
		tx.Attributes = []transaction.Attribute{{
			Type: transaction.OracleResponseT,
			Value: &transaction.OracleResponse{
				ID:     reqID,
				Code:   transaction.Success,
				Result: []byte{4, 8, 15, 16, 23, 42},
			},
		}}
		ic := bc.newInteropContext(trigger.Application, bc.dao, bc.newBlock(tx), tx)
		ic.VM = ic.SpawnVM()
		ic.VM.LoadScriptWithFlags(tx.Script, smartcontract.All)
		require.Error(t, ic.VM.Run())

		// Request is cleaned up even if callback failed.
		require.NoError(t, orc.PostPersist(ic))
		_, err = orc.GetRequestInternal(ic.DAO, reqID)
		require.Error(t, err)
	})
}

func TestOracle_SetOracleNodes(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	orc := bc.contracts.Oracle
	tx := transaction.New(netmode.UnitTestNet, []byte{}, 0)
	ic := bc.newInteropContext(trigger.System, bc.dao, nil, tx)
	ic.VM = vm.New()
	ic.VM.LoadScript([]byte{byte(opcode.RET)})

	pubs := orc.GetOracleNodes()
	require.Equal(t, 0, len(pubs))

	err := orc.SetOracleNodes(ic, keys.PublicKeys{})
	require.True(t, errors.Is(err, native.ErrEmptyNodeList), "got: %v", err)

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	pub := priv.PublicKey()
	err = orc.SetOracleNodes(ic, keys.PublicKeys{pub})
	require.True(t, errors.Is(err, native.ErrInvalidWitness), "got: %v", err)

	setSigner(tx, testchain.CommitteeScriptHash())
	require.NoError(t, orc.SetOracleNodes(ic, keys.PublicKeys{pub}))
	orc.OnPersistEnd(ic.DAO)

	pubs = orc.GetOracleNodes()
	require.Equal(t, keys.PublicKeys{pub}, pubs)
}
