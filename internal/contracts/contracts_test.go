package contracts

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

// TestGenerateHelperContracts generates contract states that are used in tests.
// See generateOracleContract and generateManagementHelperContracts comments for
// details.
func TestGenerateHelperContracts(t *testing.T) {
	const saveState = false

	generateOracleContract(t, saveState)
	generateManagementHelperContracts(t, saveState)

	require.False(t, saveState)
}

// generateOracleContract generates a helper contract that is able to call
// the native Oracle contract and has callback method. It uses testchain to define
// Oracle and StdLib native hashes and saves the generated NEF and manifest to `oracle_contract` folder.
// Set `saveState` flag to true and run the test to rewrite NEF and manifest files.
func generateOracleContract(t *testing.T, saveState bool) {
	ctr := neotest.CompileFile(t, util.Uint160{}, oracleContractModPath, oracleContractYAMLPath)

	// Write NEF file.
	bytes, err := ctr.NEF.Bytes()
	require.NoError(t, err)
	if saveState {
		err = os.WriteFile(oracleContractNEFPath, bytes, os.ModePerm)
		require.NoError(t, err)
	}

	// Write manifest file.
	mData, err := json.Marshal(ctr.Manifest)
	require.NoError(t, err)
	if saveState {
		err = os.WriteFile(oracleContractManifestPath, mData, os.ModePerm)
		require.NoError(t, err)
	}
}

// generateManagementHelperContracts generates 2 helper contracts, second of which is
// allowed to call the first. It uses testchain to define Management and StdLib
// native hashes and saves the generated NEF and manifest to `management_contract` folder.
// Set `saveState` flag to true and run the test to rewrite NEF and manifest files.
func generateManagementHelperContracts(t *testing.T, saveState bool) {
	bc, validator, committee := chain.NewMultiWithCustomConfig(t, func(c *config.Blockchain) {
		c.P2PSigExtensions = true
	})
	e := neotest.NewExecutor(t, bc, validator, committee)

	mgmtHash := e.NativeHash(t, nativenames.Management)
	stdHash := e.NativeHash(t, nativenames.StdLib)
	neoHash := e.NativeHash(t, nativenames.Neo)
	singleChainValidatorAcc := e.Validator.(neotest.MultiSigner).Single(2).Account() // priv0
	require.NoError(t, singleChainValidatorAcc.ConvertMultisig(1, keys.PublicKeys{singleChainValidatorAcc.PublicKey()}))
	singleChainValidatorHash := singleChainValidatorAcc.Contract.ScriptHash()

	w := io.NewBufBinWriter()
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	addOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.ADD, opcode.RET)
	addMultiOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.ADD, opcode.ADD, opcode.RET)
	ret7Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.PUSH7, opcode.RET)
	dropOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.DROP, opcode.RET)
	initOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.INITSSLOT, 1, opcode.PUSH3, opcode.STSFLD0, opcode.RET)
	add3Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.LDSFLD0, opcode.ADD, opcode.RET)
	invalidRetOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.PUSH1, opcode.PUSH2, opcode.RET)
	justRetOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.RET)
	verifyOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.LDSFLD0, opcode.SUB,
		opcode.CONVERT, opcode.Opcode(stackitem.BooleanT), opcode.RET)
	deployOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.SWAP, opcode.JMPIF, 2+8+1+1+1+1+39+3)
	emit.String(w.BinWriter, "create")                                  // 8 bytes
	emit.Int(w.BinWriter, 2)                                            // 1 byte
	emit.Opcodes(w.BinWriter, opcode.PACK)                              // 1 byte
	emit.Int(w.BinWriter, 1)                                            // 1 byte (args count for `serialize`)
	emit.Opcodes(w.BinWriter, opcode.PACK)                              // 1 byte (pack args into array for `serialize`)
	emit.AppCallNoArgs(w.BinWriter, stdHash, "serialize", callflag.All) // 39 bytes
	emit.Opcodes(w.BinWriter, opcode.CALL, 3+8+1+1+1+1+39+3, opcode.RET)
	emit.String(w.BinWriter, "update")                                  // 8 bytes
	emit.Int(w.BinWriter, 2)                                            // 1 byte
	emit.Opcodes(w.BinWriter, opcode.PACK)                              // 1 byte
	emit.Int(w.BinWriter, 1)                                            // 1 byte (args count for `serialize`)
	emit.Opcodes(w.BinWriter, opcode.PACK)                              // 1 byte (pack args into array for `serialize`)
	emit.AppCallNoArgs(w.BinWriter, stdHash, "serialize", callflag.All) // 39 bytes
	emit.Opcodes(w.BinWriter, opcode.CALL, 3, opcode.RET)
	putValOff := w.Len()
	emit.String(w.BinWriter, "initial")
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGetContext)
	emit.Syscall(w.BinWriter, interopnames.SystemStoragePut)
	emit.Opcodes(w.BinWriter, opcode.RET)
	getValOff := w.Len()
	emit.String(w.BinWriter, "initial")
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGetContext)
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGet)
	emit.Opcodes(w.BinWriter, opcode.RET)
	delValOff := w.Len()
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGetContext)
	emit.Syscall(w.BinWriter, interopnames.SystemStorageDelete)
	emit.Opcodes(w.BinWriter, opcode.RET)
	onNEP17PaymentOff := w.Len()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetCallingScriptHash)
	emit.Int(w.BinWriter, 4)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.String(w.BinWriter, "LastPaymentNEP17")
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeNotify)
	emit.Opcodes(w.BinWriter, opcode.RET)
	onNEP11PaymentOff := w.Len()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetCallingScriptHash)
	emit.Int(w.BinWriter, 5)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.String(w.BinWriter, "LastPaymentNEP11")
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeNotify)
	emit.Opcodes(w.BinWriter, opcode.RET)
	update3Off := w.Len()
	emit.Int(w.BinWriter, 3)
	emit.Opcodes(w.BinWriter, opcode.JMP, 2+1)
	updateOff := w.Len()
	emit.Int(w.BinWriter, 2)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.AppCallNoArgs(w.BinWriter, mgmtHash, "update", callflag.All)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	emit.Opcodes(w.BinWriter, opcode.RET)
	destroyOff := w.Len()
	emit.AppCall(w.BinWriter, mgmtHash, "destroy", callflag.All)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	emit.Opcodes(w.BinWriter, opcode.RET)
	invalidStack1Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.NEWARRAY0, opcode.DUP, opcode.DUP, opcode.APPEND) // recursive array
	emit.Opcodes(w.BinWriter, opcode.RET)
	invalidStack2Off := w.Len()
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGetReadOnlyContext) // interop item
	emit.Opcodes(w.BinWriter, opcode.RET)
	callT0Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.CALLT, 0, 0, opcode.PUSH1, opcode.ADD, opcode.RET)
	callT1Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.CALLT, 1, 0, opcode.RET)
	callT2Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.CALLT, 0, 0, opcode.RET)
	burnGasOff := w.Len()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeBurnGas)
	emit.Opcodes(w.BinWriter, opcode.RET)
	invocCounterOff := w.Len()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetInvocationCounter)
	emit.Opcodes(w.BinWriter, opcode.RET)

	script := w.Bytes()
	m := manifest.NewManifest("TestMain")
	m.ABI.Methods = []manifest.Method{
		{
			Name:   "add",
			Offset: addOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("addend1", smartcontract.IntegerType),
				manifest.NewParameter("addend2", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:   "add",
			Offset: addMultiOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("addend1", smartcontract.IntegerType),
				manifest.NewParameter("addend2", smartcontract.IntegerType),
				manifest.NewParameter("addend3", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "ret7",
			Offset:     ret7Off,
			Parameters: []manifest.Parameter{},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "drop",
			Offset:     dropOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       manifest.MethodInit,
			Offset:     initOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "add3",
			Offset: add3Off,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("addend", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "invalidReturn",
			Offset:     invalidRetOff,
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "justReturn",
			Offset:     justRetOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       manifest.MethodVerify,
			Offset:     verifyOff,
			ReturnType: smartcontract.BoolType,
		},
		{
			Name:   manifest.MethodDeploy,
			Offset: deployOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("data", smartcontract.AnyType),
				manifest.NewParameter("isUpdate", smartcontract.BoolType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       "getValue",
			Offset:     getValOff,
			ReturnType: smartcontract.StringType,
		},
		{
			Name:   "putValue",
			Offset: putValOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("value", smartcontract.StringType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "delValue",
			Offset: delValOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("key", smartcontract.StringType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   manifest.MethodOnNEP11Payment,
			Offset: onNEP11PaymentOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("from", smartcontract.Hash160Type),
				manifest.NewParameter("amount", smartcontract.IntegerType),
				manifest.NewParameter("tokenid", smartcontract.ByteArrayType),
				manifest.NewParameter("data", smartcontract.AnyType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   manifest.MethodOnNEP17Payment,
			Offset: onNEP17PaymentOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("from", smartcontract.Hash160Type),
				manifest.NewParameter("amount", smartcontract.IntegerType),
				manifest.NewParameter("data", smartcontract.AnyType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "update",
			Offset: updateOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("nef", smartcontract.ByteArrayType),
				manifest.NewParameter("manifest", smartcontract.ByteArrayType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "update",
			Offset: update3Off,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("nef", smartcontract.ByteArrayType),
				manifest.NewParameter("manifest", smartcontract.ByteArrayType),
				manifest.NewParameter("data", smartcontract.AnyType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       "destroy",
			Offset:     destroyOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       "invalidStack1",
			Offset:     invalidStack1Off,
			ReturnType: smartcontract.AnyType,
		},
		{
			Name:       "invalidStack2",
			Offset:     invalidStack2Off,
			ReturnType: smartcontract.AnyType,
		},
		{
			Name:   "callT0",
			Offset: callT0Off,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("address", smartcontract.Hash160Type),
			},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "callT1",
			Offset:     callT1Off,
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "callT2",
			Offset:     callT2Off,
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:   "burnGas",
			Offset: burnGasOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("amount", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       "invocCounter",
			Offset:     invocCounterOff,
			ReturnType: smartcontract.IntegerType,
		},
	}
	m.ABI.Events = []manifest.Event{
		{
			Name: "LastPaymentNEP17",
			Parameters: []manifest.Parameter{
				{
					Name: "from",
					Type: smartcontract.Hash160Type,
				},
				{
					Name: "to",
					Type: smartcontract.Hash160Type,
				},
				{
					Name: "amount",
					Type: smartcontract.IntegerType,
				},
				{
					Name: "data",
					Type: smartcontract.AnyType,
				},
			},
		},
		{
			Name: "LastPaymentNEP11",
			Parameters: []manifest.Parameter{
				{
					Name: "from",
					Type: smartcontract.Hash160Type,
				},
				{
					Name: "to",
					Type: smartcontract.Hash160Type,
				},
				{
					Name: "amount",
					Type: smartcontract.IntegerType,
				},
				{
					Name: "tokenId",
					Type: smartcontract.ByteArrayType,
				},
				{
					Name: "data",
					Type: smartcontract.AnyType,
				},
			},
		},
		{
			Name: "event", // This event is not emitted by the contract code and needed for System.Runtime.Notify tests.
			Parameters: []manifest.Parameter{
				{
					Name: "any",
					Type: smartcontract.AnyType,
				},
			},
		},
	}
	m.Permissions = make([]manifest.Permission, 2)
	m.Permissions[0].Contract.Type = manifest.PermissionHash
	m.Permissions[0].Contract.Value = neoHash
	m.Permissions[0].Methods.Add("balanceOf")

	m.Permissions[1].Contract.Type = manifest.PermissionHash
	m.Permissions[1].Contract.Value = util.Uint160{}
	m.Permissions[1].Methods.Add("method")

	// Generate NEF file.
	ne, err := nef.NewFile(script)
	require.NoError(t, err)
	ne.Tokens = []nef.MethodToken{
		{
			Hash:       neoHash,
			Method:     "balanceOf",
			ParamCount: 1,
			HasReturn:  true,
			CallFlag:   callflag.ReadStates,
		},
		{
			Hash:      util.Uint160{},
			Method:    "method",
			HasReturn: true,
			CallFlag:  callflag.ReadStates,
		},
	}
	ne.Checksum = ne.CalculateChecksum()

	// Write first NEF file.
	bytes, err := ne.Bytes()
	require.NoError(t, err)
	if saveState {
		err = os.WriteFile(helper1ContractNEFPath, bytes, os.ModePerm)
		require.NoError(t, err)
	}

	// Write first manifest file.
	mData, err := json.Marshal(m)
	require.NoError(t, err)
	if saveState {
		err = os.WriteFile(helper1ContractManifestPath, mData, os.ModePerm)
		require.NoError(t, err)
	}

	// Create hash of the first contract assuming that sender is single-chain validator.
	h := state.CreateContractHash(singleChainValidatorHash, ne.Checksum, m.Name)

	currScript := []byte{byte(opcode.RET)}
	m = manifest.NewManifest("TestAux")
	m.ABI.Methods = []manifest.Method{
		{
			Name:       "simpleMethod",
			Offset:     0,
			ReturnType: smartcontract.VoidType,
		},
	}
	perm := manifest.NewPermission(manifest.PermissionHash, h)
	perm.Methods.Add("add")
	perm.Methods.Add("drop")
	perm.Methods.Add("add3")
	perm.Methods.Add("invalidReturn")
	perm.Methods.Add("justReturn")
	perm.Methods.Add("getValue")
	m.Permissions = append(m.Permissions, *perm)
	ne, err = nef.NewFile(currScript)
	require.NoError(t, err)

	// Write second NEF file.
	bytes, err = ne.Bytes()
	require.NoError(t, err)
	if saveState {
		err = os.WriteFile(helper2ContractNEFPath, bytes, os.ModePerm)
		require.NoError(t, err)
	}

	// Write second manifest file.
	mData, err = json.Marshal(m)
	require.NoError(t, err)
	if saveState {
		err = os.WriteFile(helper2ContractManifestPath, mData, os.ModePerm)
		require.NoError(t, err)
	}
}
