package main

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestCalcHash(t *testing.T) {
	e := newExecutor(t, false)
	defer e.Close(t)

	nefPath := "./testdata/verify.nef"
	src, err := ioutil.ReadFile(nefPath)
	require.NoError(t, err)
	nefF, err := nef.FileFromBytes(src)
	require.NoError(t, err)
	manifestPath := "./testdata/verify.manifest.json"
	manifestBytes, err := ioutil.ReadFile(manifestPath)
	require.NoError(t, err)
	manif := &manifest.Manifest{}
	err = json.Unmarshal(manifestBytes, manif)
	require.NoError(t, err)
	sender := random.Uint160()

	cmd := []string{"neo-go", "contract", "calc-hash"}
	t.Run("no sender", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--in", nefPath, "--manifest", manifestPath)...)
	})
	t.Run("no nef file", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--sender", sender.StringLE(), "--manifest", manifestPath)...)
	})
	t.Run("no manifest file", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--sender", sender.StringLE(), "--in", nefPath)...)
	})
	t.Run("invalid path", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--sender", sender.StringLE(),
			"--in", "./testdata/verify.nef123", "--manifest", manifestPath)...)
	})
	t.Run("invalid file", func(t *testing.T) {
		p := path.Join(os.TempDir(), "neogo.calchash.verify.nef")
		defer os.Remove(p)
		require.NoError(t, ioutil.WriteFile(p, src[:4], os.ModePerm))
		e.RunWithError(t, append(cmd, "--sender", sender.StringLE(), "--in", p, "--manifest", manifestPath)...)
	})

	cmd = append(cmd, "--in", nefPath, "--manifest", manifestPath)
	expected := state.CreateContractHash(sender, nefF.Checksum, manif.Name)
	t.Run("valid, uint160", func(t *testing.T) {
		e.Run(t, append(cmd, "--sender", sender.StringLE())...)
		e.checkNextLine(t, expected.StringLE())
	})
	t.Run("valid, uint160 with 0x", func(t *testing.T) {
		e.Run(t, append(cmd, "--sender", "0x"+sender.StringLE())...)
		e.checkNextLine(t, expected.StringLE())
	})
	t.Run("valid, address", func(t *testing.T) {
		e.Run(t, append(cmd, "--sender", address.Uint160ToString(sender))...)
		e.checkNextLine(t, expected.StringLE())
	})
}

func TestContractInitAndCompile(t *testing.T) {
	tmpDir := path.Join(os.TempDir(), "neogo.inittest")
	require.NoError(t, os.Mkdir(tmpDir, os.ModePerm))
	defer os.RemoveAll(tmpDir)

	e := newExecutor(t, false)
	defer e.Close(t)

	t.Run("no path is provided", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "init")
	})
	t.Run("invalid path", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "init", "--name", "\x00")
	})

	ctrPath := path.Join(tmpDir, "testcontract")
	e.Run(t, "neo-go", "contract", "init", "--name", ctrPath)

	t.Run("don't rewrite existing directory", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "init", "--name", ctrPath)
	})

	// For proper nef generation.
	config.Version = "0.90.0-test"

	srcPath := path.Join(ctrPath, "main.go")
	cfgPath := path.Join(ctrPath, "neo-go.yml")
	nefPath := path.Join(tmpDir, "testcontract.nef")
	manifestPath := path.Join(tmpDir, "testcontract.manifest.json")
	cmd := []string{"neo-go", "contract", "compile"}
	t.Run("missing source", func(t *testing.T) {
		e.RunWithError(t, cmd...)
	})

	cmd = append(cmd, "--in", srcPath, "--out", nefPath, "--manifest", manifestPath)
	t.Run("missing config, but require manifest", func(t *testing.T) {
		e.RunWithError(t, cmd...)
	})
	t.Run("provided non-existent config", func(t *testing.T) {
		cfgName := path.Join(ctrPath, "notexists.yml")
		e.RunWithError(t, append(cmd, "--config", cfgName)...)
	})

	cmd = append(cmd, "--config", cfgPath)
	e.Run(t, cmd...)
	e.checkEOF(t)
	require.FileExists(t, nefPath)
	require.FileExists(t, manifestPath)

	t.Run("output hex script with --verbose", func(t *testing.T) {
		e.Run(t, append(cmd, "--verbose")...)
		e.checkNextLine(t, "^[0-9a-hA-H]+$")
	})
}

// Checks that error is returned if GAS available for test-invoke exceeds
// GAS needed to be consumed.
func TestDeployBigContract(t *testing.T) {
	e := newExecutorWithConfig(t, true, func(c *config.Config) {
		c.ApplicationConfiguration.RPC.MaxGasInvoke = fixedn.Fixed8(1)
	})
	defer e.Close(t)

	// For proper nef generation.
	config.Version = "0.90.0-test"

	tmpDir := path.Join(os.TempDir(), "neogo.test.deployfail")
	require.NoError(t, os.Mkdir(tmpDir, os.ModePerm))
	defer os.RemoveAll(tmpDir)

	nefName := path.Join(tmpDir, "deploy.nef")
	manifestName := path.Join(tmpDir, "deploy.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", "testdata/deploy/main.go", // compile single file
		"--config", "testdata/deploy/neo-go.yml",
		"--out", nefName, "--manifest", manifestName)

	e.In.WriteString("one\r")
	e.RunWithError(t, "neo-go", "contract", "deploy",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet, "--address", validatorAddr,
		"--in", nefName, "--manifest", manifestName)
}

func TestComlileAndInvokeFunction(t *testing.T) {
	e := newExecutor(t, true)
	defer e.Close(t)

	// For proper nef generation.
	config.Version = "0.90.0-test"

	tmpDir := path.Join(os.TempDir(), "neogo.test.compileandinvoke")
	require.NoError(t, os.Mkdir(tmpDir, os.ModePerm))
	defer os.RemoveAll(tmpDir)

	nefName := path.Join(tmpDir, "deploy.nef")
	manifestName := path.Join(tmpDir, "deploy.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", "testdata/deploy/main.go", // compile single file
		"--config", "testdata/deploy/neo-go.yml",
		"--out", nefName, "--manifest", manifestName)

	// Check that it is possible to invoke before deploy.
	// This doesn't make much sense, because every method has an offset
	// which is contained in the manifest. This should be either removed or refactored.
	e.Run(t, "neo-go", "contract", "testinvokescript",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--in", nefName, "--", util.Uint160{1, 2, 3}.StringLE())
	e.Run(t, "neo-go", "contract", "testinvokescript",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--in", nefName, "--", address.Uint160ToString(util.Uint160{1, 2, 3}))

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "contract", "deploy",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet, "--address", validatorAddr,
		"--in", nefName, "--manifest", manifestName)

	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	line = strings.TrimSpace(strings.TrimPrefix(line, "Contract: "))
	h, err := util.Uint160DecodeStringLE(line)
	require.NoError(t, err)
	e.checkTxPersisted(t)

	t.Run("check calc hash", func(t *testing.T) {
		e.Run(t, "neo-go", "contract", "calc-hash",
			"--sender", validatorAddr, "--in", nefName,
			"--manifest", manifestName)
		e.checkNextLine(t, h.StringLE())
	})

	cmd := []string{"neo-go", "contract", "testinvokefunction",
		"--rpc-endpoint", "http://" + e.RPC.Addr}
	t.Run("missing hash", func(t *testing.T) {
		e.RunWithError(t, cmd...)
	})
	t.Run("invalid hash", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "notahash")...)
	})

	cmd = append(cmd, h.StringLE())
	t.Run("missing method", func(t *testing.T) {
		e.RunWithError(t, cmd...)
	})

	cmd = append(cmd, "getValue")
	t.Run("invalid params", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "[")...)
	})
	t.Run("invalid cosigner", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--", "notahash")...)
	})
	t.Run("missing RPC address", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "testinvokefunction",
			h.StringLE(), "getValue")
	})

	e.Run(t, cmd...)

	res := new(result.Invoke)
	require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
	require.Equal(t, vm.HaltState.String(), res.State, res.FaultException)
	require.Len(t, res.Stack, 1)
	require.Equal(t, []byte("on create|sub create"), res.Stack[0].Value())

	t.Run("real invoke", func(t *testing.T) {
		cmd := []string{"neo-go", "contract", "invokefunction",
			"--rpc-endpoint", "http://" + e.RPC.Addr}
		t.Run("missing wallet", func(t *testing.T) {
			cmd := append(cmd, h.StringLE(), "getValue")
			e.RunWithError(t, cmd...)
		})
		t.Run("non-existent wallet", func(t *testing.T) {
			cmd := append(cmd, "--wallet", path.Join(tmpDir, "not.exists"),
				h.StringLE(), "getValue")
			e.RunWithError(t, cmd...)
		})

		cmd = append(cmd, "--wallet", validatorWallet, "--address", validatorAddr)
		e.In.WriteString("one\r")
		e.Run(t, append(cmd, h.StringLE(), "getValue")...)

		t.Run("failind method", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.RunWithError(t, append(cmd, h.StringLE(), "fail")...)

			e.In.WriteString("one\r")
			e.Run(t, append(cmd, "--force", h.StringLE(), "fail")...)
		})
	})

	t.Run("test Storage.Find", func(t *testing.T) {
		cmd := []string{"neo-go", "contract", "testinvokefunction",
			"--rpc-endpoint", "http://" + e.RPC.Addr,
			h.StringLE(), "testFind"}

		t.Run("keys only", func(t *testing.T) {
			e.Run(t, append(cmd, strconv.FormatInt(storage.FindKeysOnly, 10))...)
			res := new(result.Invoke)
			require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
			require.Equal(t, vm.HaltState.String(), res.State)
			require.Len(t, res.Stack, 1)
			require.Equal(t, []stackitem.Item{
				stackitem.Make("findkey1"),
				stackitem.Make("findkey2"),
			}, res.Stack[0].Value())
		})
		t.Run("both", func(t *testing.T) {
			e.Run(t, append(cmd, strconv.FormatInt(storage.FindDefault, 10))...)
			res := new(result.Invoke)
			require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
			require.Equal(t, vm.HaltState.String(), res.State)
			require.Len(t, res.Stack, 1)

			arr, ok := res.Stack[0].Value().([]stackitem.Item)
			require.True(t, ok)
			require.Len(t, arr, 2)
			require.Equal(t, []stackitem.Item{
				stackitem.Make("findkey1"), stackitem.Make("value1"),
			}, arr[0].Value())
			require.Equal(t, []stackitem.Item{
				stackitem.Make("findkey2"), stackitem.Make("value2"),
			}, arr[1].Value())
		})
	})

	t.Run("Update", func(t *testing.T) {
		nefName := path.Join(tmpDir, "updated.nef")
		manifestName := path.Join(tmpDir, "updated.manifest.json")
		e.Run(t, "neo-go", "contract", "compile",
			"--config", "testdata/deploy/neo-go.yml",
			"--in", "testdata/deploy/", // compile all files in dir
			"--out", nefName, "--manifest", manifestName)

		defer func() {
			os.Remove(nefName)
			os.Remove(manifestName)
		}()

		rawNef, err := ioutil.ReadFile(nefName)
		require.NoError(t, err)
		rawManifest, err := ioutil.ReadFile(manifestName)
		require.NoError(t, err)

		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "contract", "invokefunction",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet, "--address", validatorAddr,
			h.StringLE(), "update",
			"bytes:"+hex.EncodeToString(rawNef),
			"bytes:"+hex.EncodeToString(rawManifest),
		)
		e.checkTxPersisted(t, "Sent invocation transaction ")

		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "contract", "testinvokefunction",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			h.StringLE(), "getValue")

		res := new(result.Invoke)
		require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
		require.Equal(t, vm.HaltState.String(), res.State)
		require.Len(t, res.Stack, 1)
		require.Equal(t, []byte("on update|sub update"), res.Stack[0].Value())
	})
}

func TestContractInspect(t *testing.T) {
	e := newExecutor(t, false)
	defer e.Close(t)

	// For proper nef generation.
	config.Version = "0.90.0-test"
	const srcPath = "testdata/deploy/main.go"

	tmpDir := path.Join(os.TempDir(), "neogo.test.contract.inspect")
	require.NoError(t, os.Mkdir(tmpDir, os.ModePerm))
	defer os.RemoveAll(tmpDir)

	nefName := path.Join(tmpDir, "deploy.nef")
	manifestName := path.Join(tmpDir, "deploy.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", srcPath,
		"--config", "testdata/deploy/neo-go.yml",
		"--out", nefName, "--manifest", manifestName)

	cmd := []string{"neo-go", "contract", "inspect"}
	t.Run("missing input", func(t *testing.T) {
		e.RunWithError(t, cmd...)
	})
	t.Run("with raw '.go'", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--in", srcPath)...)
		e.Run(t, append(cmd, "--in", srcPath, "--compile")...)
		require.True(t, strings.Contains(e.Out.String(), "SYSCALL"))
	})
	t.Run("with nef", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--in", nefName, "--compile")...)
		e.RunWithError(t, append(cmd, "--in", path.Join(tmpDir, "not.exists"))...)
		e.Run(t, append(cmd, "--in", nefName)...)
		require.True(t, strings.Contains(e.Out.String(), "SYSCALL"))
	})
}

func TestCompileExamples(t *testing.T) {
	const examplePath = "../examples"
	infos, err := ioutil.ReadDir(examplePath)
	require.NoError(t, err)

	// For proper nef generation.
	config.Version = "0.90.0-test"

	tmpDir := os.TempDir()

	e := newExecutor(t, false)
	defer e.Close(t)

	for _, info := range infos {
		t.Run(info.Name(), func(t *testing.T) {
			infos, err := ioutil.ReadDir(path.Join(examplePath, info.Name()))
			require.NoError(t, err)
			require.False(t, len(infos) == 0, "detected smart contract folder with no contract in it")

			outPath := path.Join(tmpDir, info.Name()+".nef")
			manifestPath := path.Join(tmpDir, info.Name()+".manifest.json")
			defer func() {
				os.Remove(outPath)
				os.Remove(manifestPath)
			}()

			cfgName := filterFilename(infos, ".yml")
			opts := []string{
				"neo-go", "contract", "compile",
				"--in", path.Join(examplePath, info.Name()),
				"--out", outPath,
				"--manifest", manifestPath,
				"--config", path.Join(examplePath, info.Name(), cfgName),
			}
			e.Run(t, opts...)
		})
	}

	t.Run("invalid events in manifest", func(t *testing.T) {
		const dir = "./testdata/"
		for _, name := range []string{"invalid1", "invalid2", "invalid3"} {
			outPath := path.Join(tmpDir, name+".nef")
			manifestPath := path.Join(tmpDir, name+".manifest.json")
			defer func() {
				os.Remove(outPath)
				os.Remove(manifestPath)
			}()
			e.RunWithError(t, "neo-go", "contract", "compile",
				"--in", path.Join(dir, name),
				"--out", outPath,
				"--manifest", manifestPath,
				"--config", path.Join(dir, name, "invalid.yml"),
			)
		}
	})
}

func filterFilename(infos []os.FileInfo, ext string) string {
	for _, info := range infos {
		if !info.IsDir() {
			name := info.Name()
			if strings.HasSuffix(name, ext) {
				return name
			}
		}
	}
	return ""
}
