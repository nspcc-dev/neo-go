package native_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/contracts"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newManagementClient(t *testing.T) *neotest.ContractInvoker {
	return newNativeClient(t, nativenames.Management)
}

func TestManagement_MinimumDeploymentFee(t *testing.T) {
	testGetSet(t, newManagementClient(t), "MinimumDeploymentFee", 10_00000000, 0, 0)
}

func TestManagement_MinimumDeploymentFeeCache(t *testing.T) {
	c := newManagementClient(t)
	testGetSetCache(t, c, "MinimumDeploymentFee", 10_00000000)
}

func TestManagement_ContractCache(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.Committee.ScriptHash())
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	// Deploy contract, abort the transaction and check that Management cache wasn't persisted
	// for FAULTed tx at the same block.
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, managementInvoker.Hash, "deploy", callflag.All, nefBytes, manifestBytes)
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	tx1 := managementInvoker.PrepareInvocation(t, w.Bytes(), managementInvoker.Signers)
	tx2 := managementInvoker.PrepareInvoke(t, "getContract", cs1.Hash.BytesBE())
	managementInvoker.AddNewBlock(t, tx1, tx2)
	managementInvoker.CheckFault(t, tx1.Hash(), "ABORT")
	managementInvoker.CheckHalt(t, tx2.Hash(), stackitem.Null{})

	// Deploy the contract and check that cache was persisted for HALTed transaction at the same block.
	tx1 = managementInvoker.PrepareInvoke(t, "deploy", nefBytes, manifestBytes)
	tx2 = managementInvoker.PrepareInvoke(t, "getContract", cs1.Hash.BytesBE())
	managementInvoker.AddNewBlock(t, tx1, tx2)
	managementInvoker.CheckHalt(t, tx1.Hash())
	aer, err := managementInvoker.Chain.GetAppExecResults(tx2.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, aer[0].VMState, aer[0].FaultException)
	require.NotEqual(t, stackitem.Null{}, aer[0].Stack)
}

func TestManagement_ContractDeploy(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.Committee.ScriptHash())
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	t.Run("no NEF", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "no valid NEF provided", "deploy", nil, manifestBytes)
	})
	t.Run("no manifest", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "no valid manifest provided", "deploy", nefBytes, nil)
	})
	t.Run("int for NEF", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid NEF file", "deploy", int64(1), manifestBytes)
	})
	t.Run("zero-length NEF", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid NEF file", "deploy", []byte{}, manifestBytes)
	})
	t.Run("array for NEF", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid NEF file", "deploy", []interface{}{int64(1)}, manifestBytes)
	})
	t.Run("bad script in NEF", func(t *testing.T) {
		nf, err := nef.FileFromBytes(nefBytes) // make a full copy
		require.NoError(t, err)
		nf.Script[0] = 0xff
		nf.CalculateChecksum()
		nefBad, err := nf.Bytes()
		require.NoError(t, err)
		managementInvoker.InvokeFail(t, "invalid NEF file", "deploy", nefBad, manifestBytes)
	})
	t.Run("int for manifest", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid manifest", "deploy", nefBytes, int64(1))
	})
	t.Run("zero-length manifest", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid manifest", "deploy", nefBytes, []byte{})
	})
	t.Run("array for manifest", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid manifest", "deploy", nefBytes, []interface{}{int64(1)})
	})
	t.Run("non-utf8 manifest", func(t *testing.T) {
		manifestBad := bytes.Replace(manifestBytes, []byte("TestMain"), []byte("\xff\xfe\xfd"), 1) // Replace name.
		managementInvoker.InvokeFail(t, "manifest is not UTF-8 compliant", "deploy", nefBytes, manifestBad)
	})
	t.Run("invalid manifest", func(t *testing.T) {
		pkey, err := keys.NewPrivateKey()
		require.NoError(t, err)

		badManifest := cs1.Manifest
		badManifest.Groups = []manifest.Group{{PublicKey: pkey.PublicKey(), Signature: make([]byte, 64)}}
		manifB, err := json.Marshal(&badManifest)
		require.NoError(t, err)

		managementInvoker.InvokeFail(t, "invalid manifest", "deploy", nefBytes, manifB)
	})
	t.Run("bad methods in manifest 1", func(t *testing.T) {
		badManifest := cs1.Manifest
		badManifest.ABI.Methods = make([]manifest.Method, len(cs1.Manifest.ABI.Methods))
		copy(badManifest.ABI.Methods, cs1.Manifest.ABI.Methods)
		badManifest.ABI.Methods[0].Offset = 100500 // out of bounds
		manifB, err := json.Marshal(&badManifest)
		require.NoError(t, err)

		managementInvoker.InvokeFail(t, "out of bounds method offset", "deploy", nefBytes, manifB)
	})

	t.Run("bad methods in manifest 2", func(t *testing.T) {
		var badManifest = cs1.Manifest
		badManifest.ABI.Methods = make([]manifest.Method, len(cs1.Manifest.ABI.Methods))
		copy(badManifest.ABI.Methods, cs1.Manifest.ABI.Methods)
		badManifest.ABI.Methods[0].Offset = len(cs1.NEF.Script) - 2 // Ends with `CALLT(X,X);RET`.

		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)

		managementInvoker.InvokeFail(t, "some methods point to wrong offsets (not to instruction boundary)", "deploy", nefBytes, manifB)
	})

	t.Run("not enough GAS", func(t *testing.T) {
		tx := managementInvoker.NewUnsignedTx(t, managementInvoker.Hash, "deploy", nefBytes, manifestBytes)
		managementInvoker.SignTx(t, tx, 1_0000_0000, managementInvoker.Signers...)
		managementInvoker.AddNewBlock(t, tx)
		managementInvoker.CheckFault(t, tx.Hash(), "gas limit exceeded")
	})

	si, err := cs1.ToStackItem()
	require.NoError(t, err)

	t.Run("positive", func(t *testing.T) {
		tx1 := managementInvoker.PrepareInvoke(t, "deploy", nefBytes, manifestBytes)
		tx2 := managementInvoker.PrepareInvoke(t, "getContract", cs1.Hash.BytesBE())
		managementInvoker.AddNewBlock(t, tx1, tx2)
		managementInvoker.CheckHalt(t, tx1.Hash(), si)
		managementInvoker.CheckHalt(t, tx2.Hash(), si)
		managementInvoker.CheckTxNotificationEvent(t, tx1.Hash(), 0, state.NotificationEvent{
			ScriptHash: c.NativeHash(t, nativenames.Management),
			Name:       "Deploy",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		})
		t.Run("_deploy called", func(t *testing.T) {
			helperInvoker := c.Executor.CommitteeInvoker(cs1.Hash)
			expected := stackitem.NewArray([]stackitem.Item{stackitem.Make("create"), stackitem.Null{}})
			expectedBytes, err := stackitem.Serialize(expected)
			require.NoError(t, err)
			helperInvoker.Invoke(t, stackitem.NewByteArray(expectedBytes), "getValue")
		})
		t.Run("get after deploy", func(t *testing.T) {
			managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
		})
		t.Run("get after restore", func(t *testing.T) {
			w := io.NewBufBinWriter()
			require.NoError(t, chaindump.Dump(c.Executor.Chain, w.BinWriter, 0, c.Executor.Chain.BlockHeight()+1))
			require.NoError(t, w.Err)

			r := io.NewBinReaderFromBuf(w.Bytes())
			bc2, acc := chain.NewSingle(t)
			e2 := neotest.NewExecutor(t, bc2, acc, acc)
			managementInvoker2 := e2.CommitteeInvoker(e2.NativeHash(t, nativenames.Management))

			require.NoError(t, chaindump.Restore(bc2, r, 0, c.Executor.Chain.BlockHeight()+1, nil))
			require.NoError(t, r.Err)
			managementInvoker2.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
		})
	})
	t.Run("contract already exists", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "contract already exists", "deploy", nefBytes, manifestBytes)
	})
	t.Run("failed _deploy", func(t *testing.T) {
		deployScript := []byte{byte(opcode.ABORT)}
		m := manifest.NewManifest("TestDeployAbort")
		m.ABI.Methods = []manifest.Method{
			{
				Name:   manifest.MethodDeploy,
				Offset: 0,
				Parameters: []manifest.Parameter{
					manifest.NewParameter("data", smartcontract.AnyType),
					manifest.NewParameter("isUpdate", smartcontract.BoolType),
				},
				ReturnType: smartcontract.VoidType,
			},
		}
		nefD, err := nef.NewFile(deployScript)
		require.NoError(t, err)
		nefDb, err := nefD.Bytes()
		require.NoError(t, err)
		manifD, err := json.Marshal(m)
		require.NoError(t, err)
		managementInvoker.InvokeFail(t, "ABORT", "deploy", nefDb, manifD)

		t.Run("get after failed deploy", func(t *testing.T) {
			h := state.CreateContractHash(c.CommitteeHash, nefD.Checksum, m.Name)
			managementInvoker.Invoke(t, stackitem.Null{}, "getContract", h.BytesBE())
		})
	})
	t.Run("bad _deploy", func(t *testing.T) { // invalid _deploy signature
		deployScript := []byte{byte(opcode.RET)}
		m := manifest.NewManifest("TestBadDeploy")
		m.ABI.Methods = []manifest.Method{
			{
				Name:   manifest.MethodDeploy,
				Offset: 0,
				Parameters: []manifest.Parameter{
					manifest.NewParameter("data", smartcontract.AnyType),
					manifest.NewParameter("isUpdate", smartcontract.BoolType),
				},
				ReturnType: smartcontract.ArrayType,
			},
		}
		nefD, err := nef.NewFile(deployScript)
		require.NoError(t, err)
		nefDb, err := nefD.Bytes()
		require.NoError(t, err)
		manifD, err := json.Marshal(m)
		require.NoError(t, err)
		managementInvoker.InvokeFail(t, "invalid return values count: expected 0, got 2", "deploy", nefDb, manifD)

		t.Run("get after bad _deploy", func(t *testing.T) {
			h := state.CreateContractHash(c.CommitteeHash, nefD.Checksum, m.Name)
			managementInvoker.Invoke(t, stackitem.Null{}, "getContract", h.BytesBE())
		})
	})
}

func TestManagement_StartFromHeight(t *testing.T) {
	// Create database to be able to start another chain from the same height later.
	ldbDir := t.TempDir()
	dbConfig := storage.DBConfiguration{
		Type: "leveldb",
		LevelDBOptions: storage.LevelDBOptions{
			DataDirectoryPath: ldbDir,
		},
	}
	newLevelStore, err := storage.NewLevelDBStore(dbConfig.LevelDBOptions)
	require.Nil(t, err, "NewLevelDBStore error")

	// Create blockchain and put contract state to it.
	bc, acc := chain.NewSingleWithCustomConfigAndStore(t, nil, newLevelStore, false)
	go bc.Run()
	e := neotest.NewExecutor(t, bc, acc, acc)
	c := e.CommitteeInvoker(e.NativeHash(t, nativenames.Management))
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	si, err := cs1.ToStackItem()
	require.NoError(t, err)

	managementInvoker.Invoke(t, si, "deploy", nefBytes, manifestBytes)
	managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())

	// Close current blockchain and start the new one from the same height with the same db.
	bc.Close()
	newLevelStore, err = storage.NewLevelDBStore(dbConfig.LevelDBOptions)
	require.NoError(t, err)
	bc2, acc := chain.NewSingleWithCustomConfigAndStore(t, nil, newLevelStore, true)
	e2 := neotest.NewExecutor(t, bc2, acc, acc)
	managementInvoker2 := e2.CommitteeInvoker(e2.NativeHash(t, nativenames.Management))

	// Check that initialisation of native Management was correctly performed.
	managementInvoker2.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
}

func TestManagement_DeployManifestOverflow(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	manif1, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nef1, err := nef.NewFile(cs1.NEF.Script)
	require.NoError(t, err)
	nef1b, err := nef1.Bytes()
	require.NoError(t, err)

	w := io.NewBufBinWriter()
	emit.Bytes(w.BinWriter, manif1)
	emit.Int(w.BinWriter, manifest.MaxManifestSize)
	emit.Opcodes(w.BinWriter, opcode.NEWBUFFER, opcode.CAT)
	emit.Bytes(w.BinWriter, nef1b)
	emit.Int(w.BinWriter, 2)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.AppCallNoArgs(w.BinWriter, managementInvoker.Hash, "deploy", callflag.All)
	require.NoError(t, w.Err)
	script := w.Bytes()

	tx := transaction.New(script, 0)
	tx.ValidUntilBlock = managementInvoker.Chain.BlockHeight() + 1
	managementInvoker.SignTx(t, tx, 100_0000_0000, managementInvoker.Signers...)
	managementInvoker.AddNewBlock(t, tx)
	managementInvoker.CheckFault(t, tx.Hash(), fmt.Sprintf("invalid manifest: len is %d (max %d)", manifest.MaxManifestSize+len(manif1), manifest.MaxManifestSize))
}

func TestManagement_ContractDeployAndUpdateWithParameter(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	cs1.Manifest.Permissions = []manifest.Permission{*manifest.NewPermission(manifest.PermissionWildcard)}
	cs1.ID = 1
	cs1.Hash = state.CreateContractHash(c.CommitteeHash, cs1.NEF.Checksum, cs1.Manifest.Name)
	manif1, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nef1b, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	si, err := cs1.ToStackItem()
	require.NoError(t, err)
	managementInvoker.Invoke(t, si, "deploy", nef1b, manif1)
	helperInvoker := c.Executor.CommitteeInvoker(cs1.Hash)

	t.Run("_deploy called", func(t *testing.T) {
		expected := stackitem.NewArray([]stackitem.Item{stackitem.Make("create"), stackitem.Null{}})
		expectedBytes, err := stackitem.Serialize(expected)
		require.NoError(t, err)
		helperInvoker.Invoke(t, stackitem.NewByteArray(expectedBytes), "getValue")
	})

	cs1.NEF.Script = append(cs1.NEF.Script, byte(opcode.RET))
	cs1.NEF.Checksum = cs1.NEF.CalculateChecksum()
	nef1b, err = cs1.NEF.Bytes()
	require.NoError(t, err)
	cs1.UpdateCounter++

	helperInvoker.Invoke(t, stackitem.Null{}, "update", nef1b, nil, "new data")

	t.Run("_deploy called", func(t *testing.T) {
		expected := stackitem.NewArray([]stackitem.Item{stackitem.Make("update"), stackitem.Make("new data")})
		expectedBytes, err := stackitem.Serialize(expected)
		require.NoError(t, err)
		helperInvoker.Invoke(t, stackitem.NewByteArray(expectedBytes), "getValue")
	})
}

func TestManagement_ContractUpdate(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	// Allow calling management contract.
	cs1.Manifest.Permissions = []manifest.Permission{*manifest.NewPermission(manifest.PermissionWildcard)}
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	si, err := cs1.ToStackItem()
	require.NoError(t, err)
	managementInvoker.Invoke(t, si, "deploy", nefBytes, manifestBytes)
	helperInvoker := c.Executor.CommitteeInvoker(cs1.Hash)

	t.Run("unknown contract", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "contract doesn't exist", "update", nefBytes, manifestBytes)
	})
	t.Run("zero-length NEF", func(t *testing.T) {
		helperInvoker.InvokeFail(t, "invalid NEF file: empty", "update", []byte{}, manifestBytes)
	})
	t.Run("zero-length manifest", func(t *testing.T) {
		helperInvoker.InvokeFail(t, "invalid manifest: empty", "update", nefBytes, []byte{})
	})
	t.Run("no real params", func(t *testing.T) {
		helperInvoker.InvokeFail(t, "both NEF and manifest are nil", "update", nil, nil)
	})
	t.Run("invalid manifest", func(t *testing.T) {
		pkey, err := keys.NewPrivateKey()
		require.NoError(t, err)

		var badManifest = cs1.Manifest
		badManifest.Groups = []manifest.Group{{PublicKey: pkey.PublicKey(), Signature: make([]byte, 64)}}
		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)

		helperInvoker.InvokeFail(t, "invalid manifest: incorrect group signature", "update", nefBytes, manifB)
	})
	t.Run("manifest and script mismatch", func(t *testing.T) {
		nf, err := nef.FileFromBytes(nefBytes) // Make a full copy.
		require.NoError(t, err)
		nf.Script = append(nf.Script, byte(opcode.RET))
		copy(nf.Script[1:], nf.Script)  // Now all method offsets are wrong.
		nf.Script[0] = byte(opcode.RET) // Even though the script is correct.
		nf.CalculateChecksum()
		nefnew, err := nf.Bytes()
		require.NoError(t, err)
		helperInvoker.InvokeFail(t, "invalid NEF file: checksum verification failure", "update", nefnew, manifestBytes)
	})

	t.Run("change name", func(t *testing.T) {
		var badManifest = cs1.Manifest
		badManifest.Name += "tail"
		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)

		helperInvoker.InvokeFail(t, "contract name can't be changed", "update", nefBytes, manifB)
	})

	cs1.NEF.Script = append(cs1.NEF.Script, byte(opcode.RET))
	cs1.NEF.Checksum = cs1.NEF.CalculateChecksum()
	nefBytes, err = cs1.NEF.Bytes()
	require.NoError(t, err)
	cs1.UpdateCounter++
	si, err = cs1.ToStackItem()
	require.NoError(t, err)

	t.Run("update script, positive", func(t *testing.T) {
		tx1 := helperInvoker.PrepareInvoke(t, "update", nefBytes, nil)
		tx2 := managementInvoker.PrepareInvoke(t, "getContract", cs1.Hash.BytesBE())
		managementInvoker.AddNewBlock(t, tx1, tx2)
		managementInvoker.CheckHalt(t, tx1.Hash(), stackitem.Null{})
		managementInvoker.CheckHalt(t, tx2.Hash(), si)
		managementInvoker.CheckTxNotificationEvent(t, tx1.Hash(), 0, state.NotificationEvent{
			ScriptHash: c.NativeHash(t, nativenames.Management),
			Name:       "Update",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		})
		t.Run("_deploy called", func(t *testing.T) {
			helperInvoker := c.Executor.CommitteeInvoker(cs1.Hash)
			expected := stackitem.NewArray([]stackitem.Item{stackitem.Make("update"), stackitem.Null{}})
			expectedBytes, err := stackitem.Serialize(expected)
			require.NoError(t, err)
			helperInvoker.Invoke(t, stackitem.NewByteArray(expectedBytes), "getValue")
		})
		t.Run("check contract", func(t *testing.T) {
			managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
		})
	})

	cs1.Manifest.Extra = []byte(`"update me"`)
	manifestBytes, err = json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	cs1.UpdateCounter++
	si, err = cs1.ToStackItem()
	require.NoError(t, err)

	t.Run("update manifest, positive", func(t *testing.T) {
		updHash := helperInvoker.Invoke(t, stackitem.Null{}, "update", nil, manifestBytes)
		helperInvoker.CheckTxNotificationEvent(t, updHash, 0, state.NotificationEvent{
			ScriptHash: helperInvoker.NativeHash(t, nativenames.Management),
			Name:       "Update",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		})
		t.Run("check contract", func(t *testing.T) {
			managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
		})
	})

	cs1.NEF.Script = append(cs1.NEF.Script, byte(opcode.ABORT))
	cs1.NEF.Checksum = cs1.NEF.CalculateChecksum()
	nefBytes, err = cs1.NEF.Bytes()
	require.NoError(t, err)
	cs1.Manifest.Extra = []byte(`"update me once more"`)
	manifestBytes, err = json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	cs1.UpdateCounter++
	si, err = cs1.ToStackItem()
	require.NoError(t, err)

	t.Run("update both script and manifest", func(t *testing.T) {
		updHash := helperInvoker.Invoke(t, stackitem.Null{}, "update", nefBytes, manifestBytes)
		helperInvoker.CheckTxNotificationEvent(t, updHash, 0, state.NotificationEvent{
			ScriptHash: helperInvoker.NativeHash(t, nativenames.Management),
			Name:       "Update",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		})
		t.Run("check contract", func(t *testing.T) {
			managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
		})
	})
}

func TestManagement_GetContract(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	si, err := cs1.ToStackItem()
	require.NoError(t, err)
	managementInvoker.Invoke(t, si, "deploy", nefBytes, manifestBytes)

	t.Run("bad parameter type", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "invalid conversion: Array/ByteString", "getContract", []interface{}{int64(1)})
	})
	t.Run("not a hash", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "expected byte size of 20 got 3", "getContract", []byte{1, 2, 3})
	})
	t.Run("positive", func(t *testing.T) {
		managementInvoker.Invoke(t, si, "getContract", cs1.Hash.BytesBE())
	})
}

func TestManagement_ContractDestroy(t *testing.T) {
	c := newManagementClient(t)
	managementInvoker := c.WithSigners(c.Committee)

	cs1, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, c.CommitteeHash)
	// Allow calling management contract.
	cs1.Manifest.Permissions = []manifest.Permission{*manifest.NewPermission(manifest.PermissionWildcard)}
	manifestBytes, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nefBytes, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	si, err := cs1.ToStackItem()
	require.NoError(t, err)
	managementInvoker.Invoke(t, si, "deploy", nefBytes, manifestBytes)
	helperInvoker := c.Executor.CommitteeInvoker(cs1.Hash)

	t.Run("no contract", func(t *testing.T) {
		managementInvoker.InvokeFail(t, "key not found", "destroy")
	})
	t.Run("positive", func(t *testing.T) {
		dstrHash := helperInvoker.Invoke(t, stackitem.Null{}, "destroy")
		helperInvoker.CheckTxNotificationEvent(t, dstrHash, 0, state.NotificationEvent{
			ScriptHash: helperInvoker.NativeHash(t, nativenames.Management),
			Name:       "Destroy",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		})
		t.Run("check contract", func(t *testing.T) {
			managementInvoker.Invoke(t, stackitem.Null{}, "getContract", cs1.Hash.BytesBE())
		})
		// deploy after destroy should fail
		managementInvoker.InvokeFail(t, fmt.Sprintf("the contract %s has been blocked", cs1.Hash.StringLE()), "deploy", nefBytes, manifestBytes)
	})
}
