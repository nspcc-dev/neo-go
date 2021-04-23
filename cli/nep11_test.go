package main

import (
	"os"
	"path"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/stretchr/testify/require"
)

func TestNEP11Import(t *testing.T) {
	e := newExecutor(t, true)

	tmpDir := os.TempDir()
	walletPath := path.Join(tmpDir, "walletForImport.json")
	defer os.Remove(walletPath)

	nnsContractHash, err := e.Chain.GetNativeContractScriptHash(nativenames.NameService)
	require.NoError(t, err)
	neoContractHash, err := e.Chain.GetNativeContractScriptHash(nativenames.Neo)
	require.NoError(t, err)
	e.Run(t, "neo-go", "wallet", "init", "--wallet", walletPath)

	args := []string{
		"neo-go", "wallet", "nep11", "import",
		"--rpc-endpoint", "http://" + e.RPC.Addr,
		"--wallet", walletPath,
	}
	// missing token hash
	e.RunWithError(t, args...)

	// good
	e.Run(t, append(args, "--token", nnsContractHash.StringLE())...)

	// already exists
	e.RunWithError(t, append(args, "--token", nnsContractHash.StringLE())...)

	// not a NEP11 token
	e.RunWithError(t, append(args, "--token", neoContractHash.StringLE())...)

	t.Run("Info", func(t *testing.T) {
		checkNNSInfo := func(t *testing.T) {
			e.checkNextLine(t, "^Name:\\s*NameService")
			e.checkNextLine(t, "^Symbol:\\s*NNS")
			e.checkNextLine(t, "^Hash:\\s*"+nnsContractHash.StringLE())
			e.checkNextLine(t, "^Decimals:\\s*0")
			e.checkNextLine(t, "^Address:\\s*"+address.Uint160ToString(nnsContractHash))
			e.checkNextLine(t, "^Standard:\\s*"+string(manifest.NEP11StandardName))
		}
		t.Run("WithToken", func(t *testing.T) {
			e.Run(t, "neo-go", "wallet", "nep11", "info",
				"--wallet", walletPath, "--token", nnsContractHash.StringLE())
			checkNNSInfo(t)
		})
		t.Run("NoToken", func(t *testing.T) {
			e.Run(t, "neo-go", "wallet", "nep11", "info",
				"--wallet", walletPath)
			checkNNSInfo(t)
		})
	})
}
