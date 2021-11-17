package compiler_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/neo"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
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

					targetPath := filepath.Join(examplePath, info.Name())
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
		func OnNEP11Payment(from interop.Hash160, amount int, tokenID []byte, data interface{}) {}`
		require.NoError(t, compileAndCheck(t, src))
	})
	t.Run("NEP-11, bad", func(t *testing.T) {
		src := `package payable
		import "github.com/nspcc-dev/neo-go/pkg/interop"
		func OnNEP11Payment(from interop.Hash160, amount int, oldParam string, tokenID []byte, data interface{}) {}`
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

func TestSafeMethodWarnings(t *testing.T) {
	src := `package payable
		func Main() int { return 1 }`

	_, di, err := compiler.CompileWithDebugInfo("eventTest", strings.NewReader(src))
	require.NoError(t, err)

	_, err = compiler.CreateManifest(di, &compiler.Options{SafeMethods: []string{"main"}})
	require.NoError(t, err)

	_, err = compiler.CreateManifest(di, &compiler.Options{SafeMethods: []string{"main", "mississippi"}})
	require.Error(t, err)
}

func TestEventWarnings(t *testing.T) {
	src := `package payable
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		func Main() { runtime.Notify("Event", 1) }`

	_, di, err := compiler.CompileWithDebugInfo("eventTest", strings.NewReader(src))
	require.NoError(t, err)

	t.Run("event it missing from config", func(t *testing.T) {
		_, err = compiler.CreateManifest(di, &compiler.Options{})
		require.Error(t, err)

		t.Run("suppress", func(t *testing.T) {
			_, err = compiler.CreateManifest(di, &compiler.Options{NoEventsCheck: true})
			require.NoError(t, err)
		})
	})
	t.Run("wrong parameter number", func(t *testing.T) {
		_, err = compiler.CreateManifest(di, &compiler.Options{
			ContractEvents: []manifest.Event{{Name: "Event"}},
		})
		require.Error(t, err)
	})
	t.Run("wrong parameter type", func(t *testing.T) {
		_, err = compiler.CreateManifest(di, &compiler.Options{
			ContractEvents: []manifest.Event{{
				Name:       "Event",
				Parameters: []manifest.Parameter{manifest.NewParameter("number", smartcontract.StringType)},
			}},
		})
		require.Error(t, err)
	})
	t.Run("good", func(t *testing.T) {
		_, err = compiler.CreateManifest(di, &compiler.Options{
			ContractEvents: []manifest.Event{{
				Name:       "Event",
				Parameters: []manifest.Parameter{manifest.NewParameter("number", smartcontract.IntegerType)},
			}},
		})
		require.NoError(t, err)
	})
	t.Run("event in imported package", func(t *testing.T) {
		t.Run("unused", func(t *testing.T) {
			src := `package foo
			import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/notify"
			func Main() int {
				return notify.Value
			}`

			_, di, err := compiler.CompileWithDebugInfo("eventTest", strings.NewReader(src))
			require.NoError(t, err)

			_, err = compiler.CreateManifest(di, &compiler.Options{NoEventsCheck: true})
			require.NoError(t, err)
		})
		t.Run("used", func(t *testing.T) {
			src := `package foo
			import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/notify"
			func Main() int {
				notify.EmitEvent()
				return 42
			}`

			_, di, err := compiler.CompileWithDebugInfo("eventTest", strings.NewReader(src))
			require.NoError(t, err)

			_, err = compiler.CreateManifest(di, &compiler.Options{})
			require.Error(t, err)

			_, err = compiler.CreateManifest(di, &compiler.Options{
				ContractEvents: []manifest.Event{{Name: "Event"}},
			})
			require.NoError(t, err)
		})
	})
}

func TestNotifyInVerify(t *testing.T) {
	srcTmpl := `package payable
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		func Verify() bool { runtime.%s("Event"); return true }`

	for _, name := range []string{"Notify", "Log"} {
		t.Run(name, func(t *testing.T) {
			src := fmt.Sprintf(srcTmpl, name)
			_, _, err := compiler.CompileWithOptions("eventTest", strings.NewReader(src),
				&compiler.Options{ContractEvents: []manifest.Event{{Name: "Event"}}})
			require.Error(t, err)

			t.Run("suppress", func(t *testing.T) {
				_, _, err := compiler.CompileWithOptions("eventTest", strings.NewReader(src),
					&compiler.Options{NoEventsCheck: true})
				require.NoError(t, err)
			})
		})
	}
}

func TestInvokedContractsPermissons(t *testing.T) {
	testCompile := func(t *testing.T, di *compiler.DebugInfo, disable bool, ps ...manifest.Permission) error {
		o := &compiler.Options{
			NoPermissionsCheck: disable,
			Permissions:        ps,
		}

		_, err := compiler.CreateManifest(di, o)
		return err
	}

	t.Run("native", func(t *testing.T) {
		src := `package test
			import "github.com/nspcc-dev/neo-go/pkg/interop/native/neo"
			import "github.com/nspcc-dev/neo-go/pkg/interop/native/management"
			func Main() int {
				neo.Transfer(nil, nil, 10, nil)
				management.GetContract(nil) // skip read-only
				return 0
			}`

		_, di, err := compiler.CompileWithOptions("permissionTest", strings.NewReader(src), &compiler.Options{})
		require.NoError(t, err)

		var nh util.Uint160

		p := manifest.NewPermission(manifest.PermissionHash, nh)
		require.Error(t, testCompile(t, di, false, *p))
		require.NoError(t, testCompile(t, di, true, *p))

		copy(nh[:], neo.Hash)
		p.Contract.Value = nh
		require.NoError(t, testCompile(t, di, false, *p))

		p.Methods.Restrict()
		require.Error(t, testCompile(t, di, false, *p))
		require.NoError(t, testCompile(t, di, true, *p))
	})

	t.Run("custom", func(t *testing.T) {
		hashStr := "aaaaaaaaaaaaaaaaaaaa"
		src := fmt.Sprintf(`package test
			import "github.com/nspcc-dev/neo-go/pkg/interop/contract"
			import "github.com/nspcc-dev/neo-go/pkg/interop"
			import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/runh"

			const hash = "%s"
			var runtimeHash interop.Hash160
			var runtimeMethod string
			func invoke(h string) interop.Hash160 { return nil }
			func Main() {
				contract.Call(interop.Hash160(hash), "method1", contract.All)
				contract.Call(interop.Hash160(hash), "method2", contract.All)
				contract.Call(interop.Hash160(hash), "method2", contract.All)

				// skip read-only
				contract.Call(interop.Hash160(hash), "method3", contract.ReadStates)

				// skip this
				contract.Call(interop.Hash160(hash), runtimeMethod, contract.All)
				contract.Call(runtimeHash, "someMethod", contract.All)
				contract.Call(interop.Hash160(runtimeHash), "someMethod", contract.All)
				contract.Call(runh.RuntimeHash(), "method4", contract.All)
			}`, hashStr)

		_, di, err := compiler.CompileWithOptions("permissionTest", strings.NewReader(src), &compiler.Options{})
		require.NoError(t, err)

		var h util.Uint160
		copy(h[:], hashStr)

		p := manifest.NewPermission(manifest.PermissionHash, h)
		require.NoError(t, testCompile(t, di, false, *p))

		p.Methods.Add("method1")
		require.Error(t, testCompile(t, di, false, *p))
		require.NoError(t, testCompile(t, di, true, *p))

		pr := manifest.NewPermission(manifest.PermissionHash, random.Uint160())
		pr.Methods.Add("someMethod")
		pr.Methods.Add("method4")

		t.Run("wildcard", func(t *testing.T) {
			pw := manifest.NewPermission(manifest.PermissionWildcard)
			require.NoError(t, testCompile(t, di, false, *p, *pw))

			pw.Methods.Add("method2")
			require.Error(t, testCompile(t, di, false, *p, *pw))
			require.NoError(t, testCompile(t, di, false, *p, *pw, *pr))
		})

		t.Run("group", func(t *testing.T) {
			priv, _ := keys.NewPrivateKey()
			pw := manifest.NewPermission(manifest.PermissionGroup, priv.PublicKey())
			require.NoError(t, testCompile(t, di, false, *p, *pw))

			pw.Methods.Add("invalid")
			require.Error(t, testCompile(t, di, false, *p, *pw, *pr))

			pw.Methods.Add("method2")
			require.Error(t, testCompile(t, di, false, *p, *pw))
			require.NoError(t, testCompile(t, di, false, *p, *pw, *pr))
		})
	})
}
