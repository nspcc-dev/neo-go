package neotest

import (
	"io"
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

// contracts caches the compiled contracts from FS across multiple tests.
var contracts = make(map[string]*Contract)

// CompileSource compiles a contract from the reader and returns its NEF, manifest and hash.
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
func CompileFile(t testing.TB, sender util.Uint160, srcPath string, configPath string) *Contract {
	if c, ok := contracts[srcPath]; ok {
		return c
	}

	// nef.NewFile() cares about version a lot.
	config.Version = "neotest"

	ne, di, err := compiler.CompileWithOptions(srcPath, nil, nil)
	require.NoError(t, err)

	conf, err := smartcontract.ParseContractConfig(configPath)
	require.NoError(t, err)

	o := &compiler.Options{}
	o.Name = conf.Name
	o.ContractEvents = conf.Events
	o.DeclaredNamedTypes = conf.NamedTypes
	o.ContractSupportedStandards = conf.SupportedStandards
	o.Permissions = make([]manifest.Permission, len(conf.Permissions))
	for i := range conf.Permissions {
		o.Permissions[i] = manifest.Permission(conf.Permissions[i])
	}
	o.SafeMethods = conf.SafeMethods
	o.Overloads = conf.Overloads
	o.SourceURL = conf.SourceURL
	m, err := compiler.CreateManifest(di, o)
	require.NoError(t, err)

	c := &Contract{
		Hash:      state.CreateContractHash(sender, ne.Checksum, m.Name),
		NEF:       ne,
		Manifest:  m,
		DebugInfo: di,
	}
	contracts[srcPath] = c
	return c
}
