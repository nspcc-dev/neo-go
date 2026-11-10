package benchmarks

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

// --- ContractManagement.GetContract ---

func genManagementGetContractNEFSize(t *testing.T, nopNs float64) []sample {
	return sampleRange(1, getMaxScriptLength(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			nefParams: nefParams{scriptlen: curr},
		})
		return measureGetContract(t, nopNs, cs)
	})
}

func genManagementGetContractRefsDelta(t *testing.T, nopNs float64) []sample {
	return sampleRange(1, getMaxPermissionCount(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{permissioncount: curr},
		})
		return measureGetContract(t, nopNs, cs)
	})
}

func genManagementGetContractMethodNameLen(t *testing.T, nopNs float64) []sample {
	return sampleRange(1, getMaxMethodNameLen(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{permissioncount: 1, methodnamelen: curr},
		})
		return measureGetContract(t, nopNs, cs)
	})
}

func measureGetContract(t *testing.T, nopNs float64, cs *state.Contract) sample {
	const refsPerPermission = 3
	nefBytes, err := cs.NEF.Bytes()
	require.NoError(t, err)
	refsDelta := refsPerPermission * len(cs.Manifest.Permissions)
	methodNameLen := 0
	for _, p := range cs.Manifest.Permissions {
		refsDelta += len(p.Methods.Value)
		for _, name := range p.Methods.Value {
			methodNameLen += len(name)
		}
	}
	ns := measureNsPerOp(t, func(b *testing.B) {
		_, ic, md := newManagementContext(b)
		ic.VM.LoadScript(nil)
		require.NoError(b, native.PutContractState(ic.DAO, md.ID, cs))

		method, ok := md.GetMethod("getContract", 1)
		require.True(b, ok)
		benchNative(b, ic, method, stackitem.NewByteArray(cs.Hash.BytesBE()))
	})
	return sample{deps: []int{len(nefBytes), refsDelta, methodNameLen}, value: ns / nopNs}
}

// --- ContractManagement.Update ---

func genManagementTokensCount(t *testing.T, nopNs float64) []sample {
	return sampleRange(0, getMaxTokenCount(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			nefParams: nefParams{tokencount: curr},
		})
		return measureUpdate(t, nopNs, cs)
	})
}

func genManagementScriptLen(t *testing.T, nopNs float64) []sample {
	return sampleRange(1, getMaxScriptLength(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			nefParams: nefParams{scriptlen: curr},
		})
		return measureUpdate(t, nopNs, cs)
	})
}

func genManagementManifestSize(t *testing.T, nopNs float64) []sample {
	return sampleRange(1, getMaxNameLen(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{namelen: curr},
		})
		return measureUpdate(t, nopNs, cs)
	})
}

func genManagementGroupCount(t *testing.T, nopNs float64) []sample {
	return sampleRange(0, getMaxGroupCount(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{groupcount: curr},
		})
		return measureUpdate(t, nopNs, cs)
	})
}

func genManagementPermissionCount(t *testing.T, nopNs float64) []sample {
	return sampleRange(0, getMaxPermissionCount(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{permissioncount: curr},
		})
		return measureUpdate(t, nopNs, cs)
	})
}

func genManagementStandardCount(t *testing.T, nopNs float64) []sample {
	return sampleRange(0, getMaxStandardCount(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{standardcount: curr},
		})
		return measureUpdate(t, nopNs, cs)
	})
}

func genManagementTrustCount(t *testing.T, nopNs float64) []sample {
	return sampleRange(0, getMaxTrustCount(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{trustcount: curr},
		})
		return measureUpdate(t, nopNs, cs)
	})
}

func genManagementMethodCount(t *testing.T, nopNs float64) []sample {
	return sampleRange(1, getMaxMethodCount(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{methodcount: curr},
		})
		return measureUpdate(t, nopNs, cs)
	})
}

func genManagementEventCount(t *testing.T, nopNs float64) []sample {
	return sampleRange(0, getMaxEventCount(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{eventcount: curr},
		})
		return measureUpdate(t, nopNs, cs)
	})
}

func genManagementParamCount(t *testing.T, nopNs float64) []sample {
	return sampleRange(0, getMaxParamCount(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{paramcount: curr},
		})
		return measureUpdate(t, nopNs, cs)
	})
}

func measureUpdate(t *testing.T, nopNs float64, cs *state.Contract) sample {
	nefBytes, err := cs.NEF.Bytes()
	require.NoError(t, err)
	manifestBytes, err := json.Marshal(&cs.Manifest)
	require.NoError(t, err)
	paramCount := 0
	for _, m := range cs.Manifest.ABI.Methods {
		paramCount += len(m.Parameters)
	}
	ns := measureNsPerOp(t, func(b *testing.B) {
		bc, ic, md := newManagementContext(b)
		method, ok := md.GetMethod("update", 2)
		require.True(b, ok)
		ic.VM.LoadNEFMethod(&md.NEF, &md.Manifest, cs.Hash, md.Hash, callflag.All,
			false, method.MD.Offset, -1, nil, nil, false)
		old := newContract(b, contractParams{})
		old.UpdateCounter = 0
		old.Hash = cs.Hash
		old.Manifest.Name = cs.Manifest.Name
		require.NoError(b, native.PutContractState(ic.DAO, md.ID, old))
		args := []stackitem.Item{
			stackitem.NewByteArray(nefBytes),
			stackitem.NewByteArray(manifestBytes),
		}
		popArgsPushRes := func(stackitem.Item) {}
		base := ic.DAO
		for b.Loop() {
			b.StopTimer()
			ic.DAO = base.GetPrivate()
			ic.DAO.GetRWCache(md.ID)
			ic.DAO.GetRWCache(bc.NativePolicyID())
			ic.Notifications = nil
			b.StartTimer()

			method.DeferrableFunc(ic, args, popArgsPushRes)
		}
	})
	return sample{deps: []int{
		len(cs.NEF.Tokens), len(cs.NEF.Script), len(manifestBytes),
		len(cs.Manifest.Groups), len(cs.Manifest.Permissions), len(cs.Manifest.SupportedStandards),
		len(cs.Manifest.Trusts.Value), len(cs.Manifest.ABI.Methods), len(cs.Manifest.ABI.Events), paramCount,
	}, value: ns / nopNs}
}

// --- ContractManagement.GetMinimumDeploymentFee ---

func genManagementGetMinimumDeploymentFee(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		_, ic, md := newManagementContext(b)
		ic.VM.LoadScript(nil)
		method, ok := md.GetMethod("getMinimumDeploymentFee", 0)
		require.True(b, ok)
		benchNative(b, ic, method)
	})
	return ns / nopNs
}

// --- ContractManagement.SetMinimumDeploymentFee ---

func genManagementSetMinimumDeploymentFee(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		_, ic, md := newManagementContext(b)
		ic.VM.LoadScript(nil)
		for _, c := range ic.Natives {
			if neo, ok := c.(*native.NEO); ok {
				ic.Tx.Signers = []transaction.Signer{{
					Account: neo.GetCommitteeAddress(ic.DAO),
					Scopes:  transaction.Global,
				}}
				break
			}
		}
		method, ok := md.GetMethod("setMinimumDeploymentFee", 1)
		require.True(b, ok)
		benchNative(b, ic, method, stackitem.Make(1_00000000))
	})
	return ns / nopNs
}

// --- ContractManagement.HasMethod ---

func genManagementHasMethodCount(t *testing.T, nopNs float64) []sample {
	return sampleRange(1, getMaxMethodCount(t), func(curr int) sample {
		cs := newContract(t, contractParams{
			manifestParams: manifestParams{methodcount: curr},
		})
		name := cs.Manifest.ABI.Methods[len(cs.Manifest.ABI.Methods)-1].Name
		return measureHasMethod(t, nopNs, cs, name)
	})
}

func genManagementHasMethodNameLen(t *testing.T, nopNs float64) []sample {
	cs := newContract(t, contractParams{})
	return sampleRange(1, stackitem.MaxSize, func(curr int) sample {
		return measureHasMethod(t, nopNs, cs, strings.Repeat("m", curr))
	})
}

func measureHasMethod(t *testing.T, nopNs float64, cs *state.Contract, name string) sample {
	ns := measureNsPerOp(t, func(b *testing.B) {
		_, ic, md := newManagementContext(b)
		ic.VM.LoadScript(nil)
		require.NoError(b, native.PutContractState(ic.DAO, md.ID, cs))
		method, ok := md.GetMethod("hasMethod", 3)
		require.True(b, ok)
		args := []stackitem.Item{
			stackitem.NewByteArray(cs.Hash.BytesBE()),
			stackitem.Make(name),
			stackitem.Make(0),
		}
		benchNative(b, ic, method, args...)
	})
	return sample{deps: []int{len(cs.Manifest.ABI.Methods), len(name)}, value: ns / nopNs}
}

// --- ContractManagement.IsContract ---

func genManagementIsContract(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		_, ic, md := newManagementContext(b)
		ic.VM.LoadScript(nil)
		cs := newContract(b, contractParams{})
		require.NoError(b, native.PutContractState(ic.DAO, md.ID, cs))
		method, ok := md.GetMethod("isContract", 1)
		require.True(b, ok)
		benchNative(b, ic, method, stackitem.NewByteArray(cs.Hash.BytesBE()))
	})
	return ns / nopNs
}

// --- ContractManagement.Destroy ---

// The baseline uses empty contract storage; storage entry count is not modeled.
func genManagementDestroy(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		bc, ic, md := newManagementContext(b)
		method, ok := md.GetMethod("destroy", 0)
		require.True(b, ok)
		cs := newContract(b, contractParams{})
		cs.UpdateCounter = 0
		ic.VM.LoadNEFMethod(&md.NEF, &md.Manifest, cs.Hash, md.Hash, callflag.All,
			false, method.MD.Offset, -1, nil, nil, false)
		require.NoError(b, native.PutContractState(ic.DAO, md.ID, cs))
		popArgsPushRes := func(stackitem.Item) {}
		base := ic.DAO
		for b.Loop() {
			b.StopTimer()
			ic.DAO = base.GetPrivate()
			ic.DAO.GetRWCache(md.ID)
			ic.DAO.GetRWCache(bc.NativePolicyID())
			ic.Notifications = nil
			b.StartTimer()

			method.DeferrableFunc(ic, nil, popArgsPushRes)
		}
	})
	return ns / nopNs
}
