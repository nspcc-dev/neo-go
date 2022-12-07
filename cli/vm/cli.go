package vm

import (
	"bytes"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/chzyer/readline"
	"github.com/kballard/go-shellquote"
	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	chainKey            = "chain"
	chainCfgKey         = "chainCfg"
	icKey               = "ic"
	manifestKey         = "manifest"
	exitFuncKey         = "exitFunc"
	readlineInstanceKey = "readlineKey"
	printLogoKey        = "printLogoKey"
)

// Various flag names.
const (
	verboseFlagFullName   = "verbose"
	historicFlagFullName  = "historic"
	gasFlagFullName       = "gas"
	backwardsFlagFullName = "backwards"
	diffFlagFullName      = "diff"
)

var (
	historicFlag = cli.IntFlag{
		Name: historicFlagFullName,
		Usage: "Height for historic script invocation (for MPT-enabled blockchain configuration with KeepOnlyLatestState setting disabled). " +
			"Assuming that block N-th is specified as an argument, the historic invocation is based on the storage state of height N and fake currently-accepting block with index N+1.",
	}
	gasFlag = cli.Int64Flag{
		Name:  gasFlagFullName,
		Usage: "GAS limit for this execution (integer number, satoshi).",
	}
)

var commands = []cli.Command{
	{
		Name:        "exit",
		Usage:       "Exit the VM prompt",
		UsageText:   "exit",
		Description: "Exit the VM prompt.",
		Action:      handleExit,
	},
	{
		Name:        "ip",
		Usage:       "Show current instruction",
		UsageText:   "ip",
		Description: "Show current instruction.",
		Action:      handleIP,
	},
	{
		Name:      "break",
		Usage:     "Place a breakpoint",
		UsageText: `break <ip>`,
		Description: `<ip> is mandatory parameter.

Example:
> break 12`,
		Action: handleBreak,
	},
	{
		Name:      "jump",
		Usage:     "Jump to the specified instruction (absolute IP value)",
		UsageText: `jump <ip>`,
		Description: `<ip> is mandatory parameter (absolute IP value).

Example:
> jump 12`,
		Action: handleJump,
	},
	{
		Name:        "estack",
		Usage:       "Show evaluation stack contents",
		UsageText:   "estack",
		Description: "Show evaluation stack contents.",
		Action:      handleXStack,
	},
	{
		Name:        "istack",
		Usage:       "Show invocation stack contents",
		UsageText:   "istack",
		Description: "Show invocation stack contents.",
		Action:      handleXStack,
	},
	{
		Name:        "sslot",
		Usage:       "Show static slot contents",
		UsageText:   "sslot",
		Description: "Show static slot contents.",
		Action:      handleSlots,
	},
	{
		Name:        "lslot",
		Usage:       "Show local slot contents",
		UsageText:   "lslot",
		Description: "Show local slot contents",
		Action:      handleSlots,
	},
	{
		Name:        "aslot",
		Usage:       "Show arguments slot contents",
		UsageText:   "aslot",
		Description: "Show arguments slot contents.",
		Action:      handleSlots,
	},
	{
		Name:      "loadnef",
		Usage:     "Load a NEF-consistent script into the VM optionally attaching to it provided signers with scopes",
		UsageText: `loadnef [--historic <height>] [--gas <int>] <file> <manifest> [<signer-with-scope>, ...]`,
		Flags:     []cli.Flag{historicFlag, gasFlag},
		Description: `<file> and <manifest> parameters are mandatory.

` + cmdargs.SignersParsingDoc + `

Example:
> loadnef /path/to/script.nef /path/to/manifest.json`,
		Action: handleLoadNEF,
	},
	{
		Name:      "loadbase64",
		Usage:     "Load a base64-encoded script string into the VM optionally attaching to it provided signers with scopes",
		UsageText: `loadbase64 [--historic <height>] [--gas <int>] <string> [<signer-with-scope>, ...]`,
		Flags:     []cli.Flag{historicFlag, gasFlag},
		Description: `<string> is mandatory parameter.

` + cmdargs.SignersParsingDoc + `

Example:
> loadbase64 AwAQpdToAAAADBQV9ehtQR1OrVZVhtHtoUHRfoE+agwUzmFvf3Rhfg/EuAVYOvJgKiON9j8TwAwIdHJhbnNmZXIMFDt9NxHG8Mz5sdypA9G/odiW8SOMQWJ9W1I4`,
		Action: handleLoadBase64,
	},
	{
		Name:      "loadhex",
		Usage:     "Load a hex-encoded script string into the VM optionally attaching to it provided signers with scopes",
		UsageText: `loadhex [--historic <height>] [--gas <int>] <string> [<signer-with-scope>, ...]`,
		Flags:     []cli.Flag{historicFlag, gasFlag},
		Description: `<string> is mandatory parameter.

` + cmdargs.SignersParsingDoc + `

Example:
> loadhex 0c0c48656c6c6f20776f726c6421`,
		Action: handleLoadHex,
	},
	{
		Name:      "loadgo",
		Usage:     "Compile and load a Go file with the manifest into the VM optionally attaching to it provided signers with scopes",
		UsageText: `loadgo [--historic <height>] [--gas <int>] <file> [<signer-with-scope>, ...]`,
		Flags:     []cli.Flag{historicFlag, gasFlag},
		Description: `<file> is mandatory parameter.

` + cmdargs.SignersParsingDoc + `

Example:
> loadgo /path/to/file.go`,
		Action: handleLoadGo,
	},
	{
		Name:      "loadtx",
		Usage:     "Load transaction into the VM from chain or from parameter context file",
		UsageText: `loadtx [--historic <height>] [--gas <int>] <file-or-hash>`,
		Flags:     []cli.Flag{historicFlag, gasFlag},
		Description: `Load transaction into the VM from chain or from parameter context file.
   The transaction script will be loaded into VM; the resulting execution context
   will use the provided transaction as script container including its signers,
   hash and nonce. It'll also use transaction's system fee value as GAS limit if
   --gas option is not used.

<file-or-hash> is mandatory parameter.

Example:
> loadtx /path/to/file`,
		Action: handleLoadTx,
	},
	{
		Name:      "loaddeployed",
		Usage:     "Load deployed contract into the VM from chain optionally attaching to it provided signers with scopes",
		UsageText: `loaddeployed [--historic <height>] [--gas <int>] <hash-or-address-or-id>  [<signer-with-scope>, ...]`,
		Flags:     []cli.Flag{historicFlag, gasFlag},
		Description: `Load deployed contract into the VM from chain optionally attaching to it provided signers with scopes.
If '--historic' flag specified, then the historic contract state (historic script and manifest) will be loaded.

<hash-or-address-or-id> is mandatory parameter.

` + cmdargs.SignersParsingDoc + `

Example:
> loaddeployed 0x0000000009070e030d0f0e020d0c06050e030c02`,
		Action: handleLoadDeployed,
	},
	{
		Name:        "reset",
		Usage:       "Unload compiled script from the VM and reset context to proper (possibly, historic) state",
		UsageText:   "reset",
		Flags:       []cli.Flag{historicFlag},
		Description: "Unload compiled script from the VM and reset context to proper (possibly, historic) state.",
		Action:      handleReset,
	},
	{
		Name:      "parse",
		Usage:     "Parse provided argument and convert it into other possible formats",
		UsageText: `parse <arg>`,
		Description: `<arg> is an argument which is tried to be interpreted as an item of different types
and converted to other formats. Strings are escaped and output in quotes.`,
		Action: handleParse,
	},
	{
		Name:      "run",
		Usage:     "Usage Execute the current loaded script",
		UsageText: `run [<method> [<parameter>...]]`,
		Description: `<method> is a contract method, specified in manifest. It can be '_' which will push
        parameters onto the stack and execute from the current offset.
<parameter> is a parameter (can be repeated multiple times) that can be specified
        using the same rules as for 'contract testinvokefunction' command:

` + cmdargs.ParamsParsingDoc + `

Example:
> run put int:5 string:some_string_value`,
		Action: handleRun,
	},
	{
		Name:        "cont",
		Usage:       "Continue execution of the current loaded script",
		UsageText:   "cont",
		Description: "Continue execution of the current loaded script.",
		Action:      handleCont,
	},
	{
		Name:      "step",
		Usage:     "Step (n) instruction in the program",
		UsageText: `step [<n>]`,
		Description: `<n> is optional parameter to specify number of instructions to run.

Example:
> step 10`,
		Action: handleStep,
	},
	{
		Name:      "stepinto",
		Usage:     "Stepinto instruction to take in the debugger",
		UsageText: "stepinto",
		Description: `Stepinto instruction to take in the debugger.

Example:
> stepinto`,
		Action: handleStepInto,
	},
	{
		Name:      "stepout",
		Usage:     "Stepout instruction to take in the debugger",
		UsageText: "stepout",
		Description: `Stepout instruction to take in the debugger.

Example:
> stepout`,
		Action: handleStepOut,
	},
	{
		Name:      "stepover",
		Usage:     "Stepover instruction to take in the debugger",
		UsageText: "stepover",
		Description: `Stepover instruction to take in the debugger.

Example:
> stepover`,
		Action: handleStepOver,
	},
	{
		Name:        "ops",
		Usage:       "Dump opcodes of the current loaded program",
		UsageText:   "ops",
		Description: "Dump opcodes of the current loaded program",
		Action:      handleOps,
	},
	{
		Name:        "events",
		Usage:       "Dump events emitted by the current loaded program",
		UsageText:   "events",
		Description: "Dump events emitted by the current loaded program",
		Action:      handleEvents,
	},
	{
		Name:      "env",
		Usage:     "Dump state of the chain that is used for VM CLI invocations (use -v for verbose node configuration)",
		UsageText: `env [-v]`,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  verboseFlagFullName + ",v",
				Usage: "Print the whole blockchain node configuration.",
			},
		},
		Description: `Dump state of the chain that is used for VM CLI invocations (use -v for verbose node configuration).

Example:
> env -v`,
		Action: handleEnv,
	},
	{
		Name:      "storage",
		Usage:     "Dump storage of the contract with the specified hash, address or ID as is at the current stage of script invocation",
		UsageText: `storage <hash-or-address-or-id> [<prefix>] [--backwards] [--diff]`,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  backwardsFlagFullName + ",b",
				Usage: "Backwards traversal direction",
			},
			cli.BoolFlag{
				Name:  diffFlagFullName + ",d",
				Usage: "Dump only those storage items that were added or changed during the current script invocation. Note that this call won't show removed storage items, use 'changes' command for that.",
			},
		},
		Description: `Dump storage of the contract with the specified hash, address or ID as is at the current stage of script invocation.
Can be used if no script is loaded.
Hex-encoded storage items prefix may be specified (empty by default to return the whole set of storage items).
If seek prefix is not empty, then it's trimmed from the resulting keys.
Items are sorted. Backwards seek direction may be specified (false by default, which means forwards storage seek direction).
It is possible to dump only those storage items that were added or changed during current script invocation (use --diff flag for it).
To dump the whole set of storage changes including removed items use 'changes' command.

Example:
> storage 0x0000000009070e030d0f0e020d0c06050e030c02 030e --backwards --diff`,
		Action: handleStorage,
	},
	{
		Name:      "changes",
		Usage:     "Dump storage changes as is at the current stage of loaded script invocation",
		UsageText: `changes [<hash-or-address-or-id> [<prefix>]]`,
		Description: `Dump storage changes as is at the current stage of loaded script invocation.
If no script is loaded or executed, then no changes are present.
The contract hash, address or ID may be specified as the first parameter to dump the specified contract storage changes.
Hex-encoded search prefix (without contract ID) may be specified to dump matching storage changes.
Resulting values are not sorted.

Example:
> changes 0x0000000009070e030d0f0e020d0c06050e030c02 030e`,
		Action: handleChanges,
	},
}

var completer *readline.PrefixCompleter

func init() {
	var pcItems []readline.PrefixCompleterInterface
	for _, c := range commands {
		if !c.Hidden {
			var flagsItems []readline.PrefixCompleterInterface
			for _, f := range c.Flags {
				names := strings.SplitN(f.GetName(), ", ", 2) // only long name will be offered
				flagsItems = append(flagsItems, readline.PcItem("--"+names[0]))
			}
			pcItems = append(pcItems, readline.PcItem(c.Name, flagsItems...))
		}
	}
	completer = readline.NewPrefixCompleter(pcItems...)
}

// Various errors.
var (
	ErrMissingParameter = errors.New("missing argument")
	ErrInvalidParameter = errors.New("can't parse argument")
)

// CLI object for interacting with the VM.
type CLI struct {
	chain *core.Blockchain
	shell *cli.App
}

// NewWithConfig returns new CLI instance using provided config and (optionally)
// provided node config for state-backed VM.
func NewWithConfig(printLogotype bool, onExit func(int), c *readline.Config, cfg config.Config) (*CLI, error) {
	if c.AutoComplete == nil {
		// Autocomplete commands/flags on TAB.
		c.AutoComplete = completer
	}
	l, err := readline.NewEx(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create readline instance: %w", err)
	}
	ctl := cli.NewApp()
	ctl.Name = "VM CLI"

	// Note: need to set empty `ctl.HelpName` and `ctl.UsageText`, otherwise
	// `filepath.Base(os.Args[0])` will be used which is `neo-go`.
	ctl.HelpName = ""
	ctl.UsageText = ""

	ctl.Writer = l.Stdout()
	ctl.ErrWriter = l.Stderr()
	ctl.Version = config.Version
	ctl.Usage = "Official VM CLI for NeoGo"

	// Override default error handler in order not to exit on error.
	ctl.ExitErrHandler = func(context *cli.Context, err error) {}

	ctl.Commands = commands

	store, err := storage.NewStore(cfg.ApplicationConfiguration.DBConfiguration)
	if err != nil {
		writeErr(ctl.ErrWriter, fmt.Errorf("failed to open DB, clean in-memory storage will be used: %w", err))
		cfg.ApplicationConfiguration.DBConfiguration.Type = dbconfig.InMemoryDB
		store = storage.NewMemoryStore()
	}

	log, _, logCloser, err := options.HandleLoggingParams(false, cfg.ApplicationConfiguration)
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("failed to init logger: %w", err), 1)
	}
	filter := zap.WrapCore(func(z zapcore.Core) zapcore.Core {
		return options.NewFilteringCore(z, func(entry zapcore.Entry) bool {
			// Log only Runtime.Notify messages.
			return entry.Level == zapcore.InfoLevel && entry.Message == runtime.SystemRuntimeLogMessage
		})
	})
	fLog := log.WithOptions(filter)

	exitF := func(i int) {
		_ = store.Close()
		if logCloser != nil {
			_ = logCloser()
		}
		onExit(i)
	}

	chain, err := core.NewBlockchain(store, cfg.ProtocolConfiguration, fLog)
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("could not initialize blockchain: %w", err), 1)
	}
	// Do not run chain, we need only state-related functionality from it.
	ic, err := chain.GetTestVM(trigger.Application, nil, nil)
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("failed to create test VM: %w", err), 1)
	}

	vmcli := CLI{
		chain: chain,
		shell: ctl,
	}

	vmcli.shell.Metadata = map[string]interface{}{
		chainKey:            chain,
		chainCfgKey:         cfg,
		icKey:               ic,
		manifestKey:         new(manifest.Manifest),
		exitFuncKey:         exitF,
		readlineInstanceKey: l,
		printLogoKey:        printLogotype,
	}
	changePrompt(vmcli.shell)
	return &vmcli, nil
}

func getExitFuncFromContext(app *cli.App) func(int) {
	return app.Metadata[exitFuncKey].(func(int))
}

func getReadlineInstanceFromContext(app *cli.App) *readline.Instance {
	return app.Metadata[readlineInstanceKey].(*readline.Instance)
}

func getVMFromContext(app *cli.App) *vm.VM {
	return getInteropContextFromContext(app).VM
}

func getChainFromContext(app *cli.App) *core.Blockchain {
	return app.Metadata[chainKey].(*core.Blockchain)
}

func getChainConfigFromContext(app *cli.App) config.Config {
	return app.Metadata[chainCfgKey].(config.Config)
}

func getInteropContextFromContext(app *cli.App) *interop.Context {
	return app.Metadata[icKey].(*interop.Context)
}

func getManifestFromContext(app *cli.App) *manifest.Manifest {
	return app.Metadata[manifestKey].(*manifest.Manifest)
}

func getPrintLogoFromContext(app *cli.App) bool {
	return app.Metadata[printLogoKey].(bool)
}

func setInteropContextInContext(app *cli.App, ic *interop.Context) {
	app.Metadata[icKey] = ic
}

func setManifestInContext(app *cli.App, m *manifest.Manifest) {
	app.Metadata[manifestKey] = m
}

func checkVMIsReady(app *cli.App) bool {
	v := getVMFromContext(app)
	if v == nil || !v.Ready() {
		writeErr(app.Writer, errors.New("VM is not ready: no program loaded"))
		return false
	}
	return true
}

func handleExit(c *cli.Context) error {
	finalizeInteropContext(c.App)
	l := getReadlineInstanceFromContext(c.App)
	_ = l.Close()
	exit := getExitFuncFromContext(c.App)
	fmt.Fprintln(c.App.Writer, "Bye!")
	exit(0)
	return nil
}

func handleIP(c *cli.Context) error {
	if !checkVMIsReady(c.App) {
		return nil
	}
	v := getVMFromContext(c.App)
	ctx := v.Context()
	if ctx.NextIP() < ctx.LenInstr() {
		ip, opcode := v.Context().NextInstr()
		fmt.Fprintf(c.App.Writer, "instruction pointer at %d (%s)\n", ip, opcode)
	} else {
		fmt.Fprintln(c.App.Writer, "execution has finished")
	}
	return nil
}

func handleBreak(c *cli.Context) error {
	if !checkVMIsReady(c.App) {
		return nil
	}
	n, err := getInstructionParameter(c)
	if err != nil {
		return err
	}

	v := getVMFromContext(c.App)
	v.AddBreakPoint(n)
	fmt.Fprintf(c.App.Writer, "breakpoint added at instruction %d\n", n)
	return nil
}

func handleJump(c *cli.Context) error {
	if !checkVMIsReady(c.App) {
		return nil
	}
	n, err := getInstructionParameter(c)
	if err != nil {
		return err
	}

	v := getVMFromContext(c.App)
	v.Context().Jump(n)
	fmt.Fprintf(c.App.Writer, "jumped to instruction %d\n", n)
	return nil
}

func getInstructionParameter(c *cli.Context) (int, error) {
	args := c.Args()
	if len(args) != 1 {
		return 0, fmt.Errorf("%w: <ip>", ErrMissingParameter)
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidParameter, err)
	}
	return n, nil
}

func handleXStack(c *cli.Context) error {
	v := getVMFromContext(c.App)
	var stackDump string
	switch c.Command.Name {
	case "estack":
		stackDump = v.DumpEStack()
	case "istack":
		stackDump = v.DumpIStack()
	default:
		return errors.New("unknown stack")
	}
	fmt.Fprintln(c.App.Writer, stackDump)
	return nil
}

func handleSlots(c *cli.Context) error {
	v := getVMFromContext(c.App)
	vmCtx := v.Context()
	if vmCtx == nil {
		return errors.New("no program loaded")
	}
	var rawSlot string
	switch c.Command.Name {
	case "sslot":
		rawSlot = vmCtx.DumpStaticSlot()
	case "lslot":
		rawSlot = vmCtx.DumpLocalSlot()
	case "aslot":
		rawSlot = vmCtx.DumpArgumentsSlot()
	default:
		return errors.New("unknown slot")
	}
	fmt.Fprintln(c.App.Writer, rawSlot)
	return nil
}

// prepareVM retrieves --historic flag from context (if set) and resets app state
// (to the specified historic height if given).
func prepareVM(c *cli.Context, tx *transaction.Transaction) error {
	var err error
	if c.IsSet(historicFlagFullName) {
		height := c.Int(historicFlagFullName)
		err = resetState(c.App, tx, uint32(height))
	} else {
		err = resetState(c.App, tx)
	}
	if err != nil {
		return err
	}
	if c.IsSet(gasFlagFullName) {
		gas := c.Int64(gasFlagFullName)
		v := getVMFromContext(c.App)
		v.GasLimit = gas
	}
	return nil
}

func handleLoadNEF(c *cli.Context) error {
	args := c.Args()
	if len(args) < 2 {
		return fmt.Errorf("%w: <file> <manifest>", ErrMissingParameter)
	}
	b, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	nef, err := nef.FileFromBytes(b)
	if err != nil {
		return fmt.Errorf("failed to decode NEF file: %w", err)
	}
	m, err := getManifestFromFile(args[1])
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}
	var signers []transaction.Signer
	if len(args) > 2 {
		signers, err = cmdargs.ParseSigners(c.Args()[2:])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidParameter, err)
		}
	}
	err = prepareVM(c, createFakeTransaction(nef.Script, signers))
	if err != nil {
		return err
	}
	v := getVMFromContext(c.App)
	fmt.Fprintf(c.App.Writer, "READY: loaded %d instructions\n", v.Context().LenInstr())
	setManifestInContext(c.App, m)
	changePrompt(c.App)
	return nil
}

func handleLoadBase64(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return fmt.Errorf("%w: <string>", ErrMissingParameter)
	}
	b, err := base64.StdEncoding.DecodeString(args[0])
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidParameter, err)
	}
	var signers []transaction.Signer
	if len(args) > 1 {
		signers, err = cmdargs.ParseSigners(args[1:])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidParameter, err)
		}
	}
	err = prepareVM(c, createFakeTransaction(b, signers))
	if err != nil {
		return err
	}
	v := getVMFromContext(c.App)
	fmt.Fprintf(c.App.Writer, "READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c.App)
	return nil
}

// createFakeTransaction creates fake transaction with prefilled script, VUB and signers.
func createFakeTransaction(script []byte, signers []transaction.Signer) *transaction.Transaction {
	return &transaction.Transaction{
		Script:  script,
		Signers: signers,
	}
}

func handleLoadHex(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return fmt.Errorf("%w: <string>", ErrMissingParameter)
	}
	b, err := hex.DecodeString(args[0])
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidParameter, err)
	}
	var signers []transaction.Signer
	if len(args) > 1 {
		signers, err = cmdargs.ParseSigners(args[1:])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidParameter, err)
		}
	}
	err = prepareVM(c, createFakeTransaction(b, signers))
	if err != nil {
		return err
	}
	v := getVMFromContext(c.App)
	fmt.Fprintf(c.App.Writer, "READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c.App)
	return nil
}

func handleLoadGo(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return fmt.Errorf("%w: <file>", ErrMissingParameter)
	}

	name := strings.TrimSuffix(args[0], ".go")
	b, di, err := compiler.CompileWithOptions(args[0], nil, &compiler.Options{Name: name})
	if err != nil {
		return fmt.Errorf("failed to compile: %w", err)
	}

	// Don't perform checks, just load.
	m, err := di.ConvertToManifest(&compiler.Options{})
	if err != nil {
		return fmt.Errorf("can't create manifest: %w", err)
	}
	var signers []transaction.Signer
	if len(args) > 1 {
		signers, err = cmdargs.ParseSigners(args[1:])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidParameter, err)
		}
	}

	err = prepareVM(c, createFakeTransaction(b.Script, signers))
	if err != nil {
		return err
	}
	v := getVMFromContext(c.App)
	setManifestInContext(c.App, m)
	fmt.Fprintf(c.App.Writer, "READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c.App)
	return nil
}

func handleLoadTx(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return fmt.Errorf("%w: <file-or-hash>", ErrMissingParameter)
	}

	var (
		tx  *transaction.Transaction
		err error
	)
	h, err := util.Uint256DecodeStringLE(strings.TrimPrefix(args[0], "0x"))
	if err != nil {
		pc, err := paramcontext.Read(args[0])
		if err != nil {
			return fmt.Errorf("invalid tx hash or path to parameter context: %w", err)
		}
		var ok bool
		tx, ok = pc.Verifiable.(*transaction.Transaction)
		if !ok {
			return errors.New("failed to retrieve transaction from parameter context: verifiable item is not a transaction")
		}
	} else {
		bc := getChainFromContext(c.App)
		tx, _, err = bc.GetTransaction(h)
		if err != nil {
			return fmt.Errorf("failed to get transaction from chain: %w", err)
		}
	}
	err = prepareVM(c, tx)
	if err != nil {
		return err
	}
	v := getVMFromContext(c.App)
	if v.GasLimit == -1 {
		v.GasLimit = tx.SystemFee
	}
	fmt.Fprintf(c.App.Writer, "READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c.App)
	return nil
}

func handleLoadDeployed(c *cli.Context) error {
	err := prepareVM(c, nil) // prepare historic IC if needed (for further historic contract state retrieving).
	if err != nil {
		return err
	}
	if !c.Args().Present() {
		return errors.New("contract hash, address or ID is mandatory argument")
	}
	hashOrID := c.Args().Get(0)
	ic := getInteropContextFromContext(c.App)
	h, err := flags.ParseAddress(hashOrID)
	if err != nil {
		i, err := strconv.ParseInt(hashOrID, 10, 32)
		if err != nil {
			return fmt.Errorf("failed to parse contract hash, address or ID: %w", err)
		}
		h, err = native.GetContractScriptHash(ic.DAO, int32(i))
		if err != nil {
			return fmt.Errorf("failed to retrieve contract hash by ID: %w", err)
		}
	}
	cs, err := ic.GetContract(h) // will return historic contract state.
	if err != nil {
		return fmt.Errorf("contract %s not found: %w", h.StringLE(), err)
	}

	var signers []transaction.Signer
	if len(c.Args()) > 1 {
		signers, err = cmdargs.ParseSigners(c.Args()[1:])
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidParameter, err)
		}
	}
	err = prepareVM(c, createFakeTransaction(cs.NEF.Script, signers)) // prepare VM one more time for proper IC initialization.
	if err != nil {
		return err
	}
	ic = getInteropContextFromContext(c.App) // fetch newly-created IC.
	gasLimit := ic.VM.GasLimit
	ic.ReuseVM(ic.VM) // clear previously loaded program and context.
	ic.VM.GasLimit = gasLimit
	ic.VM.LoadScriptWithHash(cs.NEF.Script, cs.Hash, callflag.All)
	fmt.Fprintf(c.App.Writer, "READY: loaded %d instructions\n", ic.VM.Context().LenInstr())
	setManifestInContext(c.App, &cs.Manifest)
	changePrompt(c.App)
	return nil
}

func handleReset(c *cli.Context) error {
	err := prepareVM(c, nil)
	if err != nil {
		return err
	}
	changePrompt(c.App)
	return nil
}

// finalizeInteropContext calls finalizer for the current interop context.
func finalizeInteropContext(app *cli.App) {
	ic := getInteropContextFromContext(app)
	ic.Finalize()
}

// resetInteropContext calls finalizer for current interop context and replaces
// it with the newly created one. If transaction is provided, then its script is
// loaded into bound VM.
func resetInteropContext(app *cli.App, tx *transaction.Transaction, height ...uint32) error {
	finalizeInteropContext(app)
	bc := getChainFromContext(app)
	var (
		newIc *interop.Context
		err   error
	)
	if len(height) != 0 {
		if tx != nil {
			tx.ValidUntilBlock = height[0] + 1
		}
		newIc, err = bc.GetTestHistoricVM(trigger.Application, tx, height[0]+1)
		if err != nil {
			return fmt.Errorf("failed to create historic VM for height %d: %w", height[0], err)
		}
	} else {
		if tx != nil {
			tx.ValidUntilBlock = bc.BlockHeight() + 1
		}
		newIc, err = bc.GetTestVM(trigger.Application, tx, nil)
		if err != nil {
			return fmt.Errorf("failed to create VM: %w", err)
		}
	}
	if tx != nil {
		newIc.VM.LoadWithFlags(tx.Script, callflag.All)
	}

	setInteropContextInContext(app, newIc)
	return nil
}

// resetManifest removes manifest from app context.
func resetManifest(app *cli.App) {
	setManifestInContext(app, nil)
}

// resetState resets state of the app (clear interop context and manifest) so that it's ready
// to load new program.
func resetState(app *cli.App, tx *transaction.Transaction, height ...uint32) error {
	err := resetInteropContext(app, tx, height...)
	if err != nil {
		return err
	}
	resetManifest(app)
	return nil
}

func getManifestFromFile(name string) (*manifest.Manifest, error) {
	bs, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("%w: can't read manifest", ErrInvalidParameter)
	}

	var m manifest.Manifest
	if err := json.Unmarshal(bs, &m); err != nil {
		return nil, fmt.Errorf("%w: can't unmarshal manifest", ErrInvalidParameter)
	}
	return &m, nil
}

func handleRun(c *cli.Context) error {
	v := getVMFromContext(c.App)
	m := getManifestFromContext(c.App)
	args := c.Args()
	if len(args) != 0 {
		var (
			params     []stackitem.Item
			offset     int
			err        error
			runCurrent = args[0] != "_"
		)

		_, scParams, err := cmdargs.ParseParams(args[1:], true)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidParameter, err)
		}
		params = make([]stackitem.Item, len(scParams))
		for i := range scParams {
			params[i], err = scParams[i].ToStackItem()
			if err != nil {
				return fmt.Errorf("failed to convert parameter #%d to stackitem: %w", i, err)
			}
		}
		if runCurrent {
			if m == nil {
				return fmt.Errorf("manifest is not loaded; either use 'run' command to run loaded script from the start or use 'loadgo' and 'loadnef' commands to provide manifest")
			}
			md := m.ABI.GetMethod(args[0], len(params))
			if md == nil {
				return fmt.Errorf("%w: method not found", ErrInvalidParameter)
			}
			offset = md.Offset
		}
		for i := len(params) - 1; i >= 0; i-- {
			v.Estack().PushVal(params[i])
		}
		if runCurrent {
			if !v.Ready() {
				return errors.New("no program loaded")
			}
			v.Context().Jump(offset)
			if initMD := m.ABI.GetMethod(manifest.MethodInit, 0); initMD != nil {
				v.Call(initMD.Offset)
			}
		}
	}
	runVMWithHandling(c)
	changePrompt(c.App)
	return nil
}

// runVMWithHandling runs VM with handling errors and additional state messages.
func runVMWithHandling(c *cli.Context) {
	v := getVMFromContext(c.App)
	err := v.Run()
	if err != nil {
		writeErr(c.App.ErrWriter, err)
	}

	var (
		message string
		dumpNtf bool
	)
	switch {
	case v.HasFailed():
		message = "" // the error will be printed on return
		dumpNtf = true
	case v.HasHalted():
		message = v.DumpEStack()
		dumpNtf = true
	case v.AtBreakpoint():
		ctx := v.Context()
		if ctx.NextIP() < ctx.LenInstr() {
			i, op := ctx.NextInstr()
			message = fmt.Sprintf("at breakpoint %d (%s)", i, op)
		} else {
			message = "execution has finished"
		}
	}
	if dumpNtf {
		var e string
		e, err = dumpEvents(c.App)
		if err == nil && len(e) != 0 {
			if message != "" {
				message += "\n"
			}
			message += "Events:\n" + e
		}
	}
	if message != "" {
		fmt.Fprintln(c.App.Writer, message)
	}
}

func handleCont(c *cli.Context) error {
	if !checkVMIsReady(c.App) {
		return nil
	}
	runVMWithHandling(c)
	changePrompt(c.App)
	return nil
}

func handleStep(c *cli.Context) error {
	var (
		n   = 1
		err error
	)

	if !checkVMIsReady(c.App) {
		return nil
	}
	v := getVMFromContext(c.App)
	args := c.Args()
	if len(args) > 0 {
		n, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidParameter, err)
		}
	}
	v.AddBreakPointRel(n)
	runVMWithHandling(c)
	changePrompt(c.App)
	return nil
}

func handleStepInto(c *cli.Context) error {
	return handleStepType(c, "into")
}

func handleStepOut(c *cli.Context) error {
	return handleStepType(c, "out")
}

func handleStepOver(c *cli.Context) error {
	return handleStepType(c, "over")
}

func handleStepType(c *cli.Context, stepType string) error {
	if !checkVMIsReady(c.App) {
		return nil
	}
	v := getVMFromContext(c.App)
	var err error
	switch stepType {
	case "into":
		err = v.StepInto()
	case "out":
		err = v.StepOut()
	case "over":
		err = v.StepOver()
	}
	if err != nil {
		return err
	}
	_ = handleIP(c)
	changePrompt(c.App)
	return nil
}

func handleOps(c *cli.Context) error {
	if !checkVMIsReady(c.App) {
		return nil
	}
	v := getVMFromContext(c.App)
	out := bytes.NewBuffer(nil)
	v.PrintOps(out)
	fmt.Fprintln(c.App.Writer, out.String())
	return nil
}

func changePrompt(app *cli.App) {
	v := getVMFromContext(app)
	l := getReadlineInstanceFromContext(app)
	if v.Ready() && v.Context().NextIP() >= 0 && v.Context().NextIP() < v.Context().LenInstr() {
		l.SetPrompt(fmt.Sprintf("\033[32mNEO-GO-VM %d >\033[0m ", v.Context().NextIP()))
	} else {
		l.SetPrompt("\033[32mNEO-GO-VM >\033[0m ")
	}
}

func handleEvents(c *cli.Context) error {
	e, err := dumpEvents(c.App)
	if err != nil {
		writeErr(c.App.ErrWriter, err)
		return nil
	}
	fmt.Fprintln(c.App.Writer, e)
	return nil
}

func handleEnv(c *cli.Context) error {
	bc := getChainFromContext(c.App)
	cfg := getChainConfigFromContext(c.App)
	ic := getInteropContextFromContext(c.App)
	message := fmt.Sprintf("Chain height: %d\nVM height (may differ from chain height in case of historic call): %d\nNetwork magic: %d\nDB type: %s\n",
		bc.BlockHeight(), ic.BlockHeight(), bc.GetConfig().Magic, cfg.ApplicationConfiguration.DBConfiguration.Type)
	if c.Bool(verboseFlagFullName) {
		cfgBytes, err := json.MarshalIndent(cfg, "", "\t")
		if err != nil {
			return fmt.Errorf("failed to marshal node configuration: %w", err)
		}
		message += "Node config:\n" + string(cfgBytes) + "\n"
	}
	fmt.Fprint(c.App.Writer, message)
	return nil
}

func handleStorage(c *cli.Context) error {
	id, prefix, err := getDumpArgs(c)
	if err != nil {
		return err
	}
	var (
		backwards bool
		seekDepth int
		ic        = getInteropContextFromContext(c.App)
	)
	if c.Bool(backwardsFlagFullName) {
		backwards = true
	}
	if c.Bool(diffFlagFullName) {
		seekDepth = 1 // take only upper DAO layer which stores only added or updated items.
	}
	ic.DAO.Seek(id, storage.SeekRange{
		Prefix:      prefix,
		Backwards:   backwards,
		SearchDepth: seekDepth,
	}, func(k, v []byte) bool {
		fmt.Fprintf(c.App.Writer, "%s: %v\n", hex.EncodeToString(k), hex.EncodeToString(v))
		return true
	})
	return nil
}

func handleChanges(c *cli.Context) error {
	var (
		expectedID int32
		prefix     []byte
		err        error
		hasAgs     = c.Args().Present()
	)
	if hasAgs {
		expectedID, prefix, err = getDumpArgs(c)
		if err != nil {
			return err
		}
	}
	ic := getInteropContextFromContext(c.App)
	b := ic.DAO.GetBatch()
	if b == nil {
		return nil
	}
	ops := storage.BatchToOperations(b)
	var notFirst bool
	for _, op := range ops {
		id := int32(binary.LittleEndian.Uint32(op.Key))
		if hasAgs && (expectedID != id || (len(prefix) != 0 && !bytes.HasPrefix(op.Key[4:], prefix))) {
			continue
		}
		var message string
		if notFirst {
			message += "\n"
		}
		message += fmt.Sprintf("Contract ID: %d\nState: %s\nKey: %s\n", id, op.State, hex.EncodeToString(op.Key[4:]))
		if op.Value != nil {
			message += fmt.Sprintf("Value: %s\n", hex.EncodeToString(op.Value))
		}
		fmt.Fprint(c.App.Writer, message)
		notFirst = true
	}
	return nil
}

// getDumpArgs is a helper function that retrieves contract ID and search prefix (if given).
func getDumpArgs(c *cli.Context) (int32, []byte, error) {
	id, err := getContractID(c)
	if err != nil {
		return 0, nil, err
	}
	var prefix []byte
	if c.NArg() > 1 {
		prefix, err = hex.DecodeString(c.Args().Get(1))
		if err != nil {
			return 0, nil, fmt.Errorf("failed to decode prefix from hex: %w", err)
		}
	}
	return id, prefix, nil
}

// getContractID returns contract ID parsed from the first argument which can be ID,
// hash or address.
func getContractID(c *cli.Context) (int32, error) {
	if !c.Args().Present() {
		return 0, errors.New("contract hash, address or ID is mandatory argument")
	}
	hashOrID := c.Args().Get(0)
	var ic = getInteropContextFromContext(c.App)
	h, err := flags.ParseAddress(hashOrID)
	if err != nil {
		i, err := strconv.ParseInt(hashOrID, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("failed to parse contract hash, address or ID: %w", err)
		}
		return int32(i), nil
	}
	cs, err := ic.GetContract(h)
	if err != nil {
		return 0, fmt.Errorf("contract %s not found: %w", h.StringLE(), err)
	}
	return cs.ID, nil
}

func dumpEvents(app *cli.App) (string, error) {
	ic := getInteropContextFromContext(app)
	if len(ic.Notifications) == 0 {
		return "", nil
	}
	b, err := json.MarshalIndent(ic.Notifications, "", "\t")
	if err != nil {
		return "", fmt.Errorf("failed to marshal notifications: %w", err)
	}
	return string(b), nil
}

// Run waits for user input from Stdin and executes the passed command.
func (c *CLI) Run() error {
	if getPrintLogoFromContext(c.shell) {
		printLogo(c.shell.Writer)
	}
	l := getReadlineInstanceFromContext(c.shell)
	for {
		line, err := l.Readline()
		if errors.Is(err, io.EOF) || errors.Is(err, readline.ErrInterrupt) {
			return nil // OK, stop execution.
		}
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err) // Critical error, stop execution.
		}

		args, err := shellquote.Split(line)
		if err != nil {
			writeErr(c.shell.ErrWriter, fmt.Errorf("failed to parse arguments: %w", err))
			continue // Not a critical error, continue execution.
		}

		err = c.shell.Run(append([]string{"vm"}, args...))
		if err != nil {
			writeErr(c.shell.ErrWriter, err) // Various command/flags parsing errors and execution errors.
		}
	}
}

func handleParse(c *cli.Context) error {
	res, err := Parse(c.Args())
	if err != nil {
		return err
	}
	fmt.Fprintln(c.App.Writer, res)
	return nil
}

// Parse converts it's argument to other formats.
func Parse(args []string) (string, error) {
	if len(args) < 1 {
		return "", ErrMissingParameter
	}
	arg := args[0]
	buf := bytes.NewBuffer(nil)
	if val, err := strconv.ParseInt(arg, 10, 64); err == nil {
		bs := bigint.ToBytes(big.NewInt(val))
		buf.WriteString(fmt.Sprintf("Integer to Hex\t%s\n", hex.EncodeToString(bs)))
		buf.WriteString(fmt.Sprintf("Integer to Base64\t%s\n", base64.StdEncoding.EncodeToString(bs)))
	}
	noX := strings.TrimPrefix(arg, "0x")
	if rawStr, err := hex.DecodeString(noX); err == nil {
		if val, err := util.Uint160DecodeBytesBE(rawStr); err == nil {
			buf.WriteString(fmt.Sprintf("BE ScriptHash to Address\t%s\n", address.Uint160ToString(val)))
			buf.WriteString(fmt.Sprintf("LE ScriptHash to Address\t%s\n", address.Uint160ToString(val.Reverse())))
		}
		if pub, err := keys.NewPublicKeyFromBytes(rawStr, elliptic.P256()); err == nil {
			sh := pub.GetScriptHash()
			buf.WriteString(fmt.Sprintf("Public key to BE ScriptHash\t%s\n", sh))
			buf.WriteString(fmt.Sprintf("Public key to LE ScriptHash\t%s\n", sh.Reverse()))
			buf.WriteString(fmt.Sprintf("Public key to Address\t%s\n", address.Uint160ToString(sh)))
		}
		buf.WriteString(fmt.Sprintf("Hex to String\t%s\n", fmt.Sprintf("%q", string(rawStr))))
		buf.WriteString(fmt.Sprintf("Hex to Integer\t%s\n", bigint.FromBytes(rawStr)))
		buf.WriteString(fmt.Sprintf("Swap Endianness\t%s\n", hex.EncodeToString(slice.CopyReverse(rawStr))))
	}
	if addr, err := address.StringToUint160(arg); err == nil {
		buf.WriteString(fmt.Sprintf("Address to BE ScriptHash\t%s\n", addr))
		buf.WriteString(fmt.Sprintf("Address to LE ScriptHash\t%s\n", addr.Reverse()))
		buf.WriteString(fmt.Sprintf("Address to Base64 (BE)\t%s\n", base64.StdEncoding.EncodeToString(addr.BytesBE())))
		buf.WriteString(fmt.Sprintf("Address to Base64 (LE)\t%s\n", base64.StdEncoding.EncodeToString(addr.BytesLE())))
	}
	if rawStr, err := base64.StdEncoding.DecodeString(arg); err == nil {
		buf.WriteString(fmt.Sprintf("Base64 to String\t%s\n", fmt.Sprintf("%q", string(rawStr))))
		buf.WriteString(fmt.Sprintf("Base64 to BigInteger\t%s\n", bigint.FromBytes(rawStr)))
		if u, err := util.Uint160DecodeBytesBE(rawStr); err == nil {
			buf.WriteString(fmt.Sprintf("Base64 to BE ScriptHash\t%s\n", u.StringBE()))
			buf.WriteString(fmt.Sprintf("Base64 to LE ScriptHash\t%s\n", u.StringLE()))
			buf.WriteString(fmt.Sprintf("Base64 to Address (BE)\t%s\n", address.Uint160ToString(u)))
			buf.WriteString(fmt.Sprintf("Base64 to Address (LE)\t%s\n", address.Uint160ToString(u.Reverse())))
		}
	}

	buf.WriteString(fmt.Sprintf("String to Hex\t%s\n", hex.EncodeToString([]byte(arg))))
	buf.WriteString(fmt.Sprintf("String to Base64\t%s\n", base64.StdEncoding.EncodeToString([]byte(arg))))

	out := buf.Bytes()
	buf = bytes.NewBuffer(nil)
	w := tabwriter.NewWriter(buf, 0, 4, 4, '\t', 0)
	if _, err := w.Write(out); err != nil {
		return "", err
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const logo = `
    _   ____________        __________      _    ____  ___
   / | / / ____/ __ \      / ____/ __ \    | |  / /  |/  /
  /  |/ / __/ / / / /_____/ / __/ / / /____| | / / /|_/ / 
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /_____/ |/ / /  / /  
/_/ |_/_____/\____/      \____/\____/      |___/_/  /_/   
`

func printLogo(w io.Writer) {
	fmt.Fprint(w, logo)
	fmt.Fprintln(w)
	fmt.Fprintln(w)
	fmt.Fprintln(w)
}

func writeErr(w io.Writer, err error) {
	fmt.Fprintf(w, "Error: %s\n", err)
}
