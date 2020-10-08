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

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

func TestComlileAndInvokeFunction(t *testing.T) {
	e := newExecutor(t, true)
	defer e.Close(t)

	// For proper nef generation.
	config.Version = "0.90.0-test"

	tmpDir := os.TempDir()
	nefName := path.Join(tmpDir, "deploy.nef")
	manifestName := path.Join(tmpDir, "deploy.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", "testdata/deploy/main.go", // compile single file
		"--config", "testdata/deploy/neo-go.yml",
		"--out", nefName, "--manifest", manifestName)

	defer func() {
		os.Remove(nefName)
		os.Remove(manifestName)
	}()

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "contract", "deploy",
		"--unittest", "--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet, "--address", validatorAddr,
		"--in", nefName, "--manifest", manifestName)

	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	line = strings.TrimSpace(strings.TrimPrefix(line, "Contract: "))
	h, err := util.Uint160DecodeStringLE(line)
	require.NoError(t, err)
	e.checkTxPersisted(t)

	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "contract", "testinvokefunction",
		"--unittest", "--rpc-endpoint", "http://"+e.RPC.Addr,
		h.StringLE(), "getValue")

	res := new(result.Invoke)
	require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
	require.Equal(t, vm.HaltState.String(), res.State)
	require.Len(t, res.Stack, 1)
	require.Equal(t, []byte("on create|sub create"), res.Stack[0].Value())

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
		realNef, err := nef.FileFromBytes(rawNef)
		require.NoError(t, err)
		rawManifest, err := ioutil.ReadFile(manifestName)
		require.NoError(t, err)

		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "contract", "invokefunction",
			"--unittest", "--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", validatorWallet, "--address", validatorAddr,
			h.StringLE(), "update",
			"bytes:"+hex.EncodeToString(realNef.Script),
			"bytes:"+hex.EncodeToString(rawManifest),
		)
		e.checkTxPersisted(t, "Sent invocation transaction ")

		e.In.WriteString("one\r")
		e.Run(t, "neo-go", "contract", "testinvokefunction",
			"--unittest", "--rpc-endpoint", "http://"+e.RPC.Addr,
			hash.Hash160(realNef.Script).StringLE(), "getValue")

		res := new(result.Invoke)
		require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
		require.Equal(t, vm.HaltState.String(), res.State)
		require.Len(t, res.Stack, 1)
		require.Equal(t, []byte("on update|sub update"), res.Stack[0].Value())
	})
}

func TestInvokeNative(t *testing.T) {
	e := newExecutor(t, true)
	defer e.Close(t)

	cmd := []string{"neo-go", "contract", "testinvokefunction",
		"--unittest", "--rpc-endpoint", "http://" + e.RPC.Addr}
	checkHalt := func(t *testing.T, params ...string) {
		e.Run(t, append(cmd, params...)...)
		res := new(result.Invoke)
		require.NoError(t, json.Unmarshal(e.Out.Bytes(), res))
		require.Equal(t, vm.HaltState.String(), res.State)
	}

	checkHalt(t, "NEO", "getCandidates")
	checkHalt(t, "GAS", "totalSupply")
	checkHalt(t, "Policy", "getMaxBlockSize")
	checkHalt(t, "Designate", "getDesignatedByRole", "int:"+strconv.FormatUint(uint64(native.RoleOracle), 10))
}
