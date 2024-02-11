package vm

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	gio "io"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chzyer/readline"
	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/versionutil"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dboper"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

// Keep contract NEFs consistent between runs.
const _ = versionutil.TestVersion

type readCloser struct {
	sync.Mutex
	bytes.Buffer
}

func (r *readCloser) Close() error {
	return nil
}

func (r *readCloser) Read(p []byte) (int, error) {
	r.Lock()
	defer r.Unlock()
	return r.Buffer.Read(p)
}

func (r *readCloser) WriteString(s string) {
	r.Lock()
	defer r.Unlock()
	r.Buffer.WriteString(s)
}

type executor struct {
	in   *readCloser
	out  *bytes.Buffer
	cli  *CLI
	ch   chan struct{}
	exit atomic.Bool
}

func newTestVMCLI(t *testing.T) *executor {
	return newTestVMCLIWithLogo(t, false)
}

func newTestVMCLIWithLogo(t *testing.T, printLogo bool) *executor {
	return newTestVMCLIWithLogoAndCustomConfig(t, printLogo, nil)
}

func newTestVMCLIWithLogoAndCustomConfig(t *testing.T, printLogo bool, cfg *config.Config) *executor {
	e := &executor{
		in:  &readCloser{Buffer: *bytes.NewBuffer(nil)},
		out: bytes.NewBuffer(nil),
		ch:  make(chan struct{}),
	}
	var c config.Config
	if cfg == nil {
		configPath := filepath.Join("..", "..", "config", "protocol.unit_testnet.single.yml")
		var err error
		c, err = config.LoadFile(configPath, filepath.Join("..", "..", "config"))
		require.NoError(t, err, "could not load chain config")
		require.Equal(t, filepath.Join("..", "..", "testdata", "wallet1_solo.json"), c.ApplicationConfiguration.Consensus.UnlockWallet.Path)
		c.ApplicationConfiguration.DBConfiguration.Type = dbconfig.InMemoryDB
	} else {
		c = *cfg
	}
	var err error
	e.cli, err = NewWithConfig(printLogo,
		func(int) { e.exit.Store(true) },
		&readline.Config{
			Prompt: "",
			Stdin:  e.in,
			Stderr: e.out,
			Stdout: e.out,
			FuncIsTerminal: func() bool {
				return false
			},
		}, c)
	require.NoError(t, err)
	return e
}

// newTestVMClIWithState creates executor backed by level DB filled by simple chain.
// LevelDB-backed CLI must be exited on cleanup.
func newTestVMClIWithState(t *testing.T) *executor {
	// Firstly create a DB with chain, save and close it.
	path := t.TempDir()
	opts := dbconfig.LevelDBOptions{
		DataDirectoryPath: path,
	}
	store, err := storage.NewLevelDBStore(opts)
	require.NoError(t, err)
	customConfig := func(c *config.Blockchain) {
		c.StateRootInHeader = true // Need for P2PStateExchangeExtensions check.
		c.P2PSigExtensions = true  // Need for basic chain initializer.
	}
	bc, validators, committee, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, store)
	require.NoError(t, err)
	go bc.Run()
	e := neotest.NewExecutor(t, bc, validators, committee)
	basicchain.InitSimple(t, "../../", e)
	bc.Close()

	// After that create CLI backed by created chain.
	configPath := "../../config/protocol.unit_testnet.yml"
	cfg, err := config.LoadFile(configPath)
	require.NoError(t, err)
	cfg.ApplicationConfiguration.DBConfiguration.Type = dbconfig.LevelDB
	cfg.ApplicationConfiguration.DBConfiguration.LevelDBOptions = opts
	cfg.ProtocolConfiguration.StateRootInHeader = true
	return newTestVMCLIWithLogoAndCustomConfig(t, false, &cfg)
}

func (e *executor) runProg(t *testing.T, commands ...string) {
	e.runProgWithTimeout(t, 4*time.Second, commands...)
}

func (e *executor) runProgWithTimeout(t *testing.T, timeout time.Duration, commands ...string) {
	cmd := strings.Join(commands, "\n") + "\n"
	e.in.WriteString(cmd + "\n")
	go func() {
		require.NoError(t, e.cli.Run())
		close(e.ch)
	}()
	select {
	case <-e.ch:
	case <-time.After(timeout):
		require.Fail(t, "command took too long time")
	}
}

func (e *executor) checkNextLine(t *testing.T, expected string) {
	line, err := e.out.ReadString('\n')
	require.NoError(t, err)
	require.Regexp(t, expected, line)
}

func (e *executor) checkNextLineExact(t *testing.T, expected string) {
	line, err := e.out.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, expected, line)
}

func (e *executor) checkError(t *testing.T, expectedErr error) {
	line, err := e.out.ReadString('\n')
	require.NoError(t, err)
	expected := "Error: " + expectedErr.Error()
	require.True(t, strings.HasPrefix(line, expected), fmt.Errorf("expected `%s`, got `%s`", expected, line))
}

func (e *executor) checkStack(t *testing.T, items ...any) {
	d := json.NewDecoder(e.out)
	var actual any
	require.NoError(t, d.Decode(&actual))
	rawActual, err := json.Marshal(actual)
	require.NoError(t, err)

	expected := vm.NewStack("")
	for i := range items {
		expected.PushVal(items[i])
	}
	rawExpected, err := json.Marshal(expected)
	require.NoError(t, err)
	require.JSONEq(t, string(rawExpected), string(rawActual))

	// Decoder has it's own buffer, we need to return unread part to the output.
	outRemain := e.out.String()
	e.out.Reset()
	_, err = gio.Copy(e.out, d.Buffered())
	require.NoError(t, err)
	e.out.WriteString(outRemain)
	_, err = e.out.ReadString('\n')
	require.NoError(t, err)
}

func (e *executor) checkEvents(t *testing.T, isKeywordExpected bool, events ...state.NotificationEvent) {
	if isKeywordExpected {
		e.checkNextLine(t, "Events:")
	}
	d := json.NewDecoder(e.out)
	var actual any
	require.NoError(t, d.Decode(&actual))
	rawActual, err := json.Marshal(actual)
	require.NoError(t, err)

	rawExpected, err := json.Marshal(events)
	require.NoError(t, err)
	require.JSONEq(t, string(rawExpected), string(rawActual))

	// Decoder has it's own buffer, we need to return unread part to the output.
	outRemain := e.out.String()
	e.out.Reset()
	_, err = gio.Copy(e.out, d.Buffered())
	require.NoError(t, err)
	e.out.WriteString(outRemain)
	_, err = e.out.ReadString('\n')
	require.NoError(t, err)
}

func (e *executor) checkStorage(t *testing.T, kvs ...storage.KeyValue) {
	for _, kv := range kvs {
		e.checkNextLine(t, fmt.Sprintf("%s: %s", hex.EncodeToString(kv.Key), hex.EncodeToString(kv.Value)))
	}
}

type storageChange struct {
	ContractID int32
	dboper.Operation
}

func (e *executor) checkChange(t *testing.T, c storageChange) {
	e.checkNextLine(t, fmt.Sprintf("Contract ID: %d", c.ContractID))
	e.checkNextLine(t, fmt.Sprintf("State: %s", c.State))
	e.checkNextLine(t, fmt.Sprintf("Key: %s", hex.EncodeToString(c.Key)))
	if c.Value != nil {
		e.checkNextLine(t, fmt.Sprintf("Value: %s", hex.EncodeToString(c.Value)))
	}
}

func (e *executor) checkSlot(t *testing.T, items ...any) {
	d := json.NewDecoder(e.out)
	var actual any
	require.NoError(t, d.Decode(&actual))
	rawActual, err := json.Marshal(actual)
	require.NoError(t, err)

	expected := make([]json.RawMessage, len(items))
	for i := range items {
		if items[i] == nil {
			expected[i] = []byte("null")
			continue
		}
		data, err := stackitem.ToJSONWithTypes(stackitem.Make(items[i]))
		require.NoError(t, err)
		expected[i] = data
	}
	rawExpected, err := json.MarshalIndent(expected, "", "    ")
	require.NoError(t, err)
	require.JSONEq(t, string(rawExpected), string(rawActual))

	// Decoder has it's own buffer, we need to return unread part to the output.
	outRemain := e.out.String()
	e.out.Reset()
	_, err = gio.Copy(e.out, d.Buffered())
	require.NoError(t, err)
	e.out.WriteString(outRemain)
	_, err = e.out.ReadString('\n')
	require.NoError(t, err)
}

func TestRun_WithNewVMContextAndBreakpoints(t *testing.T) {
	t.Run("contract without init", func(t *testing.T) {
		src := `package kek
		func Main(a, b int) int {
			var c = a + b
			return c + 5
		}`
		tmpDir := t.TempDir()
		filename := prepareLoadgoSrc(t, tmpDir, src)

		e := newTestVMCLI(t)
		e.runProgWithTimeout(t, 10*time.Second,
			"loadgo "+filename,
			"break 8",
			"run main 3 5",
			"run",
		)

		e.checkNextLine(t, "READY: loaded \\d* instructions")
		e.checkNextLine(t, "breakpoint added at instruction 8")
		e.checkNextLine(t, "at breakpoint 8 (PUSH5)*")
		e.checkStack(t, 13)
	})
	t.Run("contract with init", func(t *testing.T) {
		src := `package kek
		var I = 5
		func Main(a, b int) int {
			var c = a + b
			return c + I
		}`

		tmpDir := t.TempDir()
		filename := prepareLoadgoSrc(t, tmpDir, src)

		e := newTestVMCLI(t)
		e.runProgWithTimeout(t, 10*time.Second,
			"loadgo "+filename,
			"break 10",
			"run main 3 5",
			"run",
		)

		e.checkNextLine(t, "READY: loaded \\d* instructions")
		e.checkNextLine(t, "breakpoint added at instruction 10")
		e.checkNextLine(t, "at breakpoint 10 (ADD)*")
		e.checkStack(t, 13)
	})
}

// prepareLoadgoSrc prepares provided SC source file for loading into VM via `loadgo` command.
func prepareLoadgoSrc(t *testing.T, tmpDir, src string) string {
	filename := filepath.Join(tmpDir, "vmtestcontract.go")
	require.NoError(t, os.WriteFile(filename, []byte(src), os.ModePerm))
	filename = "'" + filename + "'"
	wd, err := os.Getwd()
	require.NoError(t, err)
	goMod := []byte(`module test.example/kek
require (
	github.com/nspcc-dev/neo-go/pkg/interop v0.0.0
)
replace github.com/nspcc-dev/neo-go/pkg/interop => ` + filepath.Join(wd, "../../pkg/interop") + `
go 1.19`)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, os.ModePerm))
	return filename
}

// prepareLoadnefSrc compiles provided SC source and prepares NEF and manifest for loading into VM
// via `loadnef` command. It returns the name of manifest and NEF files ready to be used in CLI
// commands.
func prepareLoadnefSrc(t *testing.T, tmpDir, src string) (string, string) {
	nefFile, di, err := compiler.CompileWithOptions("test.go", strings.NewReader(src), nil)
	require.NoError(t, err)
	filename := filepath.Join(tmpDir, "vmtestcontract.nef")
	rawNef, err := nefFile.Bytes()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filename, rawNef, os.ModePerm))
	m, err := di.ConvertToManifest(&compiler.Options{})
	require.NoError(t, err)
	manifestFile := filepath.Join(tmpDir, "vmtestcontract.manifest.json")
	rawManifest, err := json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestFile, rawManifest, os.ModePerm))

	manifestFile = "'" + manifestFile + "'"
	filename = "'" + filename + "'"

	return manifestFile, filename
}

func TestLoad(t *testing.T) {
	script := []byte{byte(opcode.PUSH3), byte(opcode.PUSH4), byte(opcode.ADD)}

	ownerAddress := "NbrUYaZgyhSkNoRo9ugRyEMdUZxrhkNaWB"
	ownerAcc, err := address.StringToUint160(ownerAddress)
	require.NoError(t, err)
	sideAcc := util.Uint160{1, 2, 3}
	buff := io.NewBufBinWriter()
	emit.Bytes(buff.BinWriter, ownerAcc.BytesBE())
	emit.Syscall(buff.BinWriter, interopnames.SystemRuntimeCheckWitness)
	checkWitnessScript := buff.Bytes()

	t.Run("loadhex", func(t *testing.T) {
		e := newTestVMCLI(t)
		e.runProg(t,
			"loadhex",
			"loadhex notahex",
			"loadhex "+hex.EncodeToString(script),
			"loadhex "+hex.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator,
			"loadhex "+hex.EncodeToString(checkWitnessScript)+" "+"not-a-separator",
			"loadhex "+hex.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+"not-a-signer",
			"loadhex "+hex.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+ownerAddress, // owner:DefaultScope => true
			"run",
			"loadhex "+hex.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+ownerAddress+":None", // owner:None => false
			"run",
			"loadhex "+hex.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+ownerAcc.StringLE(), // ownerLE:DefaultScope => true
			"run",
			"loadhex "+hex.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+"0x"+ownerAcc.StringLE(), // owner0xLE:DefaultScope => true
			"run",
			"loadhex "+hex.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+sideAcc.StringLE(), // sideLE:DefaultScope => false
			"run",
		)

		e.checkError(t, ErrMissingParameter)
		e.checkError(t, ErrInvalidParameter)
		e.checkNextLine(t, "READY: loaded 3 instructions")
		e.checkError(t, ErrInvalidParameter)
		e.checkError(t, ErrInvalidParameter)
		e.checkError(t, ErrInvalidParameter)
		e.checkNextLine(t, "READY: loaded \\d+ instructions")
		e.checkStack(t, true)
		e.checkNextLine(t, "READY: loaded \\d+ instructions")
		e.checkStack(t, false)
		e.checkNextLine(t, "READY: loaded \\d+ instructions")
		e.checkStack(t, true)
		e.checkNextLine(t, "READY: loaded \\d+ instructions")
		e.checkStack(t, true)
		e.checkNextLine(t, "READY: loaded \\d+ instructions")
		e.checkStack(t, false)
	})
	t.Run("loadbase64", func(t *testing.T) {
		e := newTestVMCLI(t)
		e.runProg(t,
			"loadbase64",
			"loadbase64 not_a_base64",
			"loadbase64 "+base64.StdEncoding.EncodeToString(script),
			"loadbase64 "+base64.StdEncoding.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator,
			"loadbase64 "+base64.StdEncoding.EncodeToString(checkWitnessScript)+" "+"not-a-separator",
			"loadbase64 "+base64.StdEncoding.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" not-a-signer",
			"loadbase64 "+base64.StdEncoding.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+ownerAddress, // owner:DefaultScope => true
			"run",
			"loadbase64 "+base64.StdEncoding.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+ownerAddress+":None", // owner:None => false
			"run",
			"loadbase64 "+base64.StdEncoding.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+ownerAcc.StringLE(), // ownerLE:DefaultScope => true
			"run",
			"loadbase64 "+base64.StdEncoding.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+"0x"+ownerAcc.StringLE(), // owner0xLE:DefaultScope => true
			"run",
			"loadbase64 "+base64.StdEncoding.EncodeToString(checkWitnessScript)+" "+cmdargs.CosignersSeparator+" "+sideAcc.StringLE(), // sideLE:DefaultScope => false
			"run",
		)

		e.checkError(t, ErrMissingParameter)
		e.checkError(t, ErrInvalidParameter)
		e.checkNextLine(t, "READY: loaded 3 instructions")
		e.checkError(t, ErrInvalidParameter)
		e.checkError(t, ErrInvalidParameter)
		e.checkError(t, ErrInvalidParameter)
		e.checkNextLine(t, "READY: loaded \\d+ instructions")
		e.checkStack(t, true)
		e.checkNextLine(t, "READY: loaded \\d+ instructions")
		e.checkStack(t, false)
		e.checkNextLine(t, "READY: loaded \\d+ instructions")
		e.checkStack(t, true)
		e.checkNextLine(t, "READY: loaded \\d+ instructions")
		e.checkStack(t, true)
		e.checkNextLine(t, "READY: loaded \\d+ instructions")
		e.checkStack(t, false)
	})

	src := `package kek
	func Main(op string, a, b int) int {
		if op == "add" {
			return a + b
		} else {
			return a * b
		}
	}`

	t.Run("loadgo", func(t *testing.T) {
		tmpDir := t.TempDir()

		checkLoadgo := func(t *testing.T, cName, cErrName string) {
			filename := prepareLoadgoSrc(t, tmpDir, src)
			filenameErr := filepath.Join(tmpDir, cErrName)

			e := newTestVMCLI(t)
			e.runProgWithTimeout(t, 10*time.Second,
				"loadgo",
				"loadgo "+filenameErr,
				"loadgo "+filename,
				"run main add 3 5")

			e.checkError(t, ErrMissingParameter)
			e.checkNextLine(t, "Error:")
			e.checkNextLine(t, "READY: loaded \\d* instructions")
			e.checkStack(t, 8)
		}

		t.Run("simple", func(t *testing.T) {
			checkLoadgo(t, "vmtestcontract.go", "vmtestcontract_err.go")
		})
		t.Run("utf-8 with spaces", func(t *testing.T) {
			checkLoadgo(t, "тестовый контракт.go", "тестовый контракт с ошибкой.go")
		})

		t.Run("check calling flags", func(t *testing.T) {
			srcAllowNotify := `package kek
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		func Main() int {
			runtime.Log("Hello, world!")
			return 1
		}
`
			filename := prepareLoadgoSrc(t, tmpDir, srcAllowNotify)

			e := newTestVMCLI(t)
			e.runProg(t,
				"loadgo "+filename,
				"run main")
			e.checkNextLine(t, "READY: loaded \\d* instructions")
			e.checkStack(t, 1)
		})
		t.Run("check signers", func(t *testing.T) {
			srcCheckWitness := `package kek
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
			"github.com/nspcc-dev/neo-go/pkg/interop/lib/address"
		)
		func Main() bool {
			var owner = address.ToHash160("` + ownerAddress + `")
			return runtime.CheckWitness(owner)
		}
`
			filename := prepareLoadgoSrc(t, tmpDir, srcCheckWitness)
			t.Run("invalid", func(t *testing.T) {
				e := newTestVMCLI(t)
				e.runProg(t,
					"loadgo "+filename+" "+cmdargs.CosignersSeparator,
					"loadgo "+filename+" "+"not-a-separator",
					"loadgo "+filename+" "+cmdargs.CosignersSeparator+" not-a-signer",
				)
				e.checkError(t, ErrInvalidParameter)
				e.checkError(t, ErrInvalidParameter)
				e.checkError(t, ErrInvalidParameter)
			})
			t.Run("address", func(t *testing.T) {
				e := newTestVMCLI(t)
				e.runProg(t,
					"loadgo "+filename+" "+cmdargs.CosignersSeparator+" "+ownerAddress, // owner:DefaultScope => true
					"run main",
					"loadgo "+filename+" "+cmdargs.CosignersSeparator+" "+ownerAddress+":None", // owner:None => false
					"run main")
				e.checkNextLine(t, "READY: loaded \\d+ instructions")
				e.checkStack(t, true)
				e.checkNextLine(t, "READY: loaded \\d+ instructions")
				e.checkStack(t, false)
			})
			t.Run("string LE", func(t *testing.T) {
				e := newTestVMCLI(t)
				e.runProg(t,
					"loadgo "+filename+" "+cmdargs.CosignersSeparator+" "+ownerAcc.StringLE(), // ownerLE:DefaultScope => true
					"run main",
					"loadgo "+filename+" "+cmdargs.CosignersSeparator+" "+"0x"+ownerAcc.StringLE(), // owner0xLE:DefaultScope => true
					"run main")
				e.checkNextLine(t, "READY: loaded \\d+ instructions")
				e.checkStack(t, true)
				e.checkNextLine(t, "READY: loaded \\d+ instructions")
				e.checkStack(t, true)
			})
			t.Run("nonwitnessed signer", func(t *testing.T) {
				e := newTestVMCLI(t)
				e.runProg(t,
					"loadgo "+filename+" "+cmdargs.CosignersSeparator+" "+sideAcc.StringLE(), // sideLE:DefaultScope => false
					"run main")
				e.checkNextLine(t, "READY: loaded \\d+ instructions")
				e.checkStack(t, false)
			})
		})
	})

	t.Run("loadnef", func(t *testing.T) {
		tmpDir := t.TempDir()

		manifestFile, nefFile := prepareLoadnefSrc(t, tmpDir, src)
		filenameErr := filepath.Join(tmpDir, "vmtestcontract_err.nef")
		require.NoError(t, os.WriteFile(filenameErr, []byte{1, 2, 3, 4}, os.ModePerm))
		notExists := filepath.Join(tmpDir, "notexists.json")
		filenameErr = "'" + filenameErr + "'"

		e := newTestVMCLI(t)
		e.runProg(t,
			"loadnef",
			"loadnef "+filenameErr+" "+manifestFile,
			"loadnef "+nefFile+" "+notExists,
			"loadnef "+nefFile+" "+nefFile,
			"loadnef "+nefFile+" "+manifestFile,
			"run main add 3 5",
			"loadnef "+nefFile,
			"run main add 3 5",
			"loadnef "+nefFile+" "+cmdargs.CosignersSeparator,
			"loadnef "+nefFile+" "+manifestFile+" "+cmdargs.CosignersSeparator,
			"loadnef "+nefFile+" "+manifestFile+" "+"not-a-separator",
			"loadnef "+nefFile+" "+cmdargs.CosignersSeparator+" "+util.Uint160{1, 2, 3}.StringLE(),
			"run main add 3 5",
			"loadnef "+nefFile+" "+manifestFile+" "+cmdargs.CosignersSeparator+" "+util.Uint160{1, 2, 3}.StringLE(),
			"run main add 3 5",
		)

		e.checkError(t, ErrMissingParameter)
		e.checkNextLine(t, "Error:")
		e.checkNextLine(t, "Error:")
		e.checkNextLine(t, "Error:")
		e.checkNextLine(t, "READY: loaded \\d* instructions")
		e.checkStack(t, 8)
		e.checkNextLine(t, "READY: loaded \\d* instructions")
		e.checkStack(t, 8)
		e.checkNextLine(t, "Error:")                          // manifest missing, missing signer after --
		e.checkNextLine(t, "Error:")                          // manifest present, missing signer after --
		e.checkNextLine(t, "Error:")                          // manifest present, invalid separator
		e.checkNextLine(t, "READY: loaded \\d* instructions") // manifest missing, signer present, OK
		e.checkStack(t, 8)
		e.checkNextLine(t, "READY: loaded \\d* instructions") // manifest present, signer present, OK
		e.checkStack(t, 8)
	})
}

func TestLoad_RunWithCALLT(t *testing.T) {
	// Our smart compiler will generate CALLT instruction for the following StdLib call:
	src := `package kek
		import "github.com/nspcc-dev/neo-go/pkg/interop/native/std"
		func Main() int {
			return std.Atoi("123", 10)
		}`

	t.Run("loadgo", func(t *testing.T) {
		tmp := t.TempDir()
		filename := prepareLoadgoSrc(t, tmp, src)
		e := newTestVMCLI(t)
		e.runProg(t,
			"loadgo "+filename,
			"run main",
		)
		e.checkNextLine(t, "READY: loaded \\d* instructions")
		e.checkStack(t, 123)
	})

	t.Run("loadnef", func(t *testing.T) {
		tmpDir := t.TempDir()
		manifestFile, nefFile := prepareLoadnefSrc(t, tmpDir, src)

		e := newTestVMCLI(t)
		e.runProg(t,
			"loadnef "+nefFile+" "+manifestFile,
			"run main",
		)
		e.checkNextLine(t, "READY: loaded \\d* instructions")
		e.checkStack(t, 123)
	})

	t.Run("loaddeployed", func(t *testing.T) {
		// We'll use `Runtime example` example contract which has a call to native Management
		// inside performed via CALLT instruction (`destroy` method).
		e := newTestVMClIWithState(t)
		var (
			cH    util.Uint160
			cName = "Runtime example"
			bc    = e.cli.chain
		)
		for i := int32(1); ; i++ {
			h, err := bc.GetContractScriptHash(i)
			if err != nil {
				break
			}
			cs := bc.GetContractState(h)
			if cs == nil {
				break
			}
			if cs.Manifest.Name == cName {
				cH = cs.Hash
				break
			}
		}
		require.NotEmpty(t, cH, fmt.Sprintf("failed to locate `%s` example contract with CALLT usage in the simple chain", cName))
		e.runProg(t,
			"loaddeployed "+cH.StringLE()+" -- NbrUYaZgyhSkNoRo9ugRyEMdUZxrhkNaWB:Global", // the contract's owner got from the contract's code.
			"run destroy",
		)
		e.checkNextLine(t, "READY: loaded \\d* instructions")
		e.checkStack(t) // Nothing on stack, successful execution.
	})
}

func TestRunWithDifferentArguments(t *testing.T) {
	src := `package kek
	var a = 1
	func init() {
		a += 1
	}
	func InitHasRun() bool {
		return a == 2
	}
	func Negate(arg bool) bool {
		return !arg
	}
	func GetInt(arg int) int {
		return arg
	}
	func GetString(arg string) string {
		return arg
	}
	func GetArr(arg []any) []any{
		return arg
	}`

	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "run_vmtestcontract.go")
	require.NoError(t, os.WriteFile(filename, []byte(src), os.ModePerm))

	filename = "'" + filename + "'"
	e := newTestVMCLI(t)
	e.runProgWithTimeout(t, 30*time.Second,
		"loadgo "+filename, "run notexists",
		"loadgo "+filename, "run negate false",
		"loadgo "+filename, "run negate true",
		"loadgo "+filename, "run negate bool:invalid",
		"loadgo "+filename, "run getInt 123",
		"loadgo "+filename, "run getInt int:invalid",
		"loadgo "+filename, "run getString validstring",
		"loadgo "+filename, "run initHasRun",
		"loadhex "+hex.EncodeToString([]byte{byte(opcode.ADD)}),
		"run _ 1 2",
		"loadbase64 "+base64.StdEncoding.EncodeToString([]byte{byte(opcode.MUL)}),
		"run _ 21 2",
		"loadgo "+filename, "run getArr [ 1 2 3 ]",
	)

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkNextLine(t, "Error:")

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkStack(t, true)

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkStack(t, false)

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkError(t, ErrInvalidParameter)

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkStack(t, 123)

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkError(t, ErrInvalidParameter)

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkStack(t, "validstring")

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkStack(t, true)

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkStack(t, 3)

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkStack(t, 42)

	e.checkNextLine(t, "READY: loaded \\d.* instructions")
	e.checkStack(t, []stackitem.Item{
		stackitem.NewBigInteger(big.NewInt(1)),
		stackitem.NewBigInteger(big.NewInt(2)),
		stackitem.NewBigInteger(big.NewInt(3)),
	})
}

func TestPrintOps(t *testing.T) {
	w := io.NewBufBinWriter()
	emit.String(w.BinWriter, "log")
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeLog)
	emit.Instruction(w.BinWriter, opcode.PUSHDATA1, []byte{3, 1, 2, 3})
	script := w.Bytes()
	e := newTestVMCLI(t)
	e.runProg(t,
		"ops",
		"loadhex "+hex.EncodeToString(script),
		"ops")

	e.checkNextLine(t, ".*no program loaded")
	e.checkNextLine(t, fmt.Sprintf("READY: loaded %d instructions", len(script)))
	e.checkNextLine(t, "INDEX.*OPCODE.*PARAMETER")
	e.checkNextLine(t, "0.*PUSHDATA1.*6c6f67")
	e.checkNextLine(t, "5.*SYSCALL.*System\\.Runtime\\.Log")
	e.checkNextLine(t, "10.*PUSHDATA1.*010203")
}

func TestLoadAbort(t *testing.T) {
	e := newTestVMCLI(t)
	e.runProg(t,
		"loadhex "+hex.EncodeToString([]byte{byte(opcode.PUSH1), byte(opcode.ABORT)}),
		"run",
	)

	e.checkNextLine(t, "READY: loaded 2 instructions")
	e.checkNextLine(t, "Error:.*at instruction 1.*ABORT")
}

func TestBreakpoint(t *testing.T) {
	w := io.NewBufBinWriter()
	emit.Opcodes(w.BinWriter, opcode.PUSH1, opcode.PUSH2, opcode.ADD, opcode.PUSH6, opcode.ADD)
	e := newTestVMCLI(t)
	e.runProg(t,
		"break 3",
		"cont",
		"ip",
		"loadhex "+hex.EncodeToString(w.Bytes()),
		"break",
		"break second",
		"break 2",
		"break 4",
		"cont", "estack",
		"run", "estack",
		"cont",
	)

	e.checkNextLine(t, "no program loaded")
	e.checkNextLine(t, "no program loaded")
	e.checkNextLine(t, "no program loaded")
	e.checkNextLine(t, "READY: loaded 5 instructions")
	e.checkError(t, ErrMissingParameter)
	e.checkError(t, ErrInvalidParameter)
	e.checkNextLine(t, "breakpoint added at instruction 2")
	e.checkNextLine(t, "breakpoint added at instruction 4")

	e.checkNextLine(t, "at breakpoint 2.*ADD")
	e.checkStack(t, 1, 2)

	e.checkNextLine(t, "at breakpoint 4.*ADD")
	e.checkStack(t, 3, 6)

	e.checkStack(t, 9)
}

func TestDumpSSlot(t *testing.T) {
	w := io.NewBufBinWriter()
	emit.Opcodes(w.BinWriter, opcode.INITSSLOT, 2, // init static slot with size=2
		opcode.PUSH5, opcode.STSFLD, 1, // put `int(5)` to sslot[1]; sslot[0] is nil
		opcode.LDSFLD1) // put sslot[1] to the top of estack
	e := newTestVMCLI(t)
	e.runProg(t,
		"loadhex "+hex.EncodeToString(w.Bytes()),
		"break 5",
		"step", "sslot",
		"cont", "estack",
	)
	e.checkNextLine(t, "READY: loaded 6 instructions")
	e.checkNextLine(t, "breakpoint added at instruction 5")

	e.checkNextLine(t, "at breakpoint 5.*LDSFLD1")
	e.checkSlot(t, nil, 5)

	e.checkStack(t, 5)
}

func TestDumpLSlot_DumpASlot(t *testing.T) {
	w := io.NewBufBinWriter()
	emit.Opcodes(w.BinWriter, opcode.PUSH4, opcode.PUSH5, opcode.PUSH6, // items for args slot
		opcode.INITSLOT, 2, 3, // init local slot with size=2 and args slot with size 3
		opcode.PUSH7, opcode.STLOC1, // put `int(7)` to lslot[1]; lslot[0] is nil
		opcode.LDLOC, 1) // put lslot[1] to the top of estack
	e := newTestVMCLI(t)
	e.runProg(t,
		"loadhex "+hex.EncodeToString(w.Bytes()),
		"break 6",
		"break 8",
		"cont", "aslot",
		"cont", "lslot",
		"cont", "estack",
	)
	e.checkNextLine(t, "READY: loaded 10 instructions")
	e.checkNextLine(t, "breakpoint added at instruction 6")
	e.checkNextLine(t, "breakpoint added at instruction 8")

	e.checkNextLine(t, "at breakpoint 6.*PUSH7")
	e.checkSlot(t, 6, 5, 4) // args slot

	e.checkNextLine(t, "at breakpoint 8.*LDLOC")
	e.checkSlot(t, nil, 7) // local slot

	e.checkStack(t, 7)
}

func TestStep(t *testing.T) {
	script := hex.EncodeToString([]byte{
		byte(opcode.PUSH0), byte(opcode.PUSH1), byte(opcode.PUSH2), byte(opcode.PUSH3),
	})
	e := newTestVMCLI(t)
	e.runProg(t,
		"step",
		"loadhex "+script,
		"step invalid",
		"step",
		"step 2",
		"ip", "step", "ip")

	e.checkNextLine(t, "no program loaded")
	e.checkNextLine(t, "READY: loaded \\d+ instructions")
	e.checkError(t, ErrInvalidParameter)
	e.checkNextLine(t, "at breakpoint 1.*PUSH1")
	e.checkNextLine(t, "at breakpoint 3.*PUSH3")
	e.checkNextLine(t, "instruction pointer at 3.*PUSH3")
	e.checkNextLine(t, "execution has finished")
	e.checkNextLine(t, "execution has finished")
}

func TestErrorOnStepInto(t *testing.T) {
	script := hex.EncodeToString([]byte{byte(opcode.ADD)})
	e := newTestVMCLI(t)
	e.runProg(t,
		"stepover",
		"loadhex "+script,
		"stepover")

	e.checkNextLine(t, "Error:.*no program loaded")
	e.checkNextLine(t, "READY: loaded 1 instructions")
	e.checkNextLine(t, "Error:")
}

func TestStepIntoOverOut(t *testing.T) {
	script := hex.EncodeToString([]byte{
		byte(opcode.PUSH2), byte(opcode.CALL), 4, byte(opcode.NOP), byte(opcode.RET),
		byte(opcode.PUSH3), byte(opcode.ADD), byte(opcode.RET),
	})

	e := newTestVMCLI(t)
	e.runProg(t,
		"loadhex "+script,
		"step", "stepover", "run",
		"loadhex "+script,
		"step", "stepinto", "step", "estack", "run",
		"loadhex "+script,
		"step", "stepinto", "stepout", "run")

	e.checkNextLine(t, "READY: loaded 8 instructions")
	e.checkNextLine(t, "at breakpoint 1.*CALL")
	e.checkNextLine(t, "instruction pointer at.*NOP")
	e.checkStack(t, 5)

	e.checkNextLine(t, "READY: loaded 8 instructions")
	e.checkNextLine(t, "at breakpoint.*CALL")
	e.checkNextLine(t, "instruction pointer at.*PUSH3")
	e.checkNextLine(t, "at breakpoint.*ADD")
	e.checkStack(t, 2, 3)
	e.checkStack(t, 5)

	e.checkNextLine(t, "READY: loaded 8 instructions")
	e.checkNextLine(t, "at breakpoint 1.*CALL")
	e.checkNextLine(t, "instruction pointer at.*PUSH3")
	e.checkNextLine(t, "instruction pointer at.*NOP")
	e.checkStack(t, 5)
}

// `Parse` output is written via `tabwriter` so if any problems
// are encountered in this test, try to replace ' ' with '\\s+'.
func TestParse(t *testing.T) {
	t.Run("Integer", func(t *testing.T) {
		e := newTestVMCLI(t)
		e.runProg(t,
			"parse",
			"parse 6667")

		e.checkError(t, ErrMissingParameter)
		e.checkNextLine(t, "Integer to Hex.*0b1a")
		e.checkNextLine(t, "Integer to Base64.*Cxo=")
		e.checkNextLine(t, "Hex to String.*\"fg\"")
		e.checkNextLine(t, "Hex to Integer.*26470")
		e.checkNextLine(t, "Swap Endianness.*6766")
		e.checkNextLine(t, "Base64 to String.*\"뮻\"")
		e.checkNextLine(t, "Base64 to BigInteger.*-4477205")
		e.checkNextLine(t, "String to Hex.*36363637")
		e.checkNextLine(t, "String to Base64.*NjY2Nw==")
	})
	t.Run("Address", func(t *testing.T) {
		e := newTestVMCLI(t)
		e.runProg(t, "parse "+"NbTiM6h8r99kpRtb428XcsUk1TzKed2gTc")
		e.checkNextLine(t, "Address to BE ScriptHash.*aa8acf859d4fe402b34e673f2156821796a488eb")
		e.checkNextLine(t, "Address to LE ScriptHash.*eb88a496178256213f674eb302e44f9d85cf8aaa")
		e.checkNextLine(t, "Address to Base64.*(BE).*qorPhZ1P5AKzTmc/IVaCF5akiOs=")
		e.checkNextLine(t, "Address to Base64.*(LE).*64iklheCViE/Z06zAuRPnYXPiqo=")
		e.checkNextLine(t, "String to Hex.*4e6254694d3668387239396b70527462343238586373556b31547a4b656432675463")
		e.checkNextLine(t, "String to Base64.*TmJUaU02aDhyOTlrcFJ0YjQyOFhjc1VrMVR6S2VkMmdUYw==")
	})
	t.Run("Uint160", func(t *testing.T) {
		u := util.Uint160{66, 67, 68}
		e := newTestVMCLI(t)
		e.runProg(t, "parse "+u.StringLE())
		e.checkNextLine(t, "Integer to Hex.*b6c706")
		e.checkNextLine(t, "Integer to Base64.*tscG")
		e.checkNextLine(t, "BE ScriptHash to Address.*NKuyBkoGdZZSLyPbJEetheRhQKhATAzN2A")
		e.checkNextLine(t, "LE ScriptHash to Address.*NRxLN7apYwKJihzMt4eSSnU9BJ77dp2TNj")
		e.checkNextLine(t, "Hex to String")
		e.checkNextLine(t, "Hex to Integer.*378293464438118320046642359484100328446970822656")
		e.checkNextLine(t, "Swap Endianness.*4243440000000000000000000000000000000000")
		e.checkNextLine(t, "Base64 to String.*")
		e.checkNextLine(t, "Base64 to BigInteger.*376115185060690908522683414825349447309891933036899526770189324554358227")
		e.checkNextLine(t, "String to Hex.*30303030303030303030303030303030303030303030303030303030303030303030343434333432")
		e.checkNextLine(t, "String to Base64.*MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDQ0NDM0Mg==")
	})
	t.Run("public key", func(t *testing.T) {
		pub := "02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2"
		e := newTestVMCLI(t)
		e.runProg(t, "parse "+pub)
		e.checkNextLine(t, "Public key to BE ScriptHash.*ee9ea22c27e34bd0148fc4108e08f74e8f5048b2")
		e.checkNextLine(t, "Public key to LE ScriptHash.*b248508f4ef7088e10c48f14d04be3272ca29eee")
		e.checkNextLine(t, "Public key to Address.*Nhfg3TbpwogLvDGVvAvqyThbsHgoSUKwtn")
		e.checkNextLine(t, "Hex to String")
		e.checkNextLine(t, "Hex to Integer.*-7115107707948693452214836319400158580475150561081357074343221218306172781415678")
		e.checkNextLine(t, "Swap Endianness.*c28d7fbfc4bb74d7a76f0496b87d6b203f754c5fed8ac517e3df7b01f42b62b302")
		e.checkNextLine(t, "String to Hex.*303262333632326266343031376264666533313763353861656435663463373533663230366237646238393630343666613764373734626263346266376638646332")
		e.checkNextLine(t, "String to Base64.*MDJiMzYyMmJmNDAxN2JkZmUzMTdjNThhZWQ1ZjRjNzUzZjIwNmI3ZGI4OTYwNDZmYTdkNzc0YmJjNGJmN2Y4ZGMy")
	})
	t.Run("base64", func(t *testing.T) {
		e := newTestVMCLI(t)
		u := random.Uint160()
		e.runProg(t, "parse "+base64.StdEncoding.EncodeToString(u.BytesBE()))
		e.checkNextLine(t, "Base64 to String\\s+")
		e.checkNextLine(t, "Base64 to BigInteger\\s+")
		e.checkNextLine(t, "Base64 to BE ScriptHash\\s+"+u.StringBE())
		e.checkNextLine(t, "Base64 to LE ScriptHash\\s+"+u.StringLE())
		e.checkNextLine(t, "Base64 to Address \\(BE\\)\\s+"+address.Uint160ToString(u))
		e.checkNextLine(t, "Base64 to Address \\(LE\\)\\s+"+address.Uint160ToString(u.Reverse()))
		e.checkNextLine(t, "String to Hex\\s+")
		e.checkNextLine(t, "String to Base64\\s+")
	})
}

func TestPrintLogo(t *testing.T) {
	e := newTestVMCLIWithLogo(t, true)
	e.runProg(t)
	require.True(t, strings.HasPrefix(e.out.String(), logo))
	require.False(t, e.exit.Load())
}

func TestExit(t *testing.T) {
	e := newTestVMCLI(t)
	e.runProg(t, "exit")
	require.True(t, e.exit.Load())
}

func TestReset(t *testing.T) {
	script := []byte{byte(opcode.PUSH1)}
	e := newTestVMCLI(t)
	e.runProg(t,
		"loadhex "+hex.EncodeToString(script),
		"ops",
		"reset",
		"ops")

	e.checkNextLine(t, "READY: loaded 1 instructions")
	e.checkNextLine(t, "INDEX.*OPCODE.*PARAMETER")
	e.checkNextLine(t, "0.*PUSH1.*")
	e.checkNextLine(t, "")
	e.checkError(t, fmt.Errorf("VM is not ready: no program loaded"))
}

func TestRunWithState(t *testing.T) {
	e := newTestVMClIWithState(t)

	// Ensure that state is properly loaded and on-chain contract can be called.
	script := io.NewBufBinWriter()
	h, err := e.cli.chain.GetContractScriptHash(1) // examples/storage/storage.go
	require.NoError(t, err)
	emit.AppCall(script.BinWriter, h, "put", callflag.All, 3, 3)
	e.runProg(t,
		"loadhex "+hex.EncodeToString(script.Bytes()),
		"run",
		"exit")
	e.checkNextLine(t, "READY: loaded 37 instructions")
	e.checkStack(t, 3)
}

func TestRunWithHistoricState(t *testing.T) {
	e := newTestVMClIWithState(t)

	script := io.NewBufBinWriter()
	h, err := e.cli.chain.GetContractScriptHash(1) // examples/storage/storage.go
	require.NoError(t, err)
	emit.AppCall(script.BinWriter, h, "get", callflag.All, 1)
	b := script.Bytes()

	e.runProg(t,
		"loadhex "+hex.EncodeToString(b), // normal invocation
		"run",
		"loadhex --historic 3 "+hex.EncodeToString(b), // historic invocation, old value should be retrieved
		"run",
		"loadhex --historic 0 "+hex.EncodeToString(b), // historic invocation, contract is not deployed yet
		"run",
		"exit",
	)
	e.checkNextLine(t, "READY: loaded 36 instructions")
	e.checkStack(t, []byte{2})
	e.checkNextLine(t, "READY: loaded 36 instructions")
	e.checkStack(t, []byte{1})
	e.checkNextLine(t, "READY: loaded 36 instructions")
	e.checkNextLineExact(t, "Error: at instruction 31 (SYSCALL): System.Contract.Call failed: called contract 0f825b050eb8ce9eaa82993e90615025ab798016 not found: key not found\n")
}

func TestEvents(t *testing.T) {
	e := newTestVMClIWithState(t)

	script := io.NewBufBinWriter()
	h, err := e.cli.chain.GetContractScriptHash(2) // examples/runtime/runtime.go
	require.NoError(t, err)
	emit.AppCall(script.BinWriter, h, "notify", callflag.All, []any{true, 5})
	e.runProg(t,
		"loadhex "+hex.EncodeToString(script.Bytes()),
		"run",
		"events",
		"exit")
	expectedEvent := state.NotificationEvent{
		ScriptHash: h,
		Name:       "Event",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewArray([]stackitem.Item{
				stackitem.Make(true),
				stackitem.Make(5),
			}),
		}),
	}
	e.checkNextLine(t, "READY: loaded 42 instructions")
	e.checkStack(t, stackitem.Null{})
	e.checkEvents(t, true, expectedEvent)  // automatically printed after `run` command
	e.checkEvents(t, false, expectedEvent) // printed after `events` command
}

func TestEnv(t *testing.T) {
	t.Run("default setup", func(t *testing.T) {
		e := newTestVMCLI(t)
		e.runProg(t, "env")
		e.checkNextLine(t, "Chain height: 0")
		e.checkNextLineExact(t, "VM height (may differ from chain height in case of historic call): 0\n")
		e.checkNextLine(t, "Network magic: 42")
		e.checkNextLine(t, "DB type: inmemory")
	})
	t.Run("setup with state", func(t *testing.T) {
		e := newTestVMClIWithState(t)
		e.runProg(t, "env", "exit")
		e.checkNextLine(t, "Chain height: 5")
		e.checkNextLineExact(t, "VM height (may differ from chain height in case of historic call): 5\n")
		e.checkNextLine(t, "Network magic: 42")
		e.checkNextLine(t, "DB type: leveldb")
	})
	t.Run("setup with historic state", func(t *testing.T) {
		e := newTestVMClIWithState(t)
		e.runProg(t, "loadbase64 --historic 3 "+base64.StdEncoding.EncodeToString([]byte{byte(opcode.PUSH1)}),
			"env", "exit")
		e.checkNextLine(t, "READY: loaded 1 instructions")
		e.checkNextLine(t, "Chain height: 5")
		e.checkNextLineExact(t, "VM height (may differ from chain height in case of historic call): 3\n")
		e.checkNextLine(t, "Network magic: 42")
		e.checkNextLine(t, "DB type: leveldb")
	})
	t.Run("verbose", func(t *testing.T) {
		e := newTestVMClIWithState(t)
		e.runProg(t, "env -v", "exit")
		e.checkNextLine(t, "Chain height: 5")
		e.checkNextLineExact(t, "VM height (may differ from chain height in case of historic call): 5\n")
		e.checkNextLine(t, "Network magic: 42")
		e.checkNextLine(t, "DB type: leveldb")
		e.checkNextLine(t, "Node config:") // Do not check exact node config.
	})
}

func TestDumpStorage(t *testing.T) {
	e := newTestVMClIWithState(t)

	h, err := e.cli.chain.GetContractScriptHash(1) // examples/storage/storage.go
	require.NoError(t, err)
	expected := []storage.KeyValue{
		{Key: []byte{1}, Value: []byte{2}},
		{Key: []byte{2}, Value: []byte{2}},
	}
	e.runProg(t,
		"storage "+h.StringLE(),
		"storage 0x"+h.StringLE(),
		"storage "+address.Uint160ToString(h),
		"storage 1",
		"storage 1 "+hex.EncodeToString(expected[0].Key),
		"storage 1 --backwards",
		"exit",
	)
	e.checkStorage(t, expected...)
	e.checkStorage(t, expected...)
	e.checkStorage(t, expected...)
	e.checkStorage(t, expected...)
	e.checkStorage(t, storage.KeyValue{Key: nil, Value: []byte{2}}) // empty key because search prefix is trimmed
	e.checkStorage(t, expected[1], expected[0])
}

func TestDumpStorageDiff(t *testing.T) {
	e := newTestVMClIWithState(t)

	script := io.NewBufBinWriter()
	h, err := e.cli.chain.GetContractScriptHash(1) // examples/storage/storage.go
	require.NoError(t, err)
	emit.AppCall(script.BinWriter, h, "put", callflag.All, 3, 3)

	expected := []storage.KeyValue{
		{Key: []byte{1}, Value: []byte{2}},
		{Key: []byte{2}, Value: []byte{2}},
	}
	diff := storage.KeyValue{Key: []byte{3}, Value: []byte{3}}
	e.runProg(t,
		"storage 1",
		"storage 1 --diff",
		"loadhex "+hex.EncodeToString(script.Bytes()),
		"run",
		"storage 1",
		"storage 1 --diff",
		"exit",
	)

	e.checkStorage(t, expected...)
	// no script is executed => no diff
	e.checkNextLine(t, "READY: loaded 37 instructions")
	e.checkStack(t, 3)
	e.checkStorage(t, append(expected, diff)...)
	e.checkStorage(t, diff)
}

func TestDumpChanges(t *testing.T) {
	e := newTestVMClIWithState(t)

	script := io.NewBufBinWriter()
	h, err := e.cli.chain.GetContractScriptHash(1) // examples/storage/storage.go
	require.NoError(t, err)
	emit.AppCall(script.BinWriter, h, "put", callflag.All, 3, 4) // add
	emit.AppCall(script.BinWriter, h, "delete", callflag.All, 1) // remove
	emit.AppCall(script.BinWriter, h, "put", callflag.All, 2, 5) // update

	expected := []storageChange{
		{
			ContractID: 1,
			Operation: dboper.Operation{
				State: "Deleted",
				Key:   []byte{1},
			},
		},
		{
			ContractID: 1,
			Operation: dboper.Operation{
				State: "Changed",
				Key:   []byte{2},
				Value: []byte{5},
			},
		},
		{
			ContractID: 1,
			Operation: dboper.Operation{
				State: "Added",
				Key:   []byte{3},
				Value: []byte{4},
			},
		},
	}
	e.runProg(t,
		"changes",
		"changes 1",
		"loadhex "+hex.EncodeToString(script.Bytes()),
		"run",
		"changes 1 "+hex.EncodeToString([]byte{1}),
		"changes 1 "+hex.EncodeToString([]byte{2}),
		"changes 1 "+hex.EncodeToString([]byte{3}),
		"exit",
	)

	// no script is executed => no diff
	e.checkNextLine(t, "READY: loaded 113 instructions")
	e.checkStack(t, 3, true, 2)
	e.checkChange(t, expected[0])
	e.checkChange(t, expected[1])
	e.checkChange(t, expected[2])
}

func TestLoadtx(t *testing.T) {
	e := newTestVMClIWithState(t)

	b, err := e.cli.chain.GetBlock(e.cli.chain.GetHeaderHash(2)) // Block #2 contains transaction that puts (1,1) pair to storage contract.
	require.NoError(t, err)
	require.Equal(t, 1, len(b.Transactions))
	tx := b.Transactions[0]

	tmp := filepath.Join(t.TempDir(), "tx.json")
	require.NoError(t, paramcontext.InitAndSave(netmode.UnitTestNet, tx, nil, tmp))

	e.runProg(t,
		"loadtx "+tx.Hash().StringLE(), // hash LE
		"run",
		"loadtx --gas 10000 "+tx.Hash().StringLE(), // with GAS
		"run",
		"loadtx 0x"+tx.Hash().StringLE(), //  hash LE with 0x prefix
		"run",
		"loadtx '"+tmp+"'", // Tx from parameter context file.
		"run",
		"loadtx", // missing argument
		"exit",
	)
	e.checkNextLine(t, "READY: loaded \\d+ instructions")
	e.checkStack(t, 1)
	e.checkNextLine(t, "READY: loaded \\d+ instructions")
	e.checkError(t, errors.New("at instruction 3 (PACK): gas limit is exceeded"))
	e.checkNextLine(t, "READY: loaded \\d+ instructions")
	e.checkStack(t, 1)
	e.checkNextLine(t, "READY: loaded \\d+ instructions")
	e.checkStack(t, 1)
	e.checkError(t, errors.New("missing argument: <file-or-hash>"))
}

func TestLoaddeployed(t *testing.T) {
	e := newTestVMClIWithState(t)

	h, err := e.cli.chain.GetContractScriptHash(1) // examples/storage/storage.go
	require.NoError(t, err)
	ownerAddress := "NbrUYaZgyhSkNoRo9ugRyEMdUZxrhkNaWB" // owner of examples/runtime/runtime.go (taken from deployed contract with ID=2)
	ownerAcc, err := address.StringToUint160(ownerAddress)
	require.NoError(t, err)
	sideAcc := util.Uint160{1, 2, 3}

	e.runProg(t,
		"loaddeployed "+h.StringLE(), // hash LE
		"run get 1",
		"loaddeployed --gas 420000 "+h.StringLE(), // gas-limited
		"run get 1",
		"loaddeployed "+"0x"+h.StringLE(), //  hash LE with 0x prefix
		"run get 1",
		"loaddeployed 1", // contract ID
		"run get 1",
		"loaddeployed --historic 2 1", // historic state, check that hash is properly set
		"run get 1",
		// Check signers parsing:
		"loaddeployed 2 "+cmdargs.CosignersSeparator,
		"loaddeployed 2 "+"not-a-separator",
		"loaddeployed 2 "+cmdargs.CosignersSeparator+" not-a-signer",
		"loaddeployed 2 "+cmdargs.CosignersSeparator+" "+ownerAddress, // check witness (owner:DefautScope => true)
		"run checkWitness",
		"loaddeployed 2 "+cmdargs.CosignersSeparator+" "+ownerAddress+":None", // check witness (owner:None => false)
		"run checkWitness",
		"loaddeployed 2 "+cmdargs.CosignersSeparator+" "+ownerAddress+":CalledByEntry", // check witness (owner:CalledByEntry => true)
		"run checkWitness",
		"loaddeployed 2 "+cmdargs.CosignersSeparator+" "+ownerAcc.StringLE()+":CalledByEntry", // check witness (ownerLE:CalledByEntry => true)
		"run checkWitness",
		"loaddeployed 2 "+cmdargs.CosignersSeparator+" "+"0x"+ownerAcc.StringLE()+":CalledByEntry", // check witness (owner0xLE:CalledByEntry => true)
		"run checkWitness",
		"loaddeployed 2 "+cmdargs.CosignersSeparator+" "+sideAcc.StringLE()+":Global", // check witness (sideLE:Global => false)
		"run checkWitness",
		"loaddeployed", // missing argument
		"exit",
	)
	e.checkNextLine(t, "READY: loaded \\d+ instructions")
	e.checkStack(t, []byte{2})
	e.checkNextLine(t, "READY: loaded \\d+ instructions")
	e.checkError(t, errors.New("at instruction 63 (SYSCALL): System.Storage.Get failed: insufficient amount of gas"))
	e.checkNextLine(t, "READY: loaded \\d+ instructions")
	e.checkStack(t, []byte{2})
	e.checkNextLine(t, "READY: loaded \\d+ instructions")
	e.checkStack(t, []byte{2})
	e.checkNextLine(t, "READY: loaded \\d+ instructions")
	e.checkStack(t, []byte{1})
	// Check signers parsing:
	e.checkError(t, ErrInvalidParameter)
	e.checkError(t, ErrInvalidParameter)
	e.checkError(t, ErrInvalidParameter)
	e.checkNextLine(t, "READY: loaded \\d+ instructions") // check witness of owner:DefaultScope
	e.checkStack(t, true)
	e.checkNextLine(t, "READY: loaded \\d+ instructions") // check witness of owner:None
	e.checkStack(t, false)
	e.checkNextLine(t, "READY: loaded \\d+ instructions") // check witness of owner:CalledByEntry
	e.checkStack(t, true)
	e.checkNextLine(t, "READY: loaded \\d+ instructions") // check witness of ownerLE:CalledByEntry
	e.checkStack(t, true)
	e.checkNextLine(t, "READY: loaded \\d+ instructions") // check witness of owner0xLE:CalledByEntry
	e.checkStack(t, true)
	e.checkNextLine(t, "READY: loaded \\d+ instructions") // check witness of owner0xLE:CalledByEntry
	e.checkStack(t, false)
	e.checkError(t, errors.New("contract hash, address or ID is mandatory argument"))
}

func TestJump(t *testing.T) {
	buf := io.NewBufBinWriter()
	emit.Opcodes(buf.BinWriter, opcode.PUSH1, opcode.PUSH2, opcode.ABORT) // some garbage
	jmpTo := buf.Len()
	emit.Opcodes(buf.BinWriter, opcode.PUSH4, opcode.PUSH5, opcode.ADD) // useful script
	e := newTestVMCLI(t)
	e.runProg(t,
		"loadhex "+hex.EncodeToString(buf.Bytes()),
		"jump "+strconv.Itoa(jmpTo),
		"run",
	)

	e.checkNextLine(t, "READY: loaded 6 instructions")
	e.checkNextLine(t, fmt.Sprintf("jumped to instruction %d", jmpTo))
	e.checkStack(t, 9)
}
