package compiler_test

import (
	"io/ioutil"
	"os"
	"path"
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
				defer func() {
					err := os.RemoveAll(exampleSavePath)
					require.NoError(t, err)
				}()
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
