package neotest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestCompileFileCashedIdentifiers(t *testing.T) {
	sender := util.Uint160{}
	tmpDir := t.TempDir()

	srcPath := "../../internal/basicchain/testdata/test_contract.go"
	configPath1 := "../../internal/basicchain/testdata/test_contract.yml"
	bytesRead, err := os.ReadFile(configPath1)
	require.NoError(t, err)

	configPath2 := filepath.Join(tmpDir, "test_contract_2.yml")
	err = os.WriteFile(configPath2, bytesRead, 0755)
	require.NoError(t, err)

	contract1 := CompileFile(t, sender, srcPath, configPath1)
	contract2 := CompileFile(t, sender, srcPath, configPath2)
	require.NotEqual(t, contract1, contract2)
}

func TestAddSourceURLToNEF(t *testing.T) {
	srcPath := "../../internal/basicchain/testdata/test_contract.go"
	configPath := "../../internal/basicchain/testdata/test_contract.yml"
	ctr := CompileFile(t, util.Uint160{}, srcPath, configPath)
	require.NotEqual(t, "", ctr.NEF.Source)
}

func TestExecutorCoverageHook(t *testing.T) {
	e := &Executor{
		rawCoverage: make(map[util.Uint160]*scriptRawCoverage),
	}

	hash := util.Uint160{1, 2, 3}
	c := &Contract{
		Hash: hash,
		DebugInfo: &compiler.DebugInfo{
			Documents: []string{"contract.go"},
		},
	}

	e.addScriptToCoverage(c)
	require.Contains(t, e.rawCoverage, hash)

	e.coverageHook(hash, 42, opcode.NOP)
	require.Len(t, e.rawCoverage[hash].offsetsVisited, 1)
	require.Equal(t, 42, e.rawCoverage[hash].offsetsVisited[0])
}
