package core

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

// This is in a separate test because test test for long manifest
// prevents chain from being dumped. In any real scenario
// restrictions on tx script length will be applied before
// restrictions on manifest size. In this test providing manifest of max size
// leads to tx deserialization failure.
func TestRestoreAfterDeploy(t *testing.T) {
	bc := newTestChain(t)

	// nef.NewFile() cares about version a lot.
	config.Version = "0.90.0-test"
	mgmtHash := bc.ManagementContractHash()
	cs1, _ := getTestContractState(bc)
	cs1.ID = 1
	cs1.Hash = state.CreateContractHash(testchain.MultisigScriptHash(), cs1.NEF.Checksum, cs1.Manifest.Name)
	manif1, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nef1, err := nef.NewFile(cs1.NEF.Script)
	require.NoError(t, err)
	nef1b, err := nef1.Bytes()
	require.NoError(t, err)

	res, err := invokeContractMethod(bc, 100_00000000, mgmtHash, "deploy", nef1b, append(manif1, make([]byte, manifest.MaxManifestSize)...))
	require.NoError(t, err)
	checkFAULTState(t, res)
}

type memoryStore struct {
	*storage.MemoryStore
}

func (memoryStore) Close() error { return nil }

func TestStartFromHeight(t *testing.T) {
	st := memoryStore{storage.NewMemoryStore()}
	bc := newTestChainWithCustomCfgAndStore(t, st, nil)
	cs1, _ := getTestContractState(bc)
	func() {
		require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs1))
		checkContractState(t, bc, cs1.Hash, cs1)
		_, err := bc.dao.Store.Persist()
		require.NoError(t, err)
	}()

	bc2 := newTestChainWithCustomCfgAndStore(t, st, nil)
	checkContractState(t, bc2, cs1.Hash, cs1)
}

func TestContractDeployAndUpdateWithParameter(t *testing.T) {
	bc := newTestChain(t)

	// nef.NewFile() cares about version a lot.
	config.Version = "0.90.0-test"
	mgmtHash := bc.ManagementContractHash()
	cs1, _ := getTestContractState(bc)
	cs1.Manifest.Permissions = []manifest.Permission{*manifest.NewPermission(manifest.PermissionWildcard)}
	cs1.ID = 1
	cs1.Hash = state.CreateContractHash(testchain.MultisigScriptHash(), cs1.NEF.Checksum, cs1.Manifest.Name)
	manif1, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nef1b, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	aer, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nef1b, manif1, int64(42))
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, aer.VMState)

	t.Run("_deploy called", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 1_00000000, cs1.Hash, "getValue")
		require.NoError(t, err)
		require.Equal(t, 1, len(res.Stack))
		item, err := stackitem.Deserialize(res.Stack[0].Value().([]byte))
		require.NoError(t, err)
		expected := []stackitem.Item{stackitem.Make("create"), stackitem.Make(42)}
		require.Equal(t, stackitem.NewArray(expected), item)
	})

	cs1.NEF.Script = append(cs1.NEF.Script, byte(opcode.RET))
	cs1.NEF.Checksum = cs1.NEF.CalculateChecksum()
	nef1b, err = cs1.NEF.Bytes()
	require.NoError(t, err)
	cs1.UpdateCounter++

	aer, err = invokeContractMethod(bc, 10_00000000, cs1.Hash, "update", nef1b, nil, "new data")
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, aer.VMState)

	t.Run("_deploy called", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 1_00000000, cs1.Hash, "getValue")
		require.NoError(t, err)
		require.Equal(t, 1, len(res.Stack))
		item, err := stackitem.Deserialize(res.Stack[0].Value().([]byte))
		require.NoError(t, err)
		expected := []stackitem.Item{stackitem.Make("update"), stackitem.Make("new data")}
		require.Equal(t, stackitem.NewArray(expected), item)
	})
}

func TestContractDeploy(t *testing.T) {
	bc := newTestChain(t)

	// nef.NewFile() cares about version a lot.
	config.Version = "0.90.0-test"
	mgmtHash := bc.ManagementContractHash()
	cs1, _ := getTestContractState(bc)
	cs1.ID = 1
	cs1.Hash = state.CreateContractHash(testchain.MultisigScriptHash(), cs1.NEF.Checksum, cs1.Manifest.Name)
	manif1, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nef1b, err := cs1.NEF.Bytes()
	require.NoError(t, err)

	t.Run("no NEF", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nil, manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("no manifest", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nef1b, nil)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("int for NEF", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", int64(1), manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("zero-length NEF", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", []byte{}, manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("array for NEF", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", []interface{}{int64(1)}, manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("bad script in NEF", func(t *testing.T) {
		nf, err := nef.FileFromBytes(nef1b) // make a full copy
		require.NoError(t, err)
		nf.Script[0] = 0xff
		nf.CalculateChecksum()
		nefbad, err := nf.Bytes()
		require.NoError(t, err)
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nefbad, manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("int for manifest", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nef1b, int64(1))
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("zero-length manifest", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nef1b, []byte{})
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("array for manifest", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nef1b, []interface{}{int64(1)})
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("non-utf8 manifest", func(t *testing.T) {
		manifB := bytes.Replace(manif1, []byte("TestMain"), []byte("\xff\xfe\xfd"), 1) // Replace name.

		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nef1b, manifB)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("invalid manifest", func(t *testing.T) {
		pkey, err := keys.NewPrivateKey()
		require.NoError(t, err)

		var badManifest = cs1.Manifest
		badManifest.Groups = []manifest.Group{{PublicKey: pkey.PublicKey(), Signature: make([]byte, 64)}}
		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)

		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nef1b, manifB)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("bad methods in manifest 1", func(t *testing.T) {
		var badManifest = cs1.Manifest
		badManifest.ABI.Methods = make([]manifest.Method, len(cs1.Manifest.ABI.Methods))
		copy(badManifest.ABI.Methods, cs1.Manifest.ABI.Methods)
		badManifest.ABI.Methods[0].Offset = 100500 // out of bounds

		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nef1b, manifB)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})

	t.Run("bad methods in manifest 2", func(t *testing.T) {
		var badManifest = cs1.Manifest
		badManifest.ABI.Methods = make([]manifest.Method, len(cs1.Manifest.ABI.Methods))
		copy(badManifest.ABI.Methods, cs1.Manifest.ABI.Methods)
		badManifest.ABI.Methods[0].Offset = len(cs1.NEF.Script) - 2 // Ends with `CALLT(X,X);RET`.

		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nef1b, manifB)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})

	t.Run("not enough GAS", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 1_00000000, mgmtHash, "deploy", nef1b, manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("positive", func(t *testing.T) {
		tx1, err := prepareContractMethodInvoke(bc, 11_00000000, mgmtHash, "deploy", nef1b, manif1)
		require.NoError(t, err)
		tx2, err := prepareContractMethodInvoke(bc, 1_00000000, mgmtHash, "getContract", cs1.Hash.BytesBE())
		require.NoError(t, err)

		aers, err := persistBlock(bc, tx1, tx2)
		require.NoError(t, err)
		for _, res := range aers {
			require.Equal(t, vm.HaltState, res.VMState)
			require.Equal(t, 1, len(res.Stack))
			compareContractStates(t, cs1, res.Stack[0])
		}
		require.Equal(t, aers[0].Events, []state.NotificationEvent{{
			ScriptHash: mgmtHash,
			Name:       "Deploy",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		}})
		t.Run("_deploy called", func(t *testing.T) {
			res, err := invokeContractMethod(bc, 1_00000000, cs1.Hash, "getValue")
			require.NoError(t, err)
			require.Equal(t, 1, len(res.Stack))
			item, err := stackitem.Deserialize(res.Stack[0].Value().([]byte))
			require.NoError(t, err)
			expected := []stackitem.Item{stackitem.Make("create"), stackitem.Null{}}
			require.Equal(t, stackitem.NewArray(expected), item)
		})
		t.Run("get after deploy", func(t *testing.T) {
			checkContractState(t, bc, cs1.Hash, cs1)
		})
		t.Run("get after restore", func(t *testing.T) {
			w := io.NewBufBinWriter()
			require.NoError(t, chaindump.Dump(bc, w, 0, bc.BlockHeight()+1))
			require.NoError(t, w.Error())

			r := io.NewBinReaderFromBuf(w.Bytes())
			bc2 := newTestChain(t)

			require.NoError(t, chaindump.Restore(bc2, r, 0, bc.BlockHeight()+1, nil))
			require.NoError(t, r.Err)
			checkContractState(t, bc2, cs1.Hash, cs1)
		})
	})
	t.Run("contract already exists", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nef1b, manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
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
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nefDb, manifD)
		require.NoError(t, err)
		checkFAULTState(t, res)

		t.Run("get after failed deploy", func(t *testing.T) {
			h := state.CreateContractHash(neoOwner, nefD.Checksum, m.Name)
			checkContractState(t, bc, h, nil)
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
		res, err := invokeContractMethod(bc, 11_00000000, mgmtHash, "deploy", nefDb, manifD)
		require.NoError(t, err)
		checkFAULTState(t, res)

		t.Run("get after bad _deploy", func(t *testing.T) {
			h := state.CreateContractHash(neoOwner, nefD.Checksum, m.Name)
			checkContractState(t, bc, h, nil)
		})
	})
}

func checkContractState(t *testing.T, bc *Blockchain, h util.Uint160, cs *state.Contract) {
	mgmtHash := bc.contracts.Management.Hash
	res, err := invokeContractMethod(bc, 1_00000000, mgmtHash, "getContract", h.BytesBE())
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res.VMState)
	require.Equal(t, 1, len(res.Stack))
	if cs == nil {
		require.Equal(t, stackitem.Null{}, res.Stack[0])
	} else {
		compareContractStates(t, cs, res.Stack[0])
	}
}

func TestContractUpdate(t *testing.T) {
	bc := newTestChain(t)

	// nef.NewFile() cares about version a lot.
	config.Version = "0.90.0-test"
	mgmtHash := bc.ManagementContractHash()
	cs1, _ := getTestContractState(bc)
	// Allow calling management contract.
	cs1.Manifest.Permissions = []manifest.Permission{*manifest.NewPermission(manifest.PermissionWildcard)}
	err := bc.contracts.Management.PutContractState(bc.dao, cs1)
	require.NoError(t, err)
	manif1, err := json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	nef1, err := nef.NewFile(cs1.NEF.Script)
	require.NoError(t, err)
	nef1b, err := nef1.Bytes()
	require.NoError(t, err)

	t.Run("no contract", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 10_00000000, mgmtHash, "update", nef1b, manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("zero-length NEF", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 10_00000000, cs1.Hash, "update", []byte{}, manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("zero-length manifest", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 10_00000000, cs1.Hash, "update", nef1b, []byte{})
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("not enough GAS", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 1_00000000, cs1.Hash, "update", nef1b, manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("no real params", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 10_00000000, cs1.Hash, "update", nil, nil)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("invalid manifest", func(t *testing.T) {
		pkey, err := keys.NewPrivateKey()
		require.NoError(t, err)

		var badManifest = cs1.Manifest
		badManifest.Groups = []manifest.Group{{PublicKey: pkey.PublicKey(), Signature: make([]byte, 64)}}
		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)

		res, err := invokeContractMethod(bc, 10_00000000, cs1.Hash, "update", nef1b, manifB)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("manifest and script mismatch", func(t *testing.T) {
		nf, err := nef.FileFromBytes(nef1b) // Make a full copy.
		require.NoError(t, err)
		nf.Script = append(nf.Script, byte(opcode.RET))
		copy(nf.Script[1:], nf.Script)  // Now all method offsets are wrong.
		nf.Script[0] = byte(opcode.RET) // Even though the script is correct.
		nf.CalculateChecksum()
		nefnew, err := nf.Bytes()
		require.NoError(t, err)
		res, err := invokeContractMethod(bc, 10_00000000, cs1.Hash, "update", nefnew, manif1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})

	t.Run("change name", func(t *testing.T) {
		var badManifest = cs1.Manifest
		badManifest.Name += "tail"
		manifB, err := json.Marshal(badManifest)
		require.NoError(t, err)

		res, err := invokeContractMethod(bc, 10_00000000, cs1.Hash, "update", nef1b, manifB)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})

	cs1.NEF.Script = append(cs1.NEF.Script, byte(opcode.RET))
	cs1.NEF.Checksum = cs1.NEF.CalculateChecksum()
	nef1b, err = cs1.NEF.Bytes()
	require.NoError(t, err)
	cs1.UpdateCounter++

	t.Run("update script, positive", func(t *testing.T) {
		tx1, err := prepareContractMethodInvoke(bc, 10_00000000, cs1.Hash, "update", nef1b, nil)
		require.NoError(t, err)
		tx2, err := prepareContractMethodInvoke(bc, 1_00000000, mgmtHash, "getContract", cs1.Hash.BytesBE())
		require.NoError(t, err)

		aers, err := persistBlock(bc, tx1, tx2)
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, aers[0].VMState)
		require.Equal(t, vm.HaltState, aers[1].VMState)
		require.Equal(t, 1, len(aers[1].Stack))
		compareContractStates(t, cs1, aers[1].Stack[0])
		require.Equal(t, aers[0].Events, []state.NotificationEvent{{
			ScriptHash: mgmtHash,
			Name:       "Update",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		}})
		t.Run("_deploy called", func(t *testing.T) {
			res, err := invokeContractMethod(bc, 1_00000000, cs1.Hash, "getValue")
			require.NoError(t, err)
			require.Equal(t, 1, len(res.Stack))
			item, err := stackitem.Deserialize(res.Stack[0].Value().([]byte))
			require.NoError(t, err)
			expected := []stackitem.Item{stackitem.Make("update"), stackitem.Null{}}
			require.Equal(t, stackitem.NewArray(expected), item)
		})
		t.Run("check contract", func(t *testing.T) {
			checkContractState(t, bc, cs1.Hash, cs1)
		})
	})

	cs1.Manifest.Extra = []byte(`"update me"`)
	manif1, err = json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	cs1.UpdateCounter++

	t.Run("update manifest, positive", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 10_00000000, cs1.Hash, "update", nil, manif1)
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, res.VMState)
		require.Equal(t, res.Events, []state.NotificationEvent{{
			ScriptHash: mgmtHash,
			Name:       "Update",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		}})
		t.Run("check contract", func(t *testing.T) {
			checkContractState(t, bc, cs1.Hash, cs1)
		})
	})

	cs1.NEF.Script = append(cs1.NEF.Script, byte(opcode.ABORT))
	cs1.NEF.Checksum = cs1.NEF.CalculateChecksum()
	nef1b, err = cs1.NEF.Bytes()
	require.NoError(t, err)
	cs1.Manifest.Extra = []byte(`"update me once more"`)
	manif1, err = json.Marshal(cs1.Manifest)
	require.NoError(t, err)
	cs1.UpdateCounter++

	t.Run("update both script and manifest", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 10_00000000, cs1.Hash, "update", nef1b, manif1)
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, res.VMState)
		require.Equal(t, res.Events, []state.NotificationEvent{{
			ScriptHash: mgmtHash,
			Name:       "Update",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		}})
		t.Run("check contract", func(t *testing.T) {
			checkContractState(t, bc, cs1.Hash, cs1)
		})
	})
}

func TestGetContract(t *testing.T) {
	bc := newTestChain(t)

	mgmtHash := bc.ManagementContractHash()
	cs1, _ := getTestContractState(bc)
	err := bc.contracts.Management.PutContractState(bc.dao, cs1)
	require.NoError(t, err)

	t.Run("bad parameter type", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 1_00000000, mgmtHash, "getContract", []interface{}{int64(1)})
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("not a hash", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 1_00000000, mgmtHash, "getContract", []byte{1, 2, 3})
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("positive", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 1_00000000, mgmtHash, "getContract", cs1.Hash.BytesBE())
		require.NoError(t, err)
		require.Equal(t, 1, len(res.Stack))
		compareContractStates(t, cs1, res.Stack[0])
	})
}

func TestContractDestroy(t *testing.T) {
	bc := newTestChain(t)

	mgmtHash := bc.ManagementContractHash()
	cs1, _ := getTestContractState(bc)
	// Allow calling management contract.
	cs1.Manifest.Permissions = []manifest.Permission{*manifest.NewPermission(manifest.PermissionWildcard)}
	err := bc.contracts.Management.PutContractState(bc.dao, cs1)
	require.NoError(t, err)
	err = bc.dao.PutStorageItem(cs1.ID, []byte{1, 2, 3}, state.StorageItem{3, 2, 1})
	require.NoError(t, err)
	b := bc.dao.GetMPTBatch()
	_, _, err = bc.GetStateModule().(*stateroot.Module).AddMPTBatch(bc.BlockHeight(), b, bc.dao.Store)
	require.NoError(t, err)

	t.Run("no contract", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 1_00000000, mgmtHash, "destroy")
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("positive", func(t *testing.T) {
		res, err := invokeContractMethod(bc, 1_00000000, cs1.Hash, "destroy")
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, res.VMState)
		require.Equal(t, res.Events, []state.NotificationEvent{{
			ScriptHash: mgmtHash,
			Name:       "Destroy",
			Item:       stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(cs1.Hash.BytesBE())}),
		}})
		t.Run("check contract", func(t *testing.T) {
			checkContractState(t, bc, cs1.Hash, nil)
		})
	})
}

func compareContractStates(t *testing.T, expected *state.Contract, actual stackitem.Item) {
	act, ok := actual.Value().([]stackitem.Item)
	require.True(t, ok)

	expectedManifest, err := expected.Manifest.ToStackItem()
	require.NoError(t, err)
	expectedNef, err := expected.NEF.Bytes()
	require.NoError(t, err)

	require.Equal(t, 5, len(act))
	require.Equal(t, expected.ID, int32(act[0].Value().(*big.Int).Int64()))
	require.Equal(t, expected.UpdateCounter, uint16(act[1].Value().(*big.Int).Int64()))
	require.Equal(t, expected.Hash.BytesBE(), act[2].Value().([]byte))
	require.Equal(t, expectedNef, act[3].Value().([]byte))
	require.Equal(t, expectedManifest, act[4])
}

func TestMinimumDeploymentFee(t *testing.T) {
	chain := newTestChain(t)

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Management.GetMinimumDeploymentFee(chain.dao)
		require.Equal(t, 10_00000000, int(n))
	})

	testGetSet(t, chain, chain.contracts.Management.Hash, "MinimumDeploymentFee", 10_00000000, 0, 0)
}

func TestManagement_GetNEP17Contracts(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		chain := newTestChain(t)
		require.ElementsMatch(t, []util.Uint160{chain.contracts.NEO.Hash, chain.contracts.GAS.Hash}, chain.contracts.Management.GetNEP17Contracts())
	})

	t.Run("test chain", func(t *testing.T) {
		chain := newTestChain(t)
		initBasicChain(t, chain)
		rublesHash, err := chain.GetContractScriptHash(1)
		require.NoError(t, err)
		require.ElementsMatch(t, []util.Uint160{chain.contracts.NEO.Hash, chain.contracts.GAS.Hash, rublesHash}, chain.contracts.Management.GetNEP17Contracts())
	})
}
