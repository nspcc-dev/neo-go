package interop_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/limits"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const benchGasLimit int64 = 50_0000_0000

func newBenchExecutor(b *testing.B) *neotest.Executor {
	bc, acc := chain.NewSingleWithOptions(b, &chain.Options{
		BlockchainConfigHook: func(c *config.Blockchain) {
			//c.Hardforks = map[string]uint32{config.HFGorgon.String(): 0}
			c.Hardforks = map[string]uint32{config.HFHuyao.String(): 0}
		},
		Logger: zap.NewNop(),
	})
	return neotest.NewExecutor(b, bc, acc, acc)
}

func benchInteropLoop(b *testing.B, e *neotest.Executor, script []byte, caller *manifest.Manifest, signers ...transaction.Signer) {
	ic, err := e.Chain.GetTestVM(trigger.Application, &transaction.Transaction{Signers: signers}, nil)
	require.NoError(b, err)
	var callerNEF *nef.File
	if caller != nil {
		callerNEF, err = nef.NewFile(script)
		require.NoError(b, err)
	}

	for b.Loop() {
		b.StopTimer()
		ic.VM.SetGasLimit(benchGasLimit)
		if caller != nil {
			ic.VM.LoadNEFMethod(callerNEF, caller, util.Uint160{},
				state.CreateContractHash(e.Validator.ScriptHash(), callerNEF.Checksum, caller.Name),
				callflag.All, false, 0, -1, nil, nil, false)
		} else {
			ic.VM.LoadScriptWithFlags(script, callflag.All)
		}
		b.StartTimer()
		err := ic.VM.Run()
		b.StopTimer()
		require.ErrorContains(b, err, "GAS limit exceeded")
		ic.Finalize()
		ic.Notifications = nil
		ic.ReuseVM(ic.VM)
		b.StartTimer()
	}
}

// --- System.Contract.* ---

func BenchmarkContractCall(b *testing.B) {
	calleeNEF, err := nef.NewFile([]byte{byte(opcode.RET)})
	require.NoError(b, err)

	deployCallee := func(b *testing.B, e *neotest.Executor, md manifest.Method) util.Uint160 {
		m := manifest.NewManifest("callee")
		m.ABI.Methods = []manifest.Method{md}
		callee := &neotest.Contract{
			Hash:     state.CreateContractHash(e.Validator.ScriptHash(), calleeNEF.Checksum, m.Name),
			NEF:      calleeNEF,
			Manifest: m,
		}
		e.DeployContract(b, callee, nil)
		return callee.Hash
	}

	b.Run("best", func(b *testing.B) {
		e := newBenchExecutor(b)
		calleeHash := deployCallee(b, e, manifest.Method{
			Name:       "target",
			ReturnType: smartcontract.VoidType,
			Safe:       true,
		})

		w := io.NewBufBinWriter()
		emit.AppCall(w.BinWriter, calleeHash, "target", callflag.NoneFlag)
		emit.Opcodes(w.BinWriter, opcode.DROP)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-w.Len())})
		require.NoError(b, w.Err)

		benchInteropLoop(b, e, w.Bytes(), nil)
	})
	b.Run("worst", func(b *testing.B) {
		e := newBenchExecutor(b)
		calleeHash := deployCallee(b, e, manifest.Method{
			Name:       "target",
			Parameters: []manifest.Parameter{manifest.NewParameter("arr", smartcontract.ArrayType)},
			ReturnType: smartcontract.ArrayType,
		})

		permissionsCount := getMaxManifestPermissionsCount()
		caller := manifest.NewManifest("caller")
		caller.Permissions = make([]manifest.Permission, permissionsCount)
		for j := range permissionsCount - 1 {
			caller.Permissions[j] = *manifest.NewPermission(manifest.PermissionHash, random.Uint160())
		}
		caller.Permissions[permissionsCount-1] = *manifest.NewPermission(manifest.PermissionHash, calleeHash)

		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, vm.MaxStackSize-5)
		emit.Opcodes(w.BinWriter, opcode.NEWARRAY)
		beginInteropCall := w.Len()
		emit.Opcodes(w.BinWriter, opcode.PUSH1, opcode.PACK)
		emit.AppCallNoArgs(w.BinWriter, calleeHash, "target", callflag.All)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})
		require.NoError(b, w.Err)

		benchInteropLoop(b, e, w.Bytes(), caller)
	})
}

func BenchmarkContractCallNative(b *testing.B) {
	b.Run("best", func(b *testing.B) {
		w := io.NewBufBinWriter()
		emit.AppCall(w.BinWriter, nativehashes.GasToken, "decimals", callflag.NoneFlag)
		emit.Opcodes(w.BinWriter, opcode.DROP)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-w.Len())})
		require.NoError(b, w.Err)

		benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
	})
	b.Run("worst", func(b *testing.B) {
		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, vm.MaxStackSize-5)
		emit.Opcodes(w.BinWriter, opcode.NEWARRAY, opcode.PUSH1, opcode.PACK)
		emit.AppCallNoArgs(w.BinWriter, nativehashes.StdLib, "serialize", callflag.NoneFlag)
		emit.Opcodes(w.BinWriter, opcode.DROP)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-w.Len())})
		require.NoError(b, w.Err)

		benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
	})
}

func BenchmarkCreateMultisigAccount(b *testing.B) {
	priv, err := keys.NewPrivateKey()
	require.NoError(b, err)
	pub := priv.PublicKey().Bytes()

	getCreateMultisigScript := func(pubkeysCount int64) []byte {
		w := io.NewBufBinWriter()
		emit.Instruction(w.BinWriter, opcode.INITSSLOT, []byte{2})
		emit.Int(w.BinWriter, pubkeysCount)
		emit.Opcodes(w.BinWriter, opcode.PUSH0, opcode.STSFLD0, opcode.STSFLD1)
		beginCycle := w.Len()
		emit.Bytes(w.BinWriter, pub)
		emit.Opcodes(w.BinWriter, opcode.LDSFLD0, opcode.INC, opcode.DUP,
			opcode.STSFLD0, opcode.LDSFLD1,
		)
		emit.Instruction(w.BinWriter, opcode.JMPLT, []byte{byte(-(w.Len() - beginCycle))})
		emit.Opcodes(w.BinWriter, opcode.LDSFLD1, opcode.PACK)
		beginInteropCall := w.Len()
		emit.Opcodes(w.BinWriter, opcode.DUP, opcode.PUSH1)
		emit.Syscall(w.BinWriter, interopnames.SystemContractCreateMultisigAccount)
		emit.Opcodes(w.BinWriter, opcode.DROP)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})

		return w.Bytes()
	}

	b.Run("best", func(b *testing.B) {
		benchInteropLoop(b, newBenchExecutor(b), getCreateMultisigScript(1), nil)
	})
	b.Run("worst", func(b *testing.B) {
		benchInteropLoop(b, newBenchExecutor(b), getCreateMultisigScript(1024), nil)
	})
}

func BenchmarkCreateStandardAccount(b *testing.B) {
	priv, err := keys.NewPrivateKey()
	require.NoError(b, err)
	pub := priv.PublicKey().Bytes()

	w := io.NewBufBinWriter()
	emit.Bytes(w.BinWriter, pub)
	emit.Syscall(w.BinWriter, interopnames.SystemContractCreateStandardAccount)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func BenchmarkGetCallFlags(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemContractGetCallFlags)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func getMaxManifestPermissionsCount() int {
	i := sort.Search(manifest.MaxManifestSize, func(n int) bool {
		m := manifest.NewManifest("caller")
		m.Permissions = make([]manifest.Permission, n)
		for j := range n {
			m.Permissions[j] = *manifest.NewPermission(manifest.PermissionHash, random.Uint160())
		}
		data, _ := json.Marshal(m)
		return len(data) > manifest.MaxManifestSize
	})
	return i - 1
}

// --- System.Crypto.* ---

func BenchmarkCheckMultisig(b *testing.B) {
	priv, err := keys.NewPrivateKey()
	require.NoError(b, err)
	pub := priv.PublicKey().Bytes()

	getCheckMultisigScript := func(sig []byte, count int64) []byte {
		w := io.NewBufBinWriter()
		emit.Instruction(w.BinWriter, opcode.INITSSLOT, []byte{2})
		for _, elem := range [][]byte{sig, pub} {
			emit.Int(w.BinWriter, count)
			emit.Opcodes(w.BinWriter, opcode.PUSH0, opcode.STSFLD0, opcode.STSFLD1)
			beginCycle := w.Len()
			emit.Bytes(w.BinWriter, elem)
			emit.Opcodes(w.BinWriter, opcode.LDSFLD0, opcode.INC, opcode.DUP,
				opcode.STSFLD0, opcode.LDSFLD1,
			)
			emit.Instruction(w.BinWriter, opcode.JMPLT, []byte{byte(-(w.Len() - beginCycle))})
			emit.Opcodes(w.BinWriter, opcode.LDSFLD1, opcode.PACK)
		}
		beginInteropCall := w.Len()
		emit.Opcodes(w.BinWriter, opcode.OVER, opcode.OVER)
		emit.Syscall(w.BinWriter, interopnames.SystemCryptoCheckMultisig)
		emit.Opcodes(w.BinWriter, opcode.DROP)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})

		return w.Bytes()
	}

	b.Run("best", func(b *testing.B) {
		e := newBenchExecutor(b)
		sig := priv.SignHashable(uint32(e.Chain.GetConfig().Magic), &transaction.Transaction{})
		benchInteropLoop(b, e, getCheckMultisigScript(sig, 1), nil)
	})
	b.Run("worst", func(b *testing.B) {
		e := newBenchExecutor(b)
		sig := priv.SignHashable(uint32(e.Chain.GetConfig().Magic), &transaction.Transaction{})
		benchInteropLoop(b, e, getCheckMultisigScript(sig, (vm.MaxStackSize-6)/2), nil)
	})
}

func BenchmarkCheckSig(b *testing.B) {
	priv, err := keys.NewPrivateKey()
	require.NoError(b, err)
	pub := priv.PublicKey().Bytes()

	e := newBenchExecutor(b)
	sig := priv.SignHashable(uint32(e.Chain.GetConfig().Magic), &transaction.Transaction{})

	w := io.NewBufBinWriter()
	emit.Bytes(w.BinWriter, sig)
	emit.Bytes(w.BinWriter, pub)
	emit.Syscall(w.BinWriter, interopnames.SystemCryptoCheckSig)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, e, w.Bytes(), nil)
}

// --- System.Iterator.* ---

func BenchmarkIteratorNext(b *testing.B) {
	e := newBenchExecutor(b)

	w := io.NewBufBinWriter()
	emit.Int(w.BinWriter, istorage.FindDefault)
	emit.Bytes(w.BinWriter, []byte{})
	emit.Syscall(w.BinWriter, interopnames.SystemStorageLocalFind)
	beginInteropCall := w.Len()
	emit.Opcodes(w.BinWriter, opcode.DUP)
	emit.Syscall(w.BinWriter, interopnames.SystemIteratorNext)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})
	require.NoError(b, w.Err)
	script := w.Bytes()

	scriptNEF, err := nef.NewFile(script)
	require.NoError(b, err)
	m := manifest.NewManifest("iterator")
	m.ABI.Methods = []manifest.Method{{Name: "main", ReturnType: smartcontract.VoidType}}
	e.DeployContract(b, &neotest.Contract{
		Hash:     state.CreateContractHash(e.Validator.ScriptHash(), scriptNEF.Checksum, m.Name),
		NEF:      scriptNEF,
		Manifest: m,
	}, nil)

	benchInteropLoop(b, e, script, m)
}

func BenchmarkIteratorValue(b *testing.B) {
	key := []byte{0x01}

	getIteratorValueContract := func(opts int64, prefix []byte, zerosLen int) ([]byte, *manifest.Manifest) {
		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, opts)
		emit.Bytes(w.BinWriter, key)
		emit.Syscall(w.BinWriter, interopnames.SystemStorageLocalFind)
		emit.Opcodes(w.BinWriter, opcode.DUP)
		emit.Syscall(w.BinWriter, interopnames.SystemIteratorNext)
		emit.Opcodes(w.BinWriter, opcode.DROP)
		beginInteropCall := w.Len()
		emit.Opcodes(w.BinWriter, opcode.DUP)
		emit.Syscall(w.BinWriter, interopnames.SystemIteratorValue)
		emit.Opcodes(w.BinWriter, opcode.DROP)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})

		deployOffset := w.Len()
		emit.Opcodes(w.BinWriter, opcode.DROP, opcode.DROP)
		emit.Bytes(w.BinWriter, prefix)
		if zerosLen > 0 {
			emit.Int(w.BinWriter, int64(zerosLen))
			emit.Opcodes(w.BinWriter, opcode.NEWBUFFER, opcode.CAT)
		}
		emit.Bytes(w.BinWriter, key)
		emit.Syscall(w.BinWriter, interopnames.SystemStorageLocalPut)
		emit.Opcodes(w.BinWriter, opcode.RET)
		require.NoError(b, w.Err)

		m := manifest.NewManifest("iterator")
		m.ABI.Methods = []manifest.Method{
			{Name: "main", ReturnType: smartcontract.VoidType},
			{
				Name:   manifest.MethodDeploy,
				Offset: deployOffset,
				Parameters: []manifest.Parameter{
					manifest.NewParameter("data", smartcontract.AnyType),
					manifest.NewParameter("isUpdate", smartcontract.BoolType),
				},
				ReturnType: smartcontract.VoidType,
			},
		}
		return w.Bytes(), m
	}

	run := func(b *testing.B, opts int64, prefix []byte, zerosLen int) {
		e := newBenchExecutor(b)
		script, m := getIteratorValueContract(opts, prefix, zerosLen)
		scriptNEF, err := nef.NewFile(script)
		require.NoError(b, err)
		e.DeployContract(b, &neotest.Contract{
			Hash:     state.CreateContractHash(e.Validator.ScriptHash(), scriptNEF.Checksum, m.Name),
			NEF:      scriptNEF,
			Manifest: m,
		}, nil)

		benchInteropLoop(b, e, script, m)
	}

	b.Run("best", func(b *testing.B) {
		run(b, istorage.FindValuesOnly, []byte{0x01}, 0)
	})
	b.Run("worst", func(b *testing.B) {
		nested := func(bytesLen int) stackitem.Item {
			item := stackitem.Item(stackitem.NewByteArray(make([]byte, bytesLen)))
			for range vm.MaxStackSize - 2 {
				item = stackitem.NewArray([]stackitem.Item{item})
			}
			return item
		}
		data, err := stackitem.Serialize(nested(limits.MaxStorageValueLen))
		require.NoError(b, err)
		bytesLen := limits.MaxStorageValueLen - (len(data) - limits.MaxStorageValueLen)
		data, err = stackitem.Serialize(nested(bytesLen))
		require.NoError(b, err)
		require.LessOrEqual(b, len(data), limits.MaxStorageValueLen)

		run(b, istorage.FindValuesOnly|istorage.FindDeserialize, data[:len(data)-bytesLen], bytesLen)
	})
}

// --- System.Runtime.* ---

func BenchmarkBurnGas(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Opcodes(w.BinWriter, opcode.PUSH1)
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeBurnGas)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

const maxSubitems = 16

func getMaxManifestGroupsCount(b *testing.B, c *neotest.Contract) int {
	priv, err := keys.NewPrivateKey()
	require.NoError(b, err)
	nefBytes, err := c.NEF.Bytes()
	require.NoError(b, err)
	i := sort.Search(manifest.MaxManifestSize, func(n int) bool {
		m := *c.Manifest
		m.Groups = make([]manifest.Group, n)
		for j := range n {
			m.Groups[j] = manifest.Group{PublicKey: priv.PublicKey(), Signature: make([]byte, keys.SignatureLen)}
		}
		data, err := json.Marshal(m)
		require.NoError(b, err)
		script, err := smartcontract.CreateCallScript(nativehashes.ContractManagement, "deploy", nefBytes, data, nil)
		require.NoError(b, err)
		return len(data) > manifest.MaxManifestSize || len(script) > transaction.MaxScriptLength
	})
	return i - 1
}

func BenchmarkCheckWitness(b *testing.B) {
	newContract := func(e *neotest.Executor, name string, script []byte) *neotest.Contract {
		scriptNEF, err := nef.NewFile(script)
		require.NoError(b, err)
		m := manifest.NewManifest(name)
		m.ABI.Methods = []manifest.Method{{Name: "main", ReturnType: smartcontract.VoidType}}
		return &neotest.Contract{
			Hash:     state.CreateContractHash(e.Validator.ScriptHash(), scriptNEF.Checksum, name),
			NEF:      scriptNEF,
			Manifest: m,
		}
	}
	callScript := func(h util.Uint160) []byte {
		w := io.NewBufBinWriter()
		emit.AppCall(w.BinWriter, h, "main", callflag.All)
		require.NoError(b, w.Err)
		return w.Bytes()
	}
	checkWitnessLoopScript := func(w *io.BufBinWriter) []byte {
		beginInteropCall := w.Len()
		emit.Opcodes(w.BinWriter, opcode.DUP)
		emit.Syscall(w.BinWriter, interopnames.SystemRuntimeCheckWitness)
		emit.Opcodes(w.BinWriter, opcode.DROP)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})
		require.NoError(b, w.Err)
		return w.Bytes()
	}

	b.Run("best", func(b *testing.B) {
		e := newBenchExecutor(b)
		w := io.NewBufBinWriter()
		emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetCallingScriptHash)
		target := newContract(e, "target", checkWitnessLoopScript(w))
		e.DeployContract(b, target, nil)

		benchInteropLoop(b, e, callScript(target.Hash), nil)
	})
	b.Run("worst", func(b *testing.B) {
		e := newBenchExecutor(b)
		checkedHash := random.Uint160()
		w := io.NewBufBinWriter()
		emit.Bytes(w.BinWriter, checkedHash.BytesBE())
		target := newContract(e, "target", checkWitnessLoopScript(w))
		groupsCount := getMaxManifestGroupsCount(b, target)
		target.Manifest.Groups = make([]manifest.Group, groupsCount)
		for i := range groupsCount {
			priv, err := keys.NewPrivateKey()
			require.NoError(b, err)
			target.Manifest.Groups[i] = manifest.Group{
				PublicKey: priv.PublicKey(),
				Signature: priv.SignHash(hash.Sha256(target.Hash.BytesBE())),
			}
		}
		e.DeployContract(b, target, nil)

		allowedGroupKey, err := keys.NewPrivateKey()
		require.NoError(b, err)
		allowedGroups := make([]*keys.PublicKey, maxSubitems)
		for i := range allowedGroups {
			allowedGroups[i] = allowedGroupKey.PublicKey()
		}
		allowedContracts := make([]util.Uint160, maxSubitems)
		for i := range allowedContracts {
			allowedContracts[i] = random.Uint160()
		}
		rules := make([]transaction.WitnessRule, maxSubitems)
		for i := range rules {
			top := make(transaction.ConditionOr, maxSubitems)
			for j := range top {
				mid := make(transaction.ConditionOr, maxSubitems)
				for k := range mid {
					csh := transaction.ConditionScriptHash(random.Uint160())
					mid[k] = &csh
				}
				top[j] = &mid
			}
			rules[i] = transaction.WitnessRule{Action: transaction.WitnessDeny, Condition: &top}
		}
		signers := make([]transaction.Signer, transaction.MaxAttributes)
		for i := range transaction.MaxAttributes - 1 {
			signers[i] = transaction.Signer{Account: random.Uint160()}
		}
		signers[transaction.MaxAttributes-1] = transaction.Signer{
			Account:          checkedHash,
			Scopes:           transaction.CustomContracts | transaction.CustomGroups | transaction.Rules,
			AllowedContracts: allowedContracts,
			AllowedGroups:    allowedGroups,
			Rules:            rules,
		}

		benchInteropLoop(b, e, target.NEF.Script, target.Manifest, signers...)
	})
}

func BenchmarkCurrentSigners(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeCurrentSigners)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)
	script := w.Bytes()

	b.Run("best", func(b *testing.B) {
		signer := transaction.Signer{Account: random.Uint160(), Scopes: transaction.CalledByEntry}
		benchInteropLoop(b, newBenchExecutor(b), script, nil, signer)
	})
	b.Run("worst", func(b *testing.B) {
		groupKey, err := keys.NewPrivateKey()
		require.NoError(b, err)
		allowedGroups := make([]*keys.PublicKey, maxSubitems)
		for i := range allowedGroups {
			allowedGroups[i] = groupKey.PublicKey()
		}
		allowedContracts := make([]util.Uint160, maxSubitems)
		for i := range allowedContracts {
			allowedContracts[i] = random.Uint160()
		}
		rules := make([]transaction.WitnessRule, maxSubitems)
		for i := range rules {
			csh := transaction.ConditionScriptHash(random.Uint160())
			rules[i] = transaction.WitnessRule{Action: transaction.WitnessDeny, Condition: &csh}
		}
		signers := make([]transaction.Signer, transaction.MaxAttributes)
		for i := range signers {
			signers[i] = transaction.Signer{
				Account:          random.Uint160(),
				Scopes:           transaction.CustomContracts | transaction.CustomGroups | transaction.Rules,
				AllowedContracts: allowedContracts,
				AllowedGroups:    allowedGroups,
				Rules:            rules,
			}
		}
		benchInteropLoop(b, newBenchExecutor(b), script, nil, signers...)
	})
}

func BenchmarkGasLeft(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGasLeft)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func BenchmarkGetAddressVersion(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetAddressVersion)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func BenchmarkGetCallingScriptHash(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetCallingScriptHash)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func BenchmarkGetEntryScriptHash(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetEntryScriptHash)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func BenchmarkGetExecutingScriptHash(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetExecutingScriptHash)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func BenchmarkGetInvocationCounter(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetInvocationCounter)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func BenchmarkGetNetwork(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetNetwork)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func getMaxNotificationsCount() int {
	return min(interop.MaxNotificationCount, (vm.MaxStackSize-2)/4)
}

func BenchmarkGetNotifications(b *testing.B) {
	getNotificationsLoop := func(w *io.BufBinWriter, pushFilter opcode.Opcode) []byte {
		beginInteropCall := w.Len()
		emit.Opcodes(w.BinWriter, pushFilter)
		emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetNotifications)
		emit.Opcodes(w.BinWriter, opcode.DROP)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})
		require.NoError(b, w.Err)
		return w.Bytes()
	}

	b.Run("best", func(b *testing.B) {
		benchInteropLoop(b, newBenchExecutor(b), getNotificationsLoop(io.NewBufBinWriter(), opcode.PUSHNULL), nil)
	})
	b.Run("worst", func(b *testing.B) {
		const eventName = "e"
		e := newBenchExecutor(b)

		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, int64(getMaxNotificationsCount()))
		beginCycle := w.Len()
		emit.Opcodes(w.BinWriter, opcode.NEWARRAY0)
		emit.String(w.BinWriter, eventName)
		emit.Syscall(w.BinWriter, interopnames.SystemRuntimeNotify)
		emit.Opcodes(w.BinWriter, opcode.DEC, opcode.DUP)
		emit.Instruction(w.BinWriter, opcode.JMPIF, []byte{byte(-(w.Len() - beginCycle))})
		emit.Opcodes(w.BinWriter, opcode.DROP)
		emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetExecutingScriptHash)
		script := getNotificationsLoop(w, opcode.DUP)

		scriptNEF, err := nef.NewFile(script)
		require.NoError(b, err)
		m := manifest.NewManifest("notifier")
		m.ABI.Methods = []manifest.Method{{Name: "main", ReturnType: smartcontract.VoidType}}
		m.ABI.Events = []manifest.Event{{Name: eventName, Parameters: []manifest.Parameter{}}}
		e.DeployContract(b, &neotest.Contract{
			Hash:     state.CreateContractHash(e.Validator.ScriptHash(), scriptNEF.Checksum, m.Name),
			NEF:      scriptNEF,
			Manifest: m,
		}, nil)

		benchInteropLoop(b, e, script, m)
	})
}

func BenchmarkGetRandom(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetRandom)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func BenchmarkGetScriptContainer(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetScriptContainer)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil, transaction.Signer{Account: random.Uint160()})
}

func BenchmarkGetTime(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetTime)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func BenchmarkGetTrigger(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetTrigger)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func BenchmarkLoadScript(b *testing.B) {
	b.Run("best", func(b *testing.B) {
		w := io.NewBufBinWriter()
		emit.Opcodes(w.BinWriter, opcode.NEWARRAY0)
		emit.Int(w.BinWriter, int64(callflag.All))
		emit.Bytes(w.BinWriter, []byte{})
		emit.Syscall(w.BinWriter, interopnames.SystemRuntimeLoadScript)
		emit.Opcodes(w.BinWriter, opcode.DROP)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-w.Len())})
		require.NoError(b, w.Err)

		benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
	})
	b.Run("worst", func(b *testing.B) {
		loaded := make([]byte, stackitem.MaxSize)
		for i := range loaded {
			loaded[i] = byte(opcode.RET)
		}

		w := io.NewBufBinWriter()
		emit.Bytes(w.BinWriter, loaded)
		emit.Int(w.BinWriter, vm.MaxStackSize-5)
		emit.Opcodes(w.BinWriter, opcode.NEWARRAY)
		beginInteropCall := w.Len()
		emit.Opcodes(w.BinWriter, opcode.PUSH1, opcode.PACK)
		emit.Int(w.BinWriter, int64(callflag.All))
		emit.Opcodes(w.BinWriter, opcode.PUSH2, opcode.PICK)
		emit.Syscall(w.BinWriter, interopnames.SystemRuntimeLoadScript)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})
		require.NoError(b, w.Err)

		benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
	})
}

func BenchmarkLog(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Bytes(w.BinWriter, make([]byte, runtime.MaxNotificationSize))
	beginInteropCall := w.Len()
	emit.Opcodes(w.BinWriter, opcode.DUP)
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeLog)
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

func getMaxManifestEventsCount(b *testing.B, c *neotest.Contract, setEvents func(m *manifest.Manifest, n int)) int {
	nefBytes, err := c.NEF.Bytes()
	require.NoError(b, err)
	i := sort.Search(manifest.MaxManifestSize, func(n int) bool {
		m := *c.Manifest
		setEvents(&m, n)
		data, err := json.Marshal(m)
		require.NoError(b, err)
		script, err := smartcontract.CreateCallScript(nativehashes.ContractManagement, "deploy", nefBytes, data, nil)
		require.NoError(b, err)
		cs := state.Contract{ContractBase: state.ContractBase{NEF: *c.NEF, Manifest: m}}
		item, err := cs.ToStackItem()
		require.NoError(b, err)
		_, err = stackitem.Serialize(item)
		return len(data) > manifest.MaxManifestSize || len(script) > transaction.MaxScriptLength || err != nil
	})
	return i - 1
}

func BenchmarkNotify(b *testing.B) {
	const eventName = "target"

	newEmitter := func(e *neotest.Executor) *neotest.Contract {
		w := io.NewBufBinWriter()
		emit.Int(w.BinWriter, interop.MaxNotificationCount)
		beginCycle := w.Len()
		emit.Opcodes(w.BinWriter, opcode.OVER)
		emit.String(w.BinWriter, eventName)
		emit.Syscall(w.BinWriter, interopnames.SystemRuntimeNotify)
		emit.Opcodes(w.BinWriter, opcode.DEC, opcode.DUP)
		emit.Instruction(w.BinWriter, opcode.JMPIF, []byte{byte(-(w.Len() - beginCycle))})
		emit.Opcodes(w.BinWriter, opcode.THROW)
		require.NoError(b, w.Err)
		emitterNEF, err := nef.NewFile(w.Bytes())
		require.NoError(b, err)
		m := manifest.NewManifest("emitter")
		m.ABI.Methods = []manifest.Method{{
			Name:       "main",
			Parameters: []manifest.Parameter{manifest.NewParameter("args", smartcontract.ArrayType)},
			ReturnType: smartcontract.VoidType,
		}}
		return &neotest.Contract{
			Hash:     state.CreateContractHash(e.Validator.ScriptHash(), emitterNEF.Checksum, m.Name),
			NEF:      emitterNEF,
			Manifest: m,
		}
	}

	notifyLoopScript := func(w *io.BufBinWriter, emitterHash util.Uint160) []byte {
		beginInteropCall := w.Len()
		tryPos := w.Len()
		emit.Instruction(w.BinWriter, opcode.TRY, []byte{0, 0})
		emit.Opcodes(w.BinWriter, opcode.DUP, opcode.PUSH1, opcode.PACK)
		emit.AppCallNoArgs(w.BinWriter, emitterHash, "main", callflag.All)
		tryEndPos := w.Len()
		emit.Instruction(w.BinWriter, opcode.ENDTRY, []byte{0})
		catchPos := w.Len()
		emit.Opcodes(w.BinWriter, opcode.DROP)
		catchEndPos := w.Len()
		emit.Instruction(w.BinWriter, opcode.ENDTRY, []byte{0})
		endPos := w.Len()
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})
		require.NoError(b, w.Err)

		script := w.Bytes()
		script[tryPos+1] = byte(catchPos - tryPos)
		script[tryEndPos+1] = byte(endPos - tryEndPos)
		script[catchEndPos+1] = byte(endPos - catchEndPos)
		return script
	}

	b.Run("best", func(b *testing.B) {
		e := newBenchExecutor(b)
		emitter := newEmitter(e)
		emitter.Manifest.ABI.Events = []manifest.Event{{Name: eventName, Parameters: []manifest.Parameter{}}}
		e.DeployContract(b, emitter, nil)

		w := io.NewBufBinWriter()
		emit.Opcodes(w.BinWriter, opcode.NEWARRAY0)
		benchInteropLoop(b, e, notifyLoopScript(w, emitter.Hash), nil)
	})
	b.Run("worst", func(b *testing.B) {
		e := newBenchExecutor(b)
		emitter := newEmitter(e)
		setEvents := func(m *manifest.Manifest, n int) {
			m.ABI.Events = make([]manifest.Event, n)
			for i := range n - 1 {
				m.ABI.Events[i] = manifest.Event{Name: fmt.Sprintf("e%d", i), Parameters: []manifest.Parameter{}}
			}
			m.ABI.Events[n-1] = manifest.Event{
				Name:       eventName,
				Parameters: []manifest.Parameter{manifest.NewParameter("item", smartcontract.AnyType)},
			}
		}
		eventsCount := getMaxManifestEventsCount(b, emitter, setEvents)
		setEvents(emitter.Manifest, eventsCount)
		e.DeployContract(b, emitter, nil)

		nested := func(depth int) stackitem.Item {
			var item stackitem.Item = stackitem.Null{}
			for range depth - 1 {
				item = stackitem.NewArray([]stackitem.Item{item})
			}
			return item
		}
		depth := sort.Search(vm.MaxStackSize, func(n int) bool {
			data, err := stackitem.Serialize(stackitem.NewArray([]stackitem.Item{nested(n)}))
			return err != nil || len(data) > runtime.MaxNotificationSize
		}) - 1

		w := io.NewBufBinWriter()
		emit.Opcodes(w.BinWriter, opcode.PUSHNULL)
		emit.Int(w.BinWriter, int64(depth-1))
		beginCycle := w.Len()
		emit.Opcodes(w.BinWriter, opcode.SWAP, opcode.PUSH1, opcode.PACK, opcode.SWAP, opcode.DEC, opcode.DUP)
		emit.Instruction(w.BinWriter, opcode.JMPIF, []byte{byte(-(w.Len() - beginCycle))})
		emit.Opcodes(w.BinWriter, opcode.DROP, opcode.PUSH1, opcode.PACK)
		benchInteropLoop(b, e, notifyLoopScript(w, emitter.Hash), nil)
	})
}

func BenchmarkPlatform(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimePlatform)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, newBenchExecutor(b), w.Bytes(), nil)
}

// --- System.Storage.* ---

func benchStorageLoop(b *testing.B, emitInteropCall func(w *io.BinWriter)) {
	benchContractLoop(b, func(w *io.BinWriter) {
		emit.Bytes(w, []byte{0x42})
		emit.Bytes(w, []byte{0x01})
		emit.Syscall(w, interopnames.SystemStorageLocalPut)
	}, emitInteropCall)
}

func benchContractLoop(b *testing.B, emitSetup, emitInteropCall func(w *io.BinWriter)) {
	e := newBenchExecutor(b)

	w := io.NewBufBinWriter()
	if emitSetup != nil {
		emitSetup(w.BinWriter)
	}
	beginInteropCall := w.Len()
	emitInteropCall(w.BinWriter)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-(w.Len() - beginInteropCall))})
	require.NoError(b, w.Err)
	script := w.Bytes()

	scriptNEF, err := nef.NewFile(script)
	require.NoError(b, err)
	m := manifest.NewManifest("storage")
	m.ABI.Methods = []manifest.Method{{Name: "main", ReturnType: smartcontract.VoidType}}
	e.DeployContract(b, &neotest.Contract{
		Hash:     state.CreateContractHash(e.Validator.ScriptHash(), scriptNEF.Checksum, m.Name),
		NEF:      scriptNEF,
		Manifest: m,
	}, nil)

	benchInteropLoop(b, e, script, m)
}

func BenchmarkStorageGet(b *testing.B) {
	benchStorageLoop(b, func(w *io.BinWriter) {
		emit.Bytes(w, []byte{0x01})
		emit.Syscall(w, interopnames.SystemStorageLocalGet)
	})
}

func BenchmarkStorageFind(b *testing.B) {
	benchStorageLoop(b, func(w *io.BinWriter) {
		emit.Int(w, istorage.FindDefault)
		emit.Bytes(w, []byte{0x01})
		emit.Syscall(w, interopnames.SystemStorageLocalFind)
	})
}

func BenchmarkStorageGetContext(b *testing.B) {
	benchContractLoop(b, nil, func(w *io.BinWriter) {
		emit.Syscall(w, interopnames.SystemStorageGetContext)
	})
}

func BenchmarkStorageGetReadOnlyContext(b *testing.B) {
	benchContractLoop(b, nil, func(w *io.BinWriter) {
		emit.Syscall(w, interopnames.SystemStorageGetReadOnlyContext)
	})
}

func BenchmarkStorageAsReadOnly(b *testing.B) {
	emitSetup := func(w *io.BinWriter) {
		emit.Syscall(w, interopnames.SystemStorageGetContext)
	}
	emitInteropCall := func(w *io.BinWriter) {
		emit.Opcodes(w, opcode.DUP)
		emit.Syscall(w, interopnames.SystemStorageAsReadOnly)
	}
	benchContractLoop(b, emitSetup, emitInteropCall)
}
