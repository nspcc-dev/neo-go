package benchmarks

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// countPoints is the number of intervals in a parameter sweep.
const countPoints = 8

// sample is a single measurement: dependency values
// and the resulting normalized time.
type sample struct {
	deps  []int
	value float64
}

type nefParams struct {
	tokencount int
	scriptlen  int
}

type manifestParams struct {
	namelen         int
	methodnamelen   int
	groupcount      int
	permissioncount int
	standardcount   int
	trustcount      int
	methodcount     int
	eventcount      int
	paramcount      int
}

type contractParams struct {
	nefParams
	manifestParams
}

func measureNsPerOp(t *testing.T, f func(b *testing.B)) float64 {
	r := testing.Benchmark(f)
	require.NotZero(t, r.N, "benchmark failed")
	return float64(r.NsPerOp())
}

func benchNative(b *testing.B, ic *interop.Context, method interop.HFSpecificMethodAndPrice, args ...stackitem.Item) {
	estack := ic.VM.Estack()
	hasReturn := method.MD.ReturnType != smartcontract.VoidType
	for b.Loop() {
		result := method.Func(ic, args)
		if hasReturn {
			estack.PushItem(result)
		}

		b.StopTimer()
		if hasReturn {
			estack.Pop()
		}
		b.StartTimer()
	}
}

func sampleRange(start, maxValue int, measure func(int) sample) []sample {
	step := (maxValue - start) / countPoints
	curr := start
	rows := make([]sample, 0, countPoints+1)
	for range countPoints + 1 {
		rows = append(rows, measure(curr))
		curr += step
	}
	return rows
}

// newPriceChain enables Huyao for both the NOP baseline and native measurements.
func newPriceChain(t testing.TB) *core.Blockchain {
	bc, _ := chain.NewSingleWithOptions(t, &chain.Options{
		BlockchainConfigHook: func(c *config.Blockchain) {
			c.Hardforks = map[string]uint32{config.HFHuyao.String(): 0}
		},
		Logger: zap.NewNop(),
	})
	return bc
}

func newPriceContext(t testing.TB, bc *core.Blockchain, tx *transaction.Transaction) *interop.Context {
	if tx == nil {
		tx = &transaction.Transaction{}
	}
	ic, err := bc.GetTestVM(trigger.Application, tx, nil)
	require.NoError(t, err)
	t.Cleanup(ic.Finalize)
	return ic
}

func newNativeContext(t testing.TB, name string) (*core.Blockchain, *interop.Context, *interop.HFSpecificContractMD) {
	bc := newPriceChain(t)
	ic := newPriceContext(t, bc, nil)
	var md *interop.ContractMD
	for _, c := range ic.Natives {
		if c.Metadata().Name == name {
			md = c.Metadata()
			break
		}
	}
	require.NotNil(t, md, "native contract %s not found", name)
	hf := config.HFHuyao
	ic.VM.SetGasLimit(-1)
	return bc, ic, md.HFSpecificContractMD(&hf)
}

func newManifest(t testing.TB, p manifestParams, hash util.Uint160) *manifest.Manifest {
	m := manifest.NewManifest(strings.Repeat("m", max(p.namelen, 1)))
	m.ABI.Methods = make([]manifest.Method, max(p.methodcount, 1))
	for i := range m.ABI.Methods {
		m.ABI.Methods[i] = manifest.Method{
			Name:       fmt.Sprintf("m%04d", i),
			Parameters: []manifest.Parameter{},
			ReturnType: smartcontract.VoidType,
		}
	}
	m.ABI.Methods[0].Parameters = make([]manifest.Parameter, p.paramcount)
	for i := range m.ABI.Methods[0].Parameters {
		m.ABI.Methods[0].Parameters[i] = manifest.NewParameter(fmt.Sprintf("p%04d", i), smartcontract.AnyType)
	}
	m.ABI.Events = make([]manifest.Event, p.eventcount)
	for i := range m.ABI.Events {
		m.ABI.Events[i] = manifest.Event{Name: fmt.Sprintf("e%04d", i), Parameters: []manifest.Parameter{}}
	}
	m.Groups = make([]manifest.Group, p.groupcount)
	for i := range m.Groups {
		var privateKey [32]byte
		binary.BigEndian.PutUint32(privateKey[28:], uint32(i+1))
		key, err := keys.NewPrivateKeyFromBytes(privateKey[:])
		require.NoError(t, err)
		m.Groups[i] = manifest.Group{PublicKey: key.PublicKey(), Signature: key.Sign(hash.BytesBE())}
	}
	m.Permissions = make([]manifest.Permission, p.permissioncount)
	for i := range m.Permissions {
		var hash util.Uint160
		binary.LittleEndian.PutUint32(hash[:], uint32(i+1))
		permission := manifest.NewPermission(manifest.PermissionHash, hash)
		permission.Methods.Restrict()
		if p.methodnamelen > 0 {
			permission.Methods.Add(strings.Repeat("m", p.methodnamelen))
		}
		m.Permissions[i] = *permission
	}
	m.SupportedStandards = make([]string, p.standardcount)
	for i := range m.SupportedStandards {
		m.SupportedStandards[i] = fmt.Sprintf("s%04d", i)
	}
	m.Trusts.Value = make([]manifest.PermissionDesc, p.trustcount)
	for i := range m.Trusts.Value {
		var hash util.Uint160
		binary.LittleEndian.PutUint32(hash[:], uint32(i+1))
		m.Trusts.Value[i] = manifest.PermissionDesc{Type: manifest.PermissionHash, Value: hash}
	}
	return m
}

func newContract(t testing.TB, p contractParams) *state.Contract {
	nf, err := nef.NewFile([]byte{byte(opcode.RET)})
	require.NoError(t, err)
	deploymentChecksum := nf.Checksum
	nf.Script = bytes.Repeat([]byte{byte(opcode.RET)}, max(p.scriptlen, 1))
	nf.Tokens = make([]nef.MethodToken, p.tokencount)
	for i := range nf.Tokens {
		var hash util.Uint160
		binary.LittleEndian.PutUint32(hash[:], uint32(i+1))
		nf.Tokens[i] = nef.MethodToken{Hash: hash, Method: strings.Repeat("m", 32), CallFlag: callflag.All}
	}
	nf.Checksum = nf.CalculateChecksum()
	name := strings.Repeat("m", max(p.namelen, 1))
	hash := state.CreateContractHash(util.Uint160{1}, deploymentChecksum, name)
	return &state.Contract{
		ContractBase: state.ContractBase{
			ID:       1,
			Hash:     hash,
			NEF:      *nf,
			Manifest: *newManifest(t, p.manifestParams, hash),
		},
		UpdateCounter: 1,
	}
}

func validateContract(cs *state.Contract) error {
	rawManifest, err := json.Marshal(&cs.Manifest)
	if err != nil {
		return err
	}
	if len(rawManifest) > manifest.MaxManifestSize {
		return fmt.Errorf("manifest size %d exceeds %d", len(rawManifest), manifest.MaxManifestSize)
	}
	result, err := cs.ToStackItem()
	if err != nil {
		return err
	}
	_, err = stackitem.SerializeLimited(result, vm.MaxStackSize)
	return err
}

func getMaxMethodNameLen(t testing.TB) int {
	return sort.Search(manifest.MaxManifestSize, func(n int) bool {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{permissioncount: 1, methodnamelen: n + 1},
		})
		return validateContract(cs) != nil
	})
}

func getMaxTokenCount(t testing.TB) int {
	return sort.Search(stackitem.MaxSize, func(n int) bool {
		cs := newContract(t, contractParams{
			nefParams: nefParams{tokencount: n + 1},
		})
		return validateContract(cs) != nil
	})
}

func getMaxScriptLength(t testing.TB) int {
	return sort.Search(stackitem.MaxSize, func(n int) bool {
		cs := newContract(t, contractParams{
			nefParams: nefParams{scriptlen: n + 1},
		})
		return validateContract(cs) != nil
	})
}

func getMaxNameLen(t testing.TB) int {
	return sort.Search(manifest.MaxManifestSize, func(n int) bool {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{namelen: n + 1},
		})
		return validateContract(cs) != nil
	})
}

func getMaxGroupCount(t testing.TB) int {
	return sort.Search(vm.MaxStackSize, func(n int) bool {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{groupcount: n + 1},
		})
		return validateContract(cs) != nil
	})
}

func getMaxPermissionCount(t testing.TB) int {
	return sort.Search(vm.MaxStackSize, func(n int) bool {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{permissioncount: n + 1},
		})
		return validateContract(cs) != nil
	})
}

func getMaxStandardCount(t testing.TB) int {
	return sort.Search(vm.MaxStackSize, func(n int) bool {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{standardcount: n + 1},
		})
		return validateContract(cs) != nil
	})
}

func getMaxTrustCount(t testing.TB) int {
	return sort.Search(vm.MaxStackSize, func(n int) bool {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{trustcount: n + 1},
		})
		return validateContract(cs) != nil
	})
}

func getMaxMethodCount(t testing.TB) int {
	return sort.Search(vm.MaxStackSize, func(n int) bool {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{methodcount: n + 1},
		})
		return validateContract(cs) != nil
	})
}

func getMaxEventCount(t testing.TB) int {
	return sort.Search(vm.MaxStackSize, func(n int) bool {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{eventcount: n + 1},
		})
		return validateContract(cs) != nil
	})
}

func getMaxParamCount(t testing.TB) int {
	return sort.Search(vm.MaxStackSize, func(n int) bool {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{paramcount: n + 1},
		})
		return validateContract(cs) != nil
	})
}
