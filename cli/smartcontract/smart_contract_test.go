package smartcontract

import (
	"flag"
	"io/ioutil"
	"os"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func TestInitSmartContract(t *testing.T) {
	d, err := ioutil.TempDir("./", "")
	require.NoError(t, err)
	os.Chdir(d)
	defer func() {
		os.Chdir("..")
		os.RemoveAll(d)
	}()
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
		`hasstorage: false
ispayable: false
supportedstandards: []
events:
- name: Hello world!
  parameters:
  - name: args
    type: Array
`, string(manifest))
}

func TestGetFeatures(t *testing.T) {
	cfg := ProjectConfig{
		IsPayable:  true,
		HasStorage: true,
	}
	f := cfg.GetFeatures()
	require.Equal(t, smartcontract.IsPayable|smartcontract.HasStorage, f)
}

func TestParseCosigner(t *testing.T) {
	acc := util.Uint160{1, 3, 5, 7}
	testCases := map[string]transaction.Signer{
		acc.StringLE(): {
			Account: acc,
			Scopes:  transaction.Global,
		},
		"0x" + acc.StringLE(): {
			Account: acc,
			Scopes:  transaction.Global,
		},
		acc.StringLE() + ":Global": {
			Account: acc,
			Scopes:  transaction.Global,
		},
		acc.StringLE() + ":CalledByEntry": {
			Account: acc,
			Scopes:  transaction.CalledByEntry,
		},
		acc.StringLE() + ":FeeOnly": {
			Account: acc,
			Scopes:  transaction.FeeOnly,
		},
		acc.StringLE() + ":CalledByEntry,CustomContracts": {
			Account: acc,
			Scopes:  transaction.CalledByEntry | transaction.CustomContracts,
		},
	}
	for s, expected := range testCases {
		actual, err := parseCosigner(s)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	errorCases := []string{
		acc.StringLE() + "0",
		acc.StringLE() + ":Unknown",
		acc.StringLE() + ":Global,CustomContracts",
		acc.StringLE() + ":Global,FeeOnly",
	}
	for _, s := range errorCases {
		_, err := parseCosigner(s)
		require.Error(t, err)
	}
}
