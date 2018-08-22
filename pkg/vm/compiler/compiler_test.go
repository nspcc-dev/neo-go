package compiler_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
)

const examplePath = "../../../examples"

func TestExamplesFolder(t *testing.T) {
	infos, err := ioutil.ReadDir(examplePath)
	if err != nil {
		t.Fatal(err)
	}

	for _, info := range infos {
		infos, err := ioutil.ReadDir(path.Join(examplePath, info.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if len(infos) == 0 {
			t.Fatal("detected smart contract folder with no contract in it")
		}

		filename := filterFilename(infos)
		targetPath := path.Join(examplePath, info.Name(), filename)
		if err := compileFile(targetPath); err != nil {
			t.Fatal(err)
		}
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
	o := compiler.Options{
		Outfile: "tmp/contract.avm",
	}

	file, err := os.Open(src)
	if err != nil {
		return err
	}
	_, err = compiler.Compile(file, &o)
	return err
}
