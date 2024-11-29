package neotest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
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
