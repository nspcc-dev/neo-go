package vm

import (
	"bytes"
	"crypto/elliptic"
	"encoding/base64"
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
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/urfave/cli"
	"go.uber.org/zap"
)

const (
	chainKey            = "chain"
	icKey               = "ic"
	manifestKey         = "manifest"
	exitFuncKey         = "exitFunc"
	readlineInstanceKey = "readlineKey"
	printLogoKey        = "printLogoKey"
	boolType            = "bool"
	boolFalse           = "false"
	boolTrue            = "true"
	intType             = "int"
	stringType          = "string"
)

var commands = []cli.Command{
	{
		Name:        "exit",
		Usage:       "Exit the VM prompt",
		Description: "Exit the VM prompt",
		Action:      handleExit,
	},
	{
		Name:        "ip",
		Usage:       "Show current instruction",
		Description: "Show current instruction",
		Action:      handleIP,
	},
	{
		Name:      "break",
		Usage:     "Place a breakpoint",
		UsageText: `break <ip>`,
		Description: `break <ip>
<ip> is mandatory parameter, example:
> break 12`,
		Action: handleBreak,
	},
	{
		Name:        "estack",
		Usage:       "Show evaluation stack contents",
		Description: "Show evaluation stack contents",
		Action:      handleXStack,
	},
	{
		Name:        "istack",
		Usage:       "Show invocation stack contents",
		Description: "Show invocation stack contents",
		Action:      handleXStack,
	},
	{
		Name:        "sslot",
		Usage:       "Show static slot contents",
		Description: "Show static slot contents",
		Action:      handleSlots,
	},
	{
		Name:        "lslot",
		Usage:       "Show local slot contents",
		Description: "Show local slot contents",
		Action:      handleSlots,
	},
	{
		Name:        "aslot",
		Usage:       "Show arguments slot contents",
		Description: "Show arguments slot contents",
		Action:      handleSlots,
	},
	{
		Name:      "loadnef",
		Usage:     "Load a NEF-consistent script into the VM",
		UsageText: `loadnef <file> <manifest>`,
		Description: `loadnef <file> <manifest>
both parameters are mandatory, example:
> loadnef /path/to/script.nef /path/to/manifest.json`,
		Action: handleLoadNEF,
	},
	{
		Name:      "loadbase64",
		Usage:     "Load a base64-encoded script string into the VM",
		UsageText: `loadbase64 <string>`,
		Description: `loadbase64 <string>

<string> is mandatory parameter, example:
> loadbase64 AwAQpdToAAAADBQV9ehtQR1OrVZVhtHtoUHRfoE+agwUzmFvf3Rhfg/EuAVYOvJgKiON9j8TwAwIdHJhbnNmZXIMFDt9NxHG8Mz5sdypA9G/odiW8SOMQWJ9W1I4`,
		Action: handleLoadBase64,
	},
	{
		Name:      "loadhex",
		Usage:     "Load a hex-encoded script string into the VM",
		UsageText: `loadhex <string>`,
		Description: `loadhex <string>

<string> is mandatory parameter, example:
> loadhex 0c0c48656c6c6f20776f726c6421`,
		Action: handleLoadHex,
	},
	{
		Name:      "loadgo",
		Usage:     "Compile and load a Go file with the manifest into the VM",
		UsageText: `loadgo <file>`,
		Description: `loadgo <file>

<file> is mandatory parameter, example:
> loadgo /path/to/file.go`,
		Action: handleLoadGo,
	},
	{
		Name:   "reset",
		Usage:  "Unload compiled script from the VM",
		Action: handleReset,
	},
	{
		Name:      "parse",
		Usage:     "Parse provided argument and convert it into other possible formats",
		UsageText: `parse <arg>`,
		Description: `parse <arg>

<arg> is an argument which is tried to be interpreted as an item of different types
and converted to other formats. Strings are escaped and output in quotes.`,
		Action: handleParse,
	},
	{
		Name:      "run",
		Usage:     "Execute the current loaded script",
		UsageText: `run [<method> [<parameter>...]]`,
		Description: `run [<method> [<parameter>...]]

<method> is a contract method, specified in manifest. It can be '_' which will push
        parameters onto the stack and execute from the current offset.
<parameter> is a parameter (can be repeated multiple times) that can be specified
        as <type>:<value>, where type can be:
            '` + boolType + `': supports '` + boolFalse + `' and '` + boolTrue + `' values
            '` + intType + `': supports integers as values
            '` + stringType + `': supports strings as values (that are pushed as a byte array
                      values to the stack)
       or can be just <value>, for which the type will be detected automatically
       following these rules: '` + boolTrue + `' and '` + boolFalse + `' are treated as respective
       boolean values, everything that can be converted to integer is treated as
       integer and everything else is treated like a string.

Example:
> run put ` + stringType + `:"Something to put"`,
		Action: handleRun,
	},
	{
		Name:        "cont",
		Usage:       "Continue execution of the current loaded script",
		Description: "Continue execution of the current loaded script",
		Action:      handleCont,
	},
	{
		Name:      "step",
		Usage:     "Step (n) instruction in the program",
		UsageText: `step [<n>]`,
		Description: `step [<n>]
<n> is optional parameter to specify number of instructions to run, example:
> step 10`,
		Action: handleStep,
	},
	{
		Name:  "stepinto",
		Usage: "Stepinto instruction to take in the debugger",
		Description: `Usage: stepInto
example:
> stepinto`,
		Action: handleStepInto,
	},
	{
		Name:  "stepout",
		Usage: "Stepout instruction to take in the debugger",
		Description: `stepOut
example:
> stepout`,
		Action: handleStepOut,
	},
	{
		Name:  "stepover",
		Usage: "Stepover instruction to take in the debugger",
		Description: `stepOver
example:
> stepover`,
		Action: handleStepOver,
	},
	{
		Name:        "ops",
		Usage:       "Dump opcodes of the current loaded program",
		Description: "Dump opcodes of the current loaded program",
		Action:      handleOps,
	},
	{
		Name:        "events",
		Usage:       "Dump events emitted by the current loaded program",
		Description: "Dump events emitted by the current loaded program",
		Action:      handleEvents,
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

// VMCLI object for interacting with the VM.
type VMCLI struct {
	chain *core.Blockchain
	shell *cli.App
}

// NewWithConfig returns new VMCLI instance using provided config and (optionally)
// provided node config for state-backed VM.
func NewWithConfig(printLogotype bool, onExit func(int), c *readline.Config, cfg config.Config) (*VMCLI, error) {
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
	ctl.Usage = "Official VM CLI for Neo-Go"

	// Override default error handler in order not to exit on error.
	ctl.ExitErrHandler = func(context *cli.Context, err error) {}

	ctl.Commands = commands

	store, err := storage.NewStore(cfg.ApplicationConfiguration.DBConfiguration)
	if err != nil {
		writeErr(ctl.ErrWriter, fmt.Errorf("failed to open DB, clean in-memory storage will be used: %w", err))
		cfg.ApplicationConfiguration.DBConfiguration.Type = dbconfig.InMemoryDB
		store = storage.NewMemoryStore()
	}

	exitF := func(i int) {
		_ = store.Close()
		onExit(i)
	}

	log := zap.NewNop()
	chain, err := core.NewBlockchain(store, cfg.ProtocolConfiguration, log)
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("could not initialize blockchain: %w", err), 1)
	}
	// Do not run chain, we need only state-related functionality from it.
	ic := chain.GetTestVM(trigger.Application, nil, nil)

	vmcli := VMCLI{
		chain: chain,
		shell: ctl,
	}

	vmcli.shell.Metadata = map[string]interface{}{
		chainKey:            chain,
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
	v := getVMFromContext(c.App)
	args := c.Args()
	if len(args) != 1 {
		return fmt.Errorf("%w: <ip>", ErrMissingParameter)
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidParameter, err)
	}

	v.AddBreakPoint(n)
	fmt.Fprintf(c.App.Writer, "breakpoint added at instruction %d\n", n)
	return nil
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

func handleLoadNEF(c *cli.Context) error {
	resetState(c.App)
	v := getVMFromContext(c.App)
	args := c.Args()
	if len(args) < 2 {
		return fmt.Errorf("%w: <file> <manifest>", ErrMissingParameter)
	}
	if err := v.LoadFileWithFlags(args[0], callflag.All); err != nil {
		return fmt.Errorf("failed to read nef: %w", err)
	}
	m, err := getManifestFromFile(args[1])
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}
	fmt.Fprintf(c.App.Writer, "READY: loaded %d instructions\n", v.Context().LenInstr())
	setManifestInContext(c.App, m)
	changePrompt(c.App)
	return nil
}

func handleLoadBase64(c *cli.Context) error {
	resetState(c.App)
	v := getVMFromContext(c.App)
	args := c.Args()
	if len(args) < 1 {
		return fmt.Errorf("%w: <string>", ErrMissingParameter)
	}
	b, err := base64.StdEncoding.DecodeString(args[0])
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidParameter, err)
	}
	v.LoadWithFlags(b, callflag.All)
	fmt.Fprintf(c.App.Writer, "READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c.App)
	return nil
}

func handleLoadHex(c *cli.Context) error {
	resetState(c.App)
	v := getVMFromContext(c.App)
	args := c.Args()
	if len(args) < 1 {
		return fmt.Errorf("%w: <string>", ErrMissingParameter)
	}
	b, err := hex.DecodeString(args[0])
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidParameter, err)
	}
	v.LoadWithFlags(b, callflag.All)
	fmt.Fprintf(c.App.Writer, "READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c.App)
	return nil
}

func handleLoadGo(c *cli.Context) error {
	resetState(c.App)
	v := getVMFromContext(c.App)
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
	setManifestInContext(c.App, m)

	v.LoadWithFlags(b.Script, callflag.All)
	fmt.Fprintf(c.App.Writer, "READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c.App)
	return nil
}

func handleReset(c *cli.Context) error {
	resetState(c.App)
	changePrompt(c.App)
	return nil
}

// finalizeInteropContext calls finalizer for the current interop context.
func finalizeInteropContext(app *cli.App) {
	ic := getInteropContextFromContext(app)
	ic.Finalize()
}

// resetInteropContext calls finalizer for current interop context and replaces
// it with the newly created one.
func resetInteropContext(app *cli.App) {
	finalizeInteropContext(app)
	bc := getChainFromContext(app)
	newIc := bc.GetTestVM(trigger.Application, nil, nil)
	setInteropContextInContext(app, newIc)
}

// resetManifest removes manifest from app context.
func resetManifest(app *cli.App) {
	setManifestInContext(app, nil)
}

// resetState resets state of the app (clear interop context and manifest) so that it's ready
// to load new program.
func resetState(app *cli.App) {
	resetInteropContext(app)
	resetManifest(app)
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

		params, err = parseArgs(args[1:])
		if err != nil {
			return err
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
func (c *VMCLI) Run() error {
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

func parseArgs(args []string) ([]stackitem.Item, error) {
	items := make([]stackitem.Item, len(args))
	for i, arg := range args {
		var typ, value string
		typeAndVal := strings.Split(arg, ":")
		if len(typeAndVal) < 2 {
			if typeAndVal[0] == boolFalse || typeAndVal[0] == boolTrue {
				typ = boolType
			} else if _, err := strconv.Atoi(typeAndVal[0]); err == nil {
				typ = intType
			} else {
				typ = stringType
			}
			value = typeAndVal[0]
		} else {
			typ = typeAndVal[0]
			value = typeAndVal[1]
		}

		switch typ {
		case boolType:
			if value == boolFalse {
				items[i] = stackitem.NewBool(false)
			} else if value == boolTrue {
				items[i] = stackitem.NewBool(true)
			} else {
				return nil, fmt.Errorf("%w: invalid bool value", ErrInvalidParameter)
			}
		case intType:
			val, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid integer value", ErrInvalidParameter)
			}
			items[i] = stackitem.NewBigInteger(big.NewInt(val))
		case stringType:
			items[i] = stackitem.NewByteArray([]byte(value))
		}
	}

	return items, nil
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
