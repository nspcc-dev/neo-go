package native_test

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestManagement_GetNEP17Contracts(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		bc, validators, committee := chain.NewMulti(t)
		e := neotest.NewExecutor(t, bc, validators, committee)

		require.ElementsMatch(t, []util.Uint160{e.NativeHash(t, nativenames.Neo),
			e.NativeHash(t, nativenames.Gas)}, bc.GetNEP17Contracts())
	})

	t.Run("basic chain", func(t *testing.T) {
		bc, validators, committee := chain.NewMultiWithCustomConfig(t, func(c *config.Blockchain) {
			c.P2PSigExtensions = true // `basicchain.Init` requires Notary enabled
		})
		e := neotest.NewExecutor(t, bc, validators, committee)
		basicchain.Init(t, "../../../", e)

		require.ElementsMatch(t, []util.Uint160{e.NativeHash(t, nativenames.Neo),
			e.NativeHash(t, nativenames.Gas), e.ContractHash(t, 1)}, bc.GetNEP17Contracts())
	})
}

func TestManagement_DeployUpdate_HFBasilisk(t *testing.T) {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
		c.Hardforks = map[string]uint32{
			config.HFBasilisk.String(): 2,
		}
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	ne, err := nef.NewFile([]byte{byte(opcode.JMP), 0x05})
	require.NoError(t, err)

	m := &manifest.Manifest{
		Name:     "ctr",
		Features: json.RawMessage("{}"),
		Groups:   []manifest.Group{},
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name:   "main",
					Offset: 0,
				},
			},
		},
	}
	ctr := &neotest.Contract{

		Hash:     state.CreateContractHash(e.Validator.ScriptHash(), ne.Checksum, m.Name),
		NEF:      ne,
		Manifest: m,
	}

	// Block 1: no script check on deploy.
	e.DeployContract(t, ctr, nil)
	e.AddNewBlock(t)

	// Block 3: script check on deploy.
	ctr.Manifest.Name = "other name"
	e.DeployContractCheckFAULT(t, ctr, nil, "invalid contract script: invalid offset 5 ip at 0")
}

func TestManagement_CallInTheSameBlock(t *testing.T) {
	bc, acc := chain.NewSingle(t)

	e := neotest.NewExecutor(t, bc, acc, acc)

	ne, err := nef.NewFile([]byte{byte(opcode.PUSH1)})
	require.NoError(t, err)

	m := &manifest.Manifest{
		Name:     "ctr",
		Features: json.RawMessage("{}"),
		Groups:   []manifest.Group{},
		ABI: manifest.ABI{
			Methods: []manifest.Method{
				{
					Name:   "main",
					Offset: 0,
				},
			},
		},
	}

	t.Run("same tx", func(t *testing.T) {
		h := state.CreateContractHash(e.Validator.ScriptHash(), ne.Checksum, m.Name)

		neb, err := ne.Bytes()
		require.NoError(t, err)

		rawManifest, err := json.Marshal(m)
		require.NoError(t, err)

		b := smartcontract.NewBuilder()
		b.InvokeWithAssert(bc.ManagementContractHash(), "deploy", neb, rawManifest) // We need to drop the resulting contract and ASSERT does that.
		b.InvokeWithAssert(bc.ManagementContractHash(), "hasMethod", h, "main", 0)
		b.InvokeMethod(h, "main")

		script, err := b.Script()
		require.NoError(t, err)
		txHash := e.InvokeScript(t, script, []neotest.Signer{e.Validator})
		e.CheckHalt(t, txHash, stackitem.Make(1))
	})
	t.Run("next tx", func(t *testing.T) {
		m.Name = "another contract"
		h := state.CreateContractHash(e.Validator.ScriptHash(), ne.Checksum, m.Name)

		txDeploy := e.NewDeployTx(t, e.Chain, &neotest.Contract{Hash: h, NEF: ne, Manifest: m}, nil)
		txHasMethod := e.NewTx(t, []neotest.Signer{e.Validator}, bc.ManagementContractHash(), "hasMethod", h, "main", 0)

		txCall := e.NewUnsignedTx(t, h, "main") // Test invocation doesn't give true GAS cost before deployment.
		txCall = e.SignTx(t, txCall, 1_0000_0000, e.Validator)

		e.AddNewBlock(t, txDeploy, txHasMethod, txCall)
		e.CheckHalt(t, txHasMethod.Hash(), stackitem.Make(true))
		e.CheckHalt(t, txCall.Hash(), stackitem.Make(1))
	})
}
