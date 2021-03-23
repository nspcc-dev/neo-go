package compiler_test

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/stretchr/testify/require"
)

const examplePath = "../../examples"
const exampleCompilePath = "testdata/compile"
const exampleSavePath = exampleCompilePath + "/save"

type compilerTestCase struct {
	name     string
	function func(*testing.T)
}

func TestCompiler(t *testing.T) {
	// CompileAndSave use config.Version for proper .nef generation.
	config.Version = "0.90.0-test"
	testCases := []compilerTestCase{
		{
			name: "TestCompileDirectory",
			function: func(t *testing.T) {
				const multiMainDir = "testdata/multi"
				_, di, err := compiler.CompileWithDebugInfo(multiMainDir, nil)
				require.NoError(t, err)
				m := map[string]bool{}
				for i := range di.Methods {
					m[di.Methods[i].ID] = true
				}
				require.Contains(t, m, "Func1")
				require.Contains(t, m, "Func2")
			},
		},
		{
			name: "TestCompile",
			function: func(t *testing.T) {
				infos, err := ioutil.ReadDir(examplePath)
				require.NoError(t, err)
				for _, info := range infos {
					if !info.IsDir() {
						// example smart contracts are located in the `examplePath` subdirectories, but
						// there are also a couple of files inside the `examplePath` which doesn't need to be compiled
						continue
					}
					infos, err := ioutil.ReadDir(path.Join(examplePath, info.Name()))
					require.NoError(t, err)
					require.False(t, len(infos) == 0, "detected smart contract folder with no contract in it")

					filename := filterFilename(infos)
					targetPath := path.Join(examplePath, info.Name(), filename)
					require.NoError(t, compileFile(targetPath))
				}
			},
		},
		{
			name: "TestCompileAndSave",
			function: func(t *testing.T) {
				infos, err := ioutil.ReadDir(exampleCompilePath)
				require.NoError(t, err)
				err = os.MkdirAll(exampleSavePath, os.ModePerm)
				require.NoError(t, err)
				t.Cleanup(func() {
					err := os.RemoveAll(exampleSavePath)
					require.NoError(t, err)
				})
				outfile := exampleSavePath + "/test.nef"
				_, err = compiler.CompileAndSave(exampleCompilePath+"/"+infos[0].Name(), &compiler.Options{Outfile: outfile})
				require.NoError(t, err)
			},
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, tcase.function)
	}
}

func filterFilename(infos []os.FileInfo) string {
	for _, info := range infos {
		if !info.IsDir() {
			return info.Name()
		}
	}
	return ""
}

func compileFile(src string) error {
	_, err := compiler.Compile(src, nil)
	return err
}

func TestOnPayableChecks(t *testing.T) {
	compileAndCheck := func(t *testing.T, src string) error {
		_, di, err := compiler.CompileWithDebugInfo("payable", strings.NewReader(src))
		require.NoError(t, err)
		_, err = compiler.CreateManifest(di, &compiler.Options{})
		return err
	}

	t.Run("NEP-11, good", func(t *testing.T) {
		src := `package payable
		import "github.com/nspcc-dev/neo-go/pkg/interop"
		func OnNEP11Payment(from interop.Hash160, amount int, tokenID []byte) {}`
		require.NoError(t, compileAndCheck(t, src))
	})
	t.Run("NEP-11, bad", func(t *testing.T) {
		src := `package payable
		import "github.com/nspcc-dev/neo-go/pkg/interop"
		func OnNEP11Payment(from interop.Hash160, amount int, oldParam string, tokenID []byte) {}`
		require.Error(t, compileAndCheck(t, src))
	})
	t.Run("NEP-17, good", func(t *testing.T) {
		src := `package payable
		import "github.com/nspcc-dev/neo-go/pkg/interop"
		func OnNEP17Payment(from interop.Hash160, amount int, data interface{}) {}`
		require.NoError(t, compileAndCheck(t, src))
	})
	t.Run("NEP-17, bad", func(t *testing.T) {
		src := `package payable
		import "github.com/nspcc-dev/neo-go/pkg/interop"
		func OnNEP17Payment(from interop.Hash160, amount int, data interface{}, extra int) {}`
		require.Error(t, compileAndCheck(t, src))
	})
}
