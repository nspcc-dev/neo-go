package neotest

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/nspcc-dev/neo-go/cli/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

// Contract contains contract info for deployment.
type Contract struct {
	Hash      util.Uint160
	NEF       *nef.File
	Manifest  *manifest.Manifest
	DebugInfo *compiler.DebugInfo
}

// contracts caches the compiled contracts from FS across multiple tests. The key is a
// concatenation of the source file path and the config file path split by | symbol.
var contracts = make(map[string]*Contract)

// CompileSource compiles a contract from the reader and returns its NEF, manifest and hash.
// Compiled contract will have "contract.go" used for its file name and coverage
// data collection can give wrong results for it, so it's recommended to disable
// coverage ([*Executor.DisableCoverage]) when you're deploying contracts
// compiled via this function.
func CompileSource(t testing.TB, sender util.Uint160, src io.Reader, opts *compiler.Options) *Contract {
	// nef.NewFile() cares about version a lot.
	config.Version = "neotest"

	ne, di, err := compiler.CompileWithOptions("contract.go", src, opts)
	require.NoError(t, err)

	m, err := compiler.CreateManifest(di, opts)
	require.NoError(t, err)

	return &Contract{
		Hash:      state.CreateContractHash(sender, ne.Checksum, m.Name),
		NEF:       ne,
		Manifest:  m,
		DebugInfo: di,
	}
}

// CompileFile compiles a contract from the file and returns its NEF, manifest and hash.
// It uses contracts cashes.
func CompileFile(t testing.TB, sender util.Uint160, srcPath string, configPath string) *Contract {
	cacheKey := srcPath + "|" + configPath
	if c, ok := contracts[cacheKey]; ok {
		return c
	}

	// nef.NewFile() cares about version a lot.
	config.Version = "neotest"

	conf, err := smartcontract.ParseContractConfig(configPath)
	require.NoError(t, err)

	o := &compiler.Options{}
	o.Name = conf.Name
	o.ContractEvents = conf.Events
	o.ContractSupportedStandards = conf.SupportedStandards
	o.Permissions = make([]manifest.Permission, len(conf.Permissions))
	for i := range conf.Permissions {
		o.Permissions[i] = manifest.Permission(conf.Permissions[i])
	}
	o.SafeMethods = conf.SafeMethods
	o.Overloads = conf.Overloads
	o.SourceURL = conf.SourceURL
	ne, di, err := compiler.CompileWithOptions(srcPath, nil, o)
	require.NoError(t, err)
	m, err := compiler.CreateManifest(di, o)
	require.NoError(t, err)

	c := &Contract{
		Hash:      state.CreateContractHash(sender, ne.Checksum, m.Name),
		NEF:       ne,
		Manifest:  m,
		DebugInfo: di,
	}
	contracts[cacheKey] = c
	return c
}

// ReadNEF loads a contract from the specified NEF and manifest files.
func ReadNEF(t testing.TB, sender util.Uint160, nefPath, manifestPath string) *Contract {
	cacheKey := sender.StringLE() + "|" + nefPath + "|" + manifestPath
	if c, ok := contracts[cacheKey]; ok {
		return c
	}

	nefBytes, err := os.ReadFile(nefPath)
	require.NoError(t, err)

	ne, err := nef.FileFromBytes(nefBytes)
	require.NoError(t, err)

	manifestBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	m := new(manifest.Manifest)
	err = json.Unmarshal(manifestBytes, m)
	require.NoError(t, err)

	hash := state.CreateContractHash(sender, ne.Checksum, m.Name)
	err = m.IsValid(hash, true)
	require.NoError(t, err)

	c := &Contract{
		Hash:     hash,
		NEF:      &ne,
		Manifest: m,
	}

	contracts[cacheKey] = c
	return c
}
