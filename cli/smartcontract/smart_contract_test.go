package smartcontract

import (
	"encoding/hex"
	"flag"
	"io/ioutil"
	"os"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

func TestInitSmartContract(t *testing.T) {
	d := t.TempDir()
	testWD, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(d)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(testWD)) })
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
permissions:
- methods: '*'
`, string(manifest))
}

func testPermissionMarshal(t *testing.T, p *manifest.Permission, expected string) {
	out, err := yaml.Marshal((*permission)(p))
	require.NoError(t, err)
	require.Equal(t, expected, string(out))

	t.Run("Unmarshal", func(t *testing.T) {
		actual := new(permission)
		require.NoError(t, yaml.Unmarshal(out, actual))
		require.Equal(t, p, (*manifest.Permission)(actual))
	})
}

func TestPermissionMarshal(t *testing.T) {
	t.Run("Wildcard", func(t *testing.T) {
		p := manifest.NewPermission(manifest.PermissionWildcard)
		testPermissionMarshal(t, p, "methods: '*'\n")
	})
	t.Run("no allowed methods", func(t *testing.T) {
		p := manifest.NewPermission(manifest.PermissionWildcard)
		p.Methods.Restrict()
		testPermissionMarshal(t, p, "methods: []\n")
	})
	t.Run("hash", func(t *testing.T) {
		h := random.Uint160()
		p := manifest.NewPermission(manifest.PermissionHash, h)
		testPermissionMarshal(t, p,
			"hash: "+h.StringLE()+"\n"+
				"methods: '*'\n")
	})
	t.Run("group with some methods", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)

		p := manifest.NewPermission(manifest.PermissionGroup, priv.PublicKey())
		p.Methods.Add("abc")
		p.Methods.Add("lamao")
		testPermissionMarshal(t, p,
			"group: "+hex.EncodeToString(priv.PublicKey().Bytes())+"\n"+
				"methods:\n- abc\n- lamao\n")
	})
}

func TestPermissionUnmarshalInvalid(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	pub := hex.EncodeToString(priv.PublicKey().Bytes())
	u160 := random.Uint160().StringLE()
	testCases := []string{
		"hash: []\nmethods: '*'\n",                             // invalid hash type
		"hash: notahex\nmethods: '*'\n",                        // invalid hash
		"group: []\nmethods: '*'\n",                            // invalid group type
		"group: notahex\nmethods: '*'\n",                       // invalid group
		"hash: " + u160 + "\n",                                 // missing methods
		"group: " + pub + "\nhash: " + u160 + "\nmethods: '*'", // hash/group conflict
		"hash: " + u160 + "\nmethods:\n  a: b\n",               // invalid methods type
		"hash: " + u160 + "\nmethods:\n- []\n",                 // methods array, invalid single
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			require.Error(t, yaml.Unmarshal([]byte(tc), new(permission)))
		})
	}
}
