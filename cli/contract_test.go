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
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestCalcHash(t *testing.T) {
	e := newExecutor(t, false)

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
		p, err := ioutil.TempFile("", "neogo.calchash.verify.nef")
		require.NoError(t, err)
		t.Cleanup(func() {
			os.Remove(p.Name())
		})
		require.NoError(t, ioutil.WriteFile(p.Name(), src[:4], os.ModePerm))
		e.RunWithError(t, append(cmd, "--sender", sender.StringLE(), "--in", p.Name(), "--manifest", manifestPath)...)
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
	tmpDir, err := ioutil.TempDir("", "neogo.inittest")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	e := newExecutor(t, false)

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
	e := newExecutorWithConfig(t, true, true, func(c *config.Config) {
		c.ApplicationConfiguration.RPC.MaxGasInvoke = fixedn.Fixed8(1)
	})

	// For proper nef generation.
	config.Version = "0.90.0-test"

	tmpDir, err := ioutil.TempDir("", "neogo.test.deployfail")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

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

func TestContractDeployWithData(t *testing.T) {
	e := newExecutor(t, true)

	// For proper nef generation.
	config.Version = "0.90.0-test"

	tmpDir, err := ioutil.TempDir("", "neogo.test.deployfail")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	nefName := path.Join(tmpDir, "deploy.nef")
	manifestName := path.Join(tmpDir, "deploy.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", "testdata/deploy/main.go", // compile single file
		"--config", "testdata/deploy/neo-go.yml",
		"--out", nefName, "--manifest", manifestName)

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "contract", "deploy",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet, "--address", validatorAddr,
		"--in", nefName, "--manifest", manifestName,
		"[", "key1", "12", "key2", "take_me_to_church", "]")

	e.checkTxPersisted(t, "Sent invocation transaction ")
	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	line = strings.TrimSpace(strings.TrimPrefix(line, "Contract: "))
	h, err := util.Uint160DecodeStringLE(line)
	require.NoError(t, err)

	e.Run(t, "neo-go", "contract", "testinvokefunction",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		h.StringLE(),
		"getValueWithKey", "key1",
	)

	res := new(result.Invoke)
	require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
	require.Equal(t, vm.HaltState.String(), res.State, res.FaultException)
	require.Len(t, res.Stack, 1)
	require.Equal(t, []byte{12}, res.Stack[0].Value())

	e.Run(t, "neo-go", "contract", "testinvokefunction",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		h.StringLE(),
		"getValueWithKey", "key2",
	)

	res = new(result.Invoke)
	require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
	require.Equal(t, vm.HaltState.String(), res.State, res.FaultException)
	require.Len(t, res.Stack, 1)
	require.Equal(t, []byte("take_me_to_church"), res.Stack[0].Value())
}

func deployVerifyContract(t *testing.T, e *executor) util.Uint160 {
	return deployContract(t, e, "testdata/verify.go", "testdata/verify.yml", validatorWallet, validatorAddr, "one")
}

func deployContract(t *testing.T, e *executor, inPath, configPath, wallet, address, pass string) util.Uint160 {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "neogo.test.deploycontract*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})
	nefName := path.Join(tmpDir, "contract.nef")
	manifestName := path.Join(tmpDir, "contract.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", inPath,
		"--config", configPath,
		"--out", nefName, "--manifest", manifestName)
	e.In.WriteString(pass + "\r")
	e.Run(t, "neo-go", "contract", "deploy",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", wallet, "--address", address,
		"--in", nefName, "--manifest", manifestName)
	e.checkTxPersisted(t, "Sent invocation transaction ")
	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	line = strings.TrimSpace(strings.TrimPrefix(line, "Contract: "))
	h, err := util.Uint160DecodeStringLE(line)
	require.NoError(t, err)
	return h
}

func TestComlileAndInvokeFunction(t *testing.T) {
	e := newExecutor(t, true)

	// For proper nef generation.
	config.Version = "0.90.0-test"

	tmpDir, err := ioutil.TempDir("", "neogo.test.compileandinvoke")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

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

	e.checkTxPersisted(t, "Sent invocation transaction ")
	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	line = strings.TrimSpace(strings.TrimPrefix(line, "Contract: "))
	h, err := util.Uint160DecodeStringLE(line)
	require.NoError(t, err)

	t.Run("check calc hash", func(t *testing.T) {
		// missing sender
		e.RunWithError(t, "neo-go", "contract", "calc-hash",
			"--in", nefName,
			"--manifest", manifestName)

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

	// deploy verification contract
	hVerify := deployVerifyContract(t, e)

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

		t.Run("cosigner is deployed contract", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.Run(t, append(cmd, h.StringLE(), "getValue",
				"--", validatorAddr, hVerify.StringLE())...)
		})
	})

	t.Run("real invoke and save tx", func(t *testing.T) {
		txout := path.Join(tmpDir, "test_contract_tx.json")

		cmd = []string{"neo-go", "contract", "invokefunction",
			"--rpc-endpoint", "http://" + e.RPC.Addr,
			"--out", txout,
			"--wallet", validatorWallet, "--address", validatorAddr,
		}

		t.Run("without cosigner", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.Run(t, append(cmd, hVerify.StringLE(), "verify")...)
		})

		t.Run("with cosigner", func(t *testing.T) {
			t.Run("cosigner is sender", func(t *testing.T) {
				e.In.WriteString("one\r")
				e.Run(t, append(cmd, hVerify.StringLE(), "verify", "--", validatorAddr+":Global")...)
			})

			acc, err := wallet.NewAccount()
			require.NoError(t, err)
			pk, err := keys.NewPrivateKey()
			require.NoError(t, err)
			err = acc.ConvertMultisig(2, keys.PublicKeys{acc.PrivateKey().PublicKey(), pk.PublicKey()})
			require.NoError(t, err)

			t.Run("cosigner is multisig account", func(t *testing.T) {
				t.Run("missing in the wallet", func(t *testing.T) {
					e.In.WriteString("one\r")
					e.RunWithError(t, append(cmd, hVerify.StringLE(), "verify", "--", acc.Address)...)
				})

				t.Run("good", func(t *testing.T) {
					e.In.WriteString("one\r")
					e.Run(t, append(cmd, hVerify.StringLE(), "verify", "--", multisigAddr)...)
				})
			})

			t.Run("cosigner is deployed contract", func(t *testing.T) {
				t.Run("missing in the wallet", func(t *testing.T) {
					e.In.WriteString("one\r")
					e.RunWithError(t, append(cmd, hVerify.StringLE(), "verify", "--", h.StringLE())...)
				})

				t.Run("good", func(t *testing.T) {
					e.In.WriteString("one\r")
					e.Run(t, append(cmd, hVerify.StringLE(), "verify", "--", hVerify.StringLE())...)
				})
			})
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

		t.Cleanup(func() {
			os.Remove(nefName)
			os.Remove(manifestName)
		})

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

	// For proper nef generation.
	config.Version = "0.90.0-test"
	const srcPath = "testdata/deploy/main.go"

	tmpDir, err := ioutil.TempDir("", "neogo.test.contract.inspect")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

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

	e := newExecutor(t, false)

	for _, info := range infos {
		if !info.IsDir() {
			// example smart contracts are located in the `/examples` subdirectories, but
			// there are also a couple of files inside the `/examples` which doesn't need to be compiled
			continue
		}
		t.Run(info.Name(), func(t *testing.T) {
			infos, err := ioutil.ReadDir(path.Join(examplePath, info.Name()))
			require.NoError(t, err)
			require.False(t, len(infos) == 0, "detected smart contract folder with no contract in it")

			outF, err := ioutil.TempFile("", info.Name()+".nef")
			require.NoError(t, err)
			manifestF, err := ioutil.TempFile("", info.Name()+".manifest.json")
			require.NoError(t, err)
			t.Cleanup(func() {
				os.Remove(outF.Name())
				os.Remove(manifestF.Name())
			})

			cfgName := filterFilename(infos, ".yml")
			opts := []string{
				"neo-go", "contract", "compile",
				"--in", path.Join(examplePath, info.Name()),
				"--out", outF.Name(),
				"--manifest", manifestF.Name(),
				"--config", path.Join(examplePath, info.Name(), cfgName),
			}
			e.Run(t, opts...)
		})
	}

	t.Run("invalid events in manifest", func(t *testing.T) {
		const dir = "./testdata/"
		for _, name := range []string{"invalid1", "invalid2", "invalid3"} {
			outF, err := ioutil.TempFile("", name+".nef")
			require.NoError(t, err)
			manifestF, err := ioutil.TempFile("", name+".manifest.json")
			require.NoError(t, err)
			t.Cleanup(func() {
				os.Remove(outF.Name())
				os.Remove(manifestF.Name())
			})
			e.RunWithError(t, "neo-go", "contract", "compile",
				"--in", path.Join(dir, name),
				"--out", outF.Name(),
				"--manifest", manifestF.Name(),
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
