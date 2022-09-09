package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/cli/smartcontract"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCalcHash(t *testing.T) {
	tmpDir := t.TempDir()
	e := newExecutor(t, false)

	nefPath := "./testdata/verify.nef"
	src, err := os.ReadFile(nefPath)
	require.NoError(t, err)
	nefF, err := nef.FileFromBytes(src)
	require.NoError(t, err)
	manifestPath := "./testdata/verify.manifest.json"
	manifestBytes, err := os.ReadFile(manifestPath)
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
	t.Run("invalid nef path", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--sender", sender.StringLE(),
			"--in", "./testdata/verify.nef123", "--manifest", manifestPath)...)
	})
	t.Run("invalid manifest path", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--sender", sender.StringLE(),
			"--in", nefPath, "--manifest", "./testdata/verify.manifest123")...)
	})
	t.Run("invalid nef file", func(t *testing.T) {
		p := filepath.Join(tmpDir, "neogo.calchash.verify.nef")
		require.NoError(t, os.WriteFile(p, src[:4], os.ModePerm))
		e.RunWithError(t, append(cmd, "--sender", sender.StringLE(), "--in", p, "--manifest", manifestPath)...)
	})
	t.Run("invalid manifest file", func(t *testing.T) {
		p := filepath.Join(tmpDir, "neogo.calchash.verify.manifest.json")
		require.NoError(t, os.WriteFile(p, manifestBytes[:4], os.ModePerm))
		e.RunWithError(t, append(cmd, "--sender", sender.StringLE(), "--in", nefPath, "--manifest", p)...)
	})

	cmd = append(cmd, "--in", nefPath, "--manifest", manifestPath)
	expected := state.CreateContractHash(sender, nefF.Checksum, manif.Name)
	t.Run("excessive parameters", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--sender", sender.StringLE(), "something")...)
	})
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

func TestContractBindings(t *testing.T) {
	// For proper nef generation.
	config.Version = "v0.98.1-test"

	// For proper contract init. The actual version as it will be replaced.
	smartcontract.ModVersion = "v0.0.0"

	tmpDir := t.TempDir()
	e := newExecutor(t, false)

	ctrPath := filepath.Join(tmpDir, "testcontract")
	e.Run(t, "neo-go", "contract", "init", "--name", ctrPath)

	srcPath := filepath.Join(ctrPath, "main.go")
	require.NoError(t, os.WriteFile(srcPath, []byte(`package testcontract
import(
	alias "github.com/nspcc-dev/neo-go/pkg/interop/native/ledger"
)
type MyPair struct {
	Key int
	Value string
}
func ToMap(a []MyPair) map[int]string {
	return nil
}
func ToArray(m map[int]string) []MyPair {
	return nil
}
func Block() *alias.Block{
	return alias.GetBlock(1)
}
func Blocks() []*alias.Block {
	return []*alias.Block{
		alias.GetBlock(10),
		alias.GetBlock(11),
	}
}
`), os.ModePerm))

	cfgPath := filepath.Join(ctrPath, "neo-go.yml")
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	bindingsPath := filepath.Join(tmpDir, "bindings.yml")
	cmd := []string{"neo-go", "contract", "compile"}

	cmd = append(cmd, "--in", ctrPath, "--bindings", bindingsPath)

	// Replace `pkg/interop` in go.mod to avoid getting an actual module version.
	goMod := filepath.Join(ctrPath, "go.mod")
	data, err := os.ReadFile(goMod)
	require.NoError(t, err)

	i := bytes.IndexByte(data, '\n')
	data = append([]byte("module myimport.com/testcontract"), data[i:]...)

	wd, err := os.Getwd()
	require.NoError(t, err)
	data = append(data, "\nreplace github.com/nspcc-dev/neo-go/pkg/interop => "...)
	data = append(data, filepath.Join(wd, "../pkg/interop")...)
	require.NoError(t, os.WriteFile(goMod, data, os.ModePerm))

	cmd = append(cmd, "--config", cfgPath,
		"--out", filepath.Join(tmpDir, "out.nef"),
		"--manifest", manifestPath,
		"--bindings", bindingsPath)
	t.Run("excessive parameters", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "something")...)
	})
	e.Run(t, cmd...)
	e.checkEOF(t)
	require.FileExists(t, bindingsPath)

	outPath := filepath.Join(t.TempDir(), "binding.go")
	e.Run(t, "neo-go", "contract", "generate-wrapper",
		"--config", bindingsPath, "--manifest", manifestPath,
		"--out", outPath, "--hash", "0x0123456789987654321001234567899876543210")

	bs, err := os.ReadFile(outPath)
	require.NoError(t, err)
	require.Equal(t, `// Package testcontract contains wrappers for testcontract contract.
package testcontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/ledger"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
	"myimport.com/testcontract"
)

// Hash contains contract hash in big-endian form.
const Hash = "\x10\x32\x54\x76\x98\x89\x67\x45\x23\x01\x10\x32\x54\x76\x98\x89\x67\x45\x23\x01"

// Block invokes `+"`block`"+` method of contract.
func Block() *ledger.Block {
	return neogointernal.CallWithToken(Hash, "block", int(contract.All)).(*ledger.Block)
}

// Blocks invokes `+"`blocks`"+` method of contract.
func Blocks() []*ledger.Block {
	return neogointernal.CallWithToken(Hash, "blocks", int(contract.All)).([]*ledger.Block)
}

// ToArray invokes `+"`toArray`"+` method of contract.
func ToArray(m map[int]string) []testcontract.MyPair {
	return neogointernal.CallWithToken(Hash, "toArray", int(contract.All), m).([]testcontract.MyPair)
}

// ToMap invokes `+"`toMap`"+` method of contract.
func ToMap(a []testcontract.MyPair) map[int]string {
	return neogointernal.CallWithToken(Hash, "toMap", int(contract.All), a).(map[int]string)
}
`, string(bs))
}

func TestContractInitAndCompile(t *testing.T) {
	// For proper nef generation.
	config.Version = "v0.98.1-test"

	// For proper contract init. The actual version as it will be replaced.
	smartcontract.ModVersion = "v0.0.0"

	tmpDir := t.TempDir()
	e := newExecutor(t, false)

	t.Run("no path is provided", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "init")
	})
	t.Run("invalid path", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "init", "--name", "\x00")
	})

	ctrPath := filepath.Join(tmpDir, "testcontract")
	t.Run("excessive parameters", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "init", "--name", ctrPath, "something")
	})

	e.Run(t, "neo-go", "contract", "init", "--name", ctrPath)

	t.Run("don't rewrite existing directory", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "init", "--name", ctrPath)
	})

	srcPath := filepath.Join(ctrPath, "main.go")
	cfgPath := filepath.Join(ctrPath, "neo-go.yml")
	nefPath := filepath.Join(tmpDir, "testcontract.nef")
	manifestPath := filepath.Join(tmpDir, "testcontract.manifest.json")
	cmd := []string{"neo-go", "contract", "compile"}
	t.Run("missing source", func(t *testing.T) {
		e.RunWithError(t, cmd...)
	})

	cmd = append(cmd, "--in", srcPath, "--out", nefPath, "--manifest", manifestPath)
	t.Run("missing config, but require manifest", func(t *testing.T) {
		e.RunWithError(t, cmd...)
	})
	t.Run("provided non-existent config", func(t *testing.T) {
		cfgName := filepath.Join(ctrPath, "notexists.yml")
		e.RunWithError(t, append(cmd, "--config", cfgName)...)
	})
	t.Run("provided corrupted config", func(t *testing.T) {
		data, err := os.ReadFile(cfgPath)
		require.NoError(t, err)
		badCfg := filepath.Join(tmpDir, "bad.yml")
		require.NoError(t, os.WriteFile(badCfg, data[:len(data)-5], os.ModePerm))
		e.RunWithError(t, append(cmd, "--config", badCfg)...)
	})

	// Replace `pkg/interop` in go.mod to avoid getting an actual module version.
	goMod := filepath.Join(ctrPath, "go.mod")
	data, err := os.ReadFile(goMod)
	require.NoError(t, err)

	wd, err := os.Getwd()
	require.NoError(t, err)
	data = append(data, "\nreplace github.com/nspcc-dev/neo-go/pkg/interop => "...)
	data = append(data, filepath.Join(wd, "../pkg/interop")...)
	require.NoError(t, os.WriteFile(goMod, data, os.ModePerm))

	cmd = append(cmd, "--config", cfgPath)

	t.Run("excessive parameters", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "something")...)
	})

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
	tmpDir := t.TempDir()

	nefName := filepath.Join(tmpDir, "deploy.nef")
	manifestName := filepath.Join(tmpDir, "deploy.manifest.json")
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
	eCompile := newExecutor(t, false)

	// For proper nef generation.
	config.Version = "0.90.0-test"
	tmpDir := t.TempDir()

	nefName := filepath.Join(tmpDir, "deploy.nef")
	manifestName := filepath.Join(tmpDir, "deploy.manifest.json")
	eCompile.Run(t, "neo-go", "contract", "compile",
		"--in", "testdata/deploy/main.go", // compile single file
		"--config", "testdata/deploy/neo-go.yml",
		"--out", nefName, "--manifest", manifestName)

	deployContract := func(t *testing.T, haveData bool, scope string) {
		e := newExecutor(t, true)
		cmd := []string{
			"neo-go", "contract", "deploy",
			"--rpc-endpoint", "http://" + e.RPC.Addr,
			"--wallet", validatorWallet, "--address", validatorAddr,
			"--in", nefName, "--manifest", manifestName,
			"--force",
		}

		if haveData {
			cmd = append(cmd, "[", "key1", "12", "key2", "take_me_to_church", "]")
		}
		if scope != "" {
			cmd = append(cmd, "--", validatorAddr+":"+scope)
		} else {
			scope = "CalledByEntry"
		}
		e.In.WriteString("one\r")
		e.Run(t, cmd...)

		tx, _ := e.checkTxPersisted(t, "Sent invocation transaction ")
		require.Equal(t, scope, tx.Signers[0].Scopes.String())
		if !haveData {
			return
		}

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
		require.Equal(t, vmstate.Halt.String(), res.State, res.FaultException)
		require.Len(t, res.Stack, 1)
		require.Equal(t, []byte{12}, res.Stack[0].Value())

		e.Run(t, "neo-go", "contract", "testinvokefunction",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			h.StringLE(),
			"getValueWithKey", "key2",
		)

		res = new(result.Invoke)
		require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
		require.Equal(t, vmstate.Halt.String(), res.State, res.FaultException)
		require.Len(t, res.Stack, 1)
		require.Equal(t, []byte("take_me_to_church"), res.Stack[0].Value())
	}

	deployContract(t, true, "")
	deployContract(t, false, "Global")
	deployContract(t, true, "Global")
}

func TestDeployWithSigners(t *testing.T) {
	e := newExecutor(t, true)

	// For proper nef generation.
	config.Version = "0.90.0-test"
	tmpDir := t.TempDir()

	nefName := filepath.Join(tmpDir, "deploy.nef")
	manifestName := filepath.Join(tmpDir, "deploy.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", "testdata/deploy/main.go",
		"--config", "testdata/deploy/neo-go.yml",
		"--out", nefName, "--manifest", manifestName)

	t.Run("missing nef", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "deploy",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet, "--address", validatorAddr,
			"--in", "", "--manifest", manifestName)
	})
	t.Run("missing manifest", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "deploy",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet, "--address", validatorAddr,
			"--in", nefName, "--manifest", "")
	})
	t.Run("corrupted data", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "deploy",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet, "--address", validatorAddr,
			"--in", nefName, "--manifest", manifestName,
			"[", "str1")
	})
	t.Run("invalid data", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "deploy",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet, "--address", validatorAddr,
			"--in", nefName, "--manifest", manifestName,
			"str1", "str2")
	})
	t.Run("missing wallet", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "deploy",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--address", validatorAddr,
			"--in", nefName, "--manifest", manifestName,
			"[", "str1", "str2", "]")
	})
	t.Run("missing RPC", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "deploy",
			"--wallet", validatorWallet, "--address", validatorAddr,
			"--in", nefName, "--manifest", manifestName,
			"[", "str1", "str2", "]")
	})
	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "contract", "deploy",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet, "--address", validatorAddr,
		"--in", nefName, "--manifest", manifestName,
		"--force",
		"--", validatorAddr+":Global")
	tx, _ := e.checkTxPersisted(t, "Sent invocation transaction ")
	require.Equal(t, transaction.Global, tx.Signers[0].Scopes)
}

func TestContractManifestGroups(t *testing.T) {
	e := newExecutor(t, true)

	// For proper nef generation.
	config.Version = "0.90.0-test"
	tmpDir := t.TempDir()

	_, err := wallet.NewWalletFromFile(testWalletPath)
	require.NoError(t, err)

	nefName := filepath.Join(tmpDir, "deploy.nef")
	manifestName := filepath.Join(tmpDir, "deploy.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", "testdata/deploy/main.go", // compile single file
		"--config", "testdata/deploy/neo-go.yml",
		"--out", nefName, "--manifest", manifestName)

	t.Run("missing wallet", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "manifest", "add-group")
	})
	t.Run("invalid wallet", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "manifest", "add-group",
			"--wallet", t.TempDir())
	})
	t.Run("invalid sender", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "manifest", "add-group",
			"--wallet", testWalletPath, "--address", testWalletAccount,
			"--sender", "not-a-sender")
	})
	t.Run("invalid NEF file", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "manifest", "add-group",
			"--wallet", testWalletPath, "--address", testWalletAccount,
			"--sender", testWalletAccount, "--nef", tmpDir)
	})
	t.Run("corrupted NEF file", func(t *testing.T) {
		f := filepath.Join(tmpDir, "invalid.nef")
		require.NoError(t, os.WriteFile(f, []byte{1, 2, 3}, os.ModePerm))
		e.RunWithError(t, "neo-go", "contract", "manifest", "add-group",
			"--wallet", testWalletPath, "--address", testWalletAccount,
			"--sender", testWalletAccount, "--nef", f)
	})
	t.Run("invalid manifest file", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "manifest", "add-group",
			"--wallet", testWalletPath, "--address", testWalletAccount,
			"--sender", testWalletAccount, "--nef", nefName,
			"--manifest", tmpDir)
	})
	t.Run("corrupted manifest file", func(t *testing.T) {
		f := filepath.Join(tmpDir, "invalid.manifest.json")
		require.NoError(t, os.WriteFile(f, []byte{1, 2, 3}, os.ModePerm))
		e.RunWithError(t, "neo-go", "contract", "manifest", "add-group",
			"--wallet", testWalletPath, "--address", testWalletAccount,
			"--sender", testWalletAccount, "--nef", nefName,
			"--manifest", f)
	})
	t.Run("unknown account", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "manifest", "add-group",
			"--wallet", testWalletPath, "--address", util.Uint160{}.StringLE(),
			"--sender", testWalletAccount, "--nef", nefName,
			"--manifest", manifestName)
	})
	cmd := []string{"neo-go", "contract", "manifest", "add-group",
		"--nef", nefName, "--manifest", manifestName}

	t.Run("excessive parameters", func(t *testing.T) {
		e.RunWithError(t, append(cmd, "--wallet", testWalletPath,
			"--sender", testWalletAccount, "--address", testWalletAccount, "something")...)
	})
	e.In.WriteString("testpass\r")
	e.Run(t, append(cmd, "--wallet", testWalletPath,
		"--sender", testWalletAccount, "--address", testWalletAccount)...)

	e.In.WriteString("testpass\r") // should override signature with the previous sender
	e.Run(t, append(cmd, "--wallet", testWalletPath,
		"--sender", validatorAddr, "--address", testWalletAccount)...)

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "contract", "deploy",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--in", nefName, "--manifest", manifestName,
		"--force",
		"--wallet", validatorWallet, "--address", validatorAddr)
}

func deployVerifyContract(t *testing.T, e *executor) util.Uint160 {
	return deployContract(t, e, "testdata/verify.go", "testdata/verify.yml", validatorWallet, validatorAddr, "one")
}

func deployContract(t *testing.T, e *executor, inPath, configPath, wallet, address, pass string) util.Uint160 {
	tmpDir := t.TempDir()
	nefName := filepath.Join(tmpDir, "contract.nef")
	manifestName := filepath.Join(tmpDir, "contract.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", inPath,
		"--config", configPath,
		"--out", nefName, "--manifest", manifestName)
	e.In.WriteString(pass + "\r")
	e.Run(t, "neo-go", "contract", "deploy",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", wallet, "--address", address,
		"--force",
		"--in", nefName, "--manifest", manifestName)
	e.checkTxPersisted(t, "Sent invocation transaction ")
	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	line = strings.TrimSpace(strings.TrimPrefix(line, "Contract: "))
	h, err := util.Uint160DecodeStringLE(line)
	require.NoError(t, err)
	return h
}

func TestContract_TestInvokeScript(t *testing.T) {
	e := newExecutor(t, true)
	tmpDir := t.TempDir()
	badNef := filepath.Join(tmpDir, "invalid.nef")
	goodNef := filepath.Join(tmpDir, "deploy.nef")
	manifestName := filepath.Join(tmpDir, "deploy.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", "testdata/deploy/main.go", // compile single file
		"--config", "testdata/deploy/neo-go.yml",
		"--out", goodNef, "--manifest", manifestName)

	t.Run("missing in", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://"+e.RPC.Addr)
	})
	t.Run("unexisting in", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--in", badNef)
	})
	t.Run("invalid nef", func(t *testing.T) {
		require.NoError(t, os.WriteFile(badNef, []byte("qwer"), os.ModePerm))
		e.RunWithError(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--in", badNef)
	})
	t.Run("invalid signers", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--in", goodNef, "--", "not-a-valid-signer")
	})
	t.Run("no RPC endpoint", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://123456789",
			"--in", goodNef)
	})
	t.Run("good", func(t *testing.T) {
		e.Run(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--in", goodNef)
	})
	t.Run("good with hashed signer", func(t *testing.T) {
		e.Run(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--in", goodNef, "--", util.Uint160{1, 2, 3}.StringLE())
	})
	t.Run("good with addressed signer", func(t *testing.T) {
		e.Run(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--in", goodNef, "--", address.Uint160ToString(util.Uint160{1, 2, 3}))
	})
	t.Run("historic, invalid", func(t *testing.T) {
		e.RunWithError(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--historic", "bad",
			"--in", goodNef)
	})
	t.Run("historic, index", func(t *testing.T) {
		e.Run(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--historic", "0",
			"--in", goodNef)
	})
	t.Run("historic, hash", func(t *testing.T) {
		e.Run(t, "neo-go", "contract", "testinvokescript",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--historic", e.Chain.GetHeaderHash(0).StringLE(),
			"--in", goodNef)
	})
}

func TestComlileAndInvokeFunction(t *testing.T) {
	e := newExecutor(t, true)

	// For proper nef generation.
	config.Version = "0.90.0-test"
	tmpDir := t.TempDir()

	nefName := filepath.Join(tmpDir, "deploy.nef")
	manifestName := filepath.Join(tmpDir, "deploy.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", "testdata/deploy/main.go", // compile single file
		"--config", "testdata/deploy/neo-go.yml",
		"--out", nefName, "--manifest", manifestName)

	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")
	cfg := config.Wallet{
		Path:     validatorWallet,
		Password: "one",
	}
	yml, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, yml, 0666))
	e.Run(t, "neo-go", "contract", "deploy",
		"--rpc-endpoint", "http://"+e.RPC.Addr, "--force",
		"--wallet-config", configPath, "--address", validatorAddr,
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

	checkGetValueOut := func(str string) {
		res := new(result.Invoke)
		require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
		require.Equal(t, vmstate.Halt.String(), res.State, res.FaultException)
		require.Len(t, res.Stack, 1)
		require.Equal(t, []byte(str), res.Stack[0].Value())
	}
	checkGetValueOut("on create|sub create")

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
			cmd := append(cmd, "--wallet", filepath.Join(tmpDir, "not.exists"),
				h.StringLE(), "getValue")
			e.RunWithError(t, cmd...)
		})
		t.Run("corrupted wallet", func(t *testing.T) {
			tmp := t.TempDir()
			tmpPath := filepath.Join(tmp, "wallet.json")
			require.NoError(t, os.WriteFile(tmpPath, []byte("{"), os.ModePerm))

			cmd := append(cmd, "--wallet", tmpPath,
				h.StringLE(), "getValue")
			e.RunWithError(t, cmd...)
		})
		t.Run("non-existent address", func(t *testing.T) {
			cmd := append(cmd, "--wallet", validatorWallet,
				"--address", random.Uint160().StringLE(),
				h.StringLE(), "getValue")
			e.RunWithError(t, cmd...)
		})
		t.Run("invalid password", func(t *testing.T) {
			e.In.WriteString("invalid_password\r")
			cmd := append(cmd, "--wallet", validatorWallet,
				h.StringLE(), "getValue")
			e.RunWithError(t, cmd...)
		})
		t.Run("good: default address", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.In.WriteString("y\r")
			e.Run(t, append(cmd, "--wallet", validatorWallet, h.StringLE(), "getValue")...)
		})
		t.Run("good: from wallet config", func(t *testing.T) {
			e.In.WriteString("y\r")
			e.Run(t, append(cmd, "--wallet-config", configPath, h.StringLE(), "getValue")...)
		})

		cmd = append(cmd, "--wallet", validatorWallet, "--address", validatorAddr)
		t.Run("cancelled", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.In.WriteString("n\r")
			e.RunWithError(t, append(cmd, h.StringLE(), "getValue")...)
		})
		t.Run("confirmed", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.In.WriteString("y\r")
			e.Run(t, append(cmd, h.StringLE(), "getValue")...)
		})

		t.Run("failind method", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.In.WriteString("y\r")
			e.RunWithError(t, append(cmd, h.StringLE(), "fail")...)

			e.In.WriteString("one\r")
			e.Run(t, append(cmd, "--force", h.StringLE(), "fail")...)
		})

		t.Run("cosigner is deployed contract", func(t *testing.T) {
			e.In.WriteString("one\r")
			e.In.WriteString("y\r")
			e.Run(t, append(cmd, h.StringLE(), "getValue",
				"--", validatorAddr, hVerify.StringLE())...)
		})
	})

	t.Run("real invoke and save tx", func(t *testing.T) {
		txout := filepath.Join(tmpDir, "test_contract_tx.json")

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
			t.Run("cosigner is sender (none)", func(t *testing.T) {
				e.In.WriteString("one\r")
				e.RunWithError(t, append(cmd, h.StringLE(), "checkSenderWitness", "--", validatorAddr+":None")...)
			})
			t.Run("cosigner is sender (customcontract)", func(t *testing.T) {
				e.In.WriteString("one\r")
				e.Run(t, append(cmd, h.StringLE(), "checkSenderWitness", "--", validatorAddr+":CustomContracts:"+h.StringLE())...)
			})
			t.Run("cosigner is sender (global)", func(t *testing.T) {
				e.In.WriteString("one\r")
				e.Run(t, append(cmd, h.StringLE(), "checkSenderWitness", "--", validatorAddr+":Global")...)
			})

			acc, err := wallet.NewAccount()
			require.NoError(t, err)
			pk, err := keys.NewPrivateKey()
			require.NoError(t, err)
			err = acc.ConvertMultisig(2, keys.PublicKeys{acc.PublicKey(), pk.PublicKey()})
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
			require.Equal(t, vmstate.Halt.String(), res.State)
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
			require.Equal(t, vmstate.Halt.String(), res.State)
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

	var (
		hashBeforeUpdate  util.Uint256
		indexBeforeUpdate uint32
		indexAfterUpdate  uint32
		stateBeforeUpdate util.Uint256
	)
	t.Run("Update", func(t *testing.T) {
		nefName := filepath.Join(tmpDir, "updated.nef")
		manifestName := filepath.Join(tmpDir, "updated.manifest.json")
		e.Run(t, "neo-go", "contract", "compile",
			"--config", "testdata/deploy/neo-go.yml",
			"--in", "testdata/deploy/", // compile all files in dir
			"--out", nefName, "--manifest", manifestName)

		t.Cleanup(func() {
			os.Remove(nefName)
			os.Remove(manifestName)
		})

		rawNef, err := os.ReadFile(nefName)
		require.NoError(t, err)
		rawManifest, err := os.ReadFile(manifestName)
		require.NoError(t, err)

		indexBeforeUpdate = e.Chain.BlockHeight()
		hashBeforeUpdate = e.Chain.CurrentHeaderHash()
		mptBeforeUpdate, err := e.Chain.GetStateRoot(indexBeforeUpdate)
		require.NoError(t, err)
		stateBeforeUpdate = mptBeforeUpdate.Root
		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "contract", "invokefunction",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet, "--address", validatorAddr,
			"--force",
			h.StringLE(), "update",
			"bytes:"+hex.EncodeToString(rawNef),
			"bytes:"+hex.EncodeToString(rawManifest),
		)
		e.checkTxPersisted(t, "Sent invocation transaction ")

		indexAfterUpdate = e.Chain.BlockHeight()
		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "contract", "testinvokefunction",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			h.StringLE(), "getValue")
		checkGetValueOut("on update|sub update")
	})
	t.Run("historic", func(t *testing.T) {
		t.Run("bad ref", func(t *testing.T) {
			e.RunWithError(t, "neo-go", "contract", "testinvokefunction",
				"--rpc-endpoint", "http://"+e.RPC.Addr,
				"--historic", "bad",
				h.StringLE(), "getValue")
		})
		for name, ref := range map[string]string{
			"by index":      strconv.FormatUint(uint64(indexBeforeUpdate), 10),
			"by block hash": hashBeforeUpdate.StringLE(),
			"by state hash": stateBeforeUpdate.StringLE(),
		} {
			t.Run(name, func(t *testing.T) {
				e.Run(t, "neo-go", "contract", "testinvokefunction",
					"--rpc-endpoint", "http://"+e.RPC.Addr,
					"--historic", ref,
					h.StringLE(), "getValue")
			})
			checkGetValueOut("on create|sub create")
		}
		t.Run("updated historic", func(t *testing.T) {
			e.Run(t, "neo-go", "contract", "testinvokefunction",
				"--rpc-endpoint", "http://"+e.RPC.Addr,
				"--historic", strconv.FormatUint(uint64(indexAfterUpdate), 10),
				h.StringLE(), "getValue")
			checkGetValueOut("on update|sub update")
		})
	})
}

func TestContractInspect(t *testing.T) {
	e := newExecutor(t, false)

	// For proper nef generation.
	config.Version = "0.90.0-test"
	const srcPath = "testdata/deploy/main.go"
	tmpDir := t.TempDir()

	nefName := filepath.Join(tmpDir, "deploy.nef")
	manifestName := filepath.Join(tmpDir, "deploy.manifest.json")
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
		e.RunWithError(t, append(cmd, "--in", filepath.Join(tmpDir, "not.exists"))...)
		e.RunWithError(t, append(cmd, "--in", nefName, "something")...)
		e.Run(t, append(cmd, "--in", nefName)...)
		require.True(t, strings.Contains(e.Out.String(), "SYSCALL"))
	})
}

func TestCompileExamples(t *testing.T) {
	tmpDir := t.TempDir()
	const examplePath = "../examples"
	infos, err := os.ReadDir(examplePath)
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
			infos, err := os.ReadDir(filepath.Join(examplePath, info.Name()))
			require.NoError(t, err)
			require.False(t, len(infos) == 0, "detected smart contract folder with no contract in it")

			outF := filepath.Join(tmpDir, info.Name()+".nef")
			manifestF := filepath.Join(tmpDir, info.Name()+".manifest.json")

			cfgName := filterFilename(infos, ".yml")
			opts := []string{
				"neo-go", "contract", "compile",
				"--in", filepath.Join(examplePath, info.Name()),
				"--out", outF,
				"--manifest", manifestF,
				"--config", filepath.Join(examplePath, info.Name(), cfgName),
			}
			e.Run(t, opts...)

			if info.Name() == "storage" {
				rawM, err := os.ReadFile(manifestF)
				require.NoError(t, err)

				m := new(manifest.Manifest)
				require.NoError(t, json.Unmarshal(rawM, m))

				require.Nil(t, m.ABI.GetMethod("getDefault", 0))
				require.NotNil(t, m.ABI.GetMethod("get", 0))
				require.NotNil(t, m.ABI.GetMethod("get", 1))

				require.Nil(t, m.ABI.GetMethod("putDefault", 1))
				require.NotNil(t, m.ABI.GetMethod("put", 1))
				require.NotNil(t, m.ABI.GetMethod("put", 2))
			}
		})
	}

	t.Run("invalid manifest", func(t *testing.T) {
		const dir = "./testdata/"
		for _, name := range []string{"invalid1", "invalid2", "invalid3", "invalid4"} {
			outF := filepath.Join(tmpDir, name+".nef")
			manifestF := filepath.Join(tmpDir, name+".manifest.json")
			e.RunWithError(t, "neo-go", "contract", "compile",
				"--in", filepath.Join(dir, name),
				"--out", outF,
				"--manifest", manifestF,
				"--config", filepath.Join(dir, name, "invalid.yml"),
			)
		}
	})
}

func filterFilename(infos []os.DirEntry, ext string) string {
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
