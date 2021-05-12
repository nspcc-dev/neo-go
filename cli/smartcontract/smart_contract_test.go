package smartcontract

import (
	"flag"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func TestInitSmartContract(t *testing.T) {
	d, err := ioutil.TempDir("./", "")
	require.NoError(t, err)
	err = os.Chdir(d)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = os.Chdir("..")
		require.NoError(t, err)
		os.RemoveAll(d)
	})
	contractName := "testContract"

	set := flag.NewFlagSet("flagSet", flag.ExitOnError)
	set.String("name", contractName, "")
	ctx := cli.NewContext(cli.NewApp(), set, nil)
	require.NoError(t, initSmartContract(ctx))
	dirInfo, err := os.Stat(contractName)
	require.NoError(t, err)
	require.True(t, dirInfo.IsDir())
	files, err := ioutil.ReadDir(contractName)
	require.NoError(t, err)
	require.Equal(t, 2, len(files))
	require.Equal(t, "main.go", files[0].Name())
	require.Equal(t, "neo-go.yml", files[1].Name())
	main, err := ioutil.ReadFile(contractName + "/" + files[0].Name())
	require.NoError(t, err)
	require.Equal(t,
		`package `+contractName+`

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

var notificationName string

// init initializes notificationName before calling any other smart-contract method
func init() {
	notificationName = "Hello world!"
}

// RuntimeNotify sends runtime notification with "Hello world!" name
func RuntimeNotify(args []interface{}) {
    runtime.Notify(notificationName, args)
}`, string(main))

	manifest, err := ioutil.ReadFile(contractName + "/" + files[1].Name())
	require.NoError(t, err)
	require.Equal(t,
		`name: testContract
safemethods: []
supportedstandards: []
events:
- name: Hello world!
  parameters:
  - name: args
    type: Array
`, string(manifest))
}
