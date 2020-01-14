package compiler_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/compiler"
	"github.com/stretchr/testify/require"
)

const examplePath = "../../examples"
const exampleCompilePath = "testdata/compile"
const exampleSavePath = exampleCompilePath + "/save"

type compilerTestCase struct {
	name     string
	function func()
}

func TestCompiler(t *testing.T) {
	testCases := []compilerTestCase{
		{
			name: "TestCompile",
			function: func() {
				infos, err := ioutil.ReadDir(examplePath)
				require.NoError(t, err)
				for _, info := range infos {
					infos, err := ioutil.ReadDir(path.Join(examplePath, info.Name()))
					require.NoError(t, err)
					if len(infos) == 0 {
						t.Fatal("detected smart contract folder with no contract in it")
					}

					filename := filterFilename(infos)
					targetPath := path.Join(examplePath, info.Name(), filename)
					if err := compileFile(targetPath); err != nil {
						t.Fatal(err)
					}
				}
			},
		},
		{
			name: "TestCompileAndSave",
			function: func() {
				infos, err := ioutil.ReadDir(exampleCompilePath)
				require.NoError(t, err)
				err = os.MkdirAll(exampleSavePath, os.ModePerm)
				require.NoError(t, err)
				outfile := exampleSavePath + "/test.avm"
				if _, err := compiler.CompileAndSave(exampleCompilePath+"/"+infos[0].Name(), &compiler.Options{Outfile: outfile}); err != nil {
					t.Fatal(err)
				}
				defer func() {
					err := os.RemoveAll(exampleSavePath)
					require.NoError(t, err)
				}()
			},
		},
	}

	for _, tcase := range testCases {
		t.Run(tcase.name, func(t *testing.T) {
			tcase.function()
		})
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
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	_, err = compiler.Compile(file)
	return err
}
