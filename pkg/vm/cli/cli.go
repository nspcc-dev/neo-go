package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/abiosoft/ishell/v2"
	"github.com/abiosoft/readline"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	vmKey       = "vm"
	manifestKey = "manifest"
	boolType    = "bool"
	boolFalse   = "false"
	boolTrue    = "true"
	intType     = "int"
	stringType  = "string"
	exitFunc    = "exitFunc"
)

var commands = []*ishell.Cmd{
	{
		Name:     "exit",
		Help:     "Exit the VM prompt",
		LongHelp: "Exit the VM prompt",
		Func:     handleExit,
	},
	{
		Name:     "ip",
		Help:     "Show current instruction",
		LongHelp: "Show current instruction",
		Func:     handleIP,
	},
	{
		Name: "break",
		Help: "Place a breakpoint",
		LongHelp: `Usage: break <ip>
<ip> is mandatory parameter, example:
> break 12`,
		Func: handleBreak,
	},
	{
		Name:     "estack",
		Help:     "Show evaluation stack contents",
		LongHelp: "Show evaluation stack contents",
		Func:     handleXStack,
	},
	{
		Name:     "astack",
		Help:     "Show alt stack contents",
		LongHelp: "Show alt stack contents",
		Func:     handleXStack,
	},
	{
		Name:     "istack",
		Help:     "Show invocation stack contents",
		LongHelp: "Show invocation stack contents",
		Func:     handleXStack,
	},
	{
		Name: "loadnef",
		Help: "Load a NEF-consistent script into the VM",
		LongHelp: `Usage: loadnef <file> <manifest>
both parameters are mandatory, example:
> loadnef /path/to/script.nef /path/to/manifest.json`,
		Func: handleLoadNEF,
	},
	{
		Name: "loadbase64",
		Help: "Load a base64-encoded script string into the VM",
		LongHelp: `Usage: loadbase64 <string>
<string> is mandatory parameter, example:
> loadbase64 AwAQpdToAAAADBQV9ehtQR1OrVZVhtHtoUHRfoE+agwUzmFvf3Rhfg/EuAVYOvJgKiON9j8TwAwIdHJhbnNmZXIMFDt9NxHG8Mz5sdypA9G/odiW8SOMQWJ9W1I4`,
		Func: handleLoadBase64,
	},
	{
		Name: "loadhex",
		Help: "Load a hex-encoded script string into the VM",
		LongHelp: `Usage: loadhex <string>
<string> is mandatory parameter, example:
> loadhex 0c0c48656c6c6f20776f726c6421`,
		Func: handleLoadHex,
	},
	{
		Name: "loadgo",
		Help: "Compile and load a Go file with the manifest into the VM",
		LongHelp: `Usage: loadgo <file>
<file> is mandatory parameter, example:
> loadgo /path/to/file.go`,
		Func: handleLoadGo,
	},
	{
		Name: "parse",
		Help: "Parse provided argument and convert it into other possible formats",
		LongHelp: `Usage: parse <arg>

<arg> is an argument which is tried to be interpreted as an item of different types
        and converted to other formats. Strings are escaped and output in quotes.`,
		Func: handleParse,
	},
	{
		Name: "run",
		Help: "Execute the current loaded script",
		LongHelp: `Usage: run [<method> [<parameter>...]]

<method> is a contract method, specified in manifest (and it
        can't be 'help' at the moment). It can be '_' which will push parameters
        onto the stack and execute from the current offset.
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
		Func: handleRun,
	},
	{
		Name:     "cont",
		Help:     "Continue execution of the current loaded script",
		LongHelp: "Continue execution of the current loaded script",
		Func:     handleCont,
	},
	{
		Name: "step",
		Help: "Step (n) instruction in the program",
		LongHelp: `Usage: step [<n>]
<n> is optional parameter to specify number of instructions to run, example:
> step 10`,
		Func: handleStep,
	},
	{
		Name: "stepinto",
		Help: "Stepinto instruction to take in the debugger",
		LongHelp: `Usage: stepInto
example:
> stepinto`,
		Func: handleStepInto,
	},
	{
		Name: "stepout",
		Help: "Stepout instruction to take in the debugger",
		LongHelp: `Usage: stepOut
example:
> stepout`,
		Func: handleStepOut,
	},
	{
		Name: "stepover",
		Help: "Stepover instruction to take in the debugger",
		LongHelp: `Usage: stepOver
example:
> stepover`,
		Func: handleStepOver,
	},
	{
		Name:     "ops",
		Help:     "Dump opcodes of the current loaded program",
		LongHelp: "Dump opcodes of the current loaded program",
		Func:     handleOps,
	},
}

// Various errors.
var (
	ErrMissingParameter = errors.New("missing argument")
	ErrInvalidParameter = errors.New("can't parse argument")
)

// VMCLI object for interacting with the VM.
type VMCLI struct {
	vm    *vm.VM
	shell *ishell.Shell
	// printLogo specifies if logo is printed.
	printLogo bool
}

// New returns a new VMCLI object.
func New() *VMCLI {
	return NewWithConfig(true, os.Exit, &readline.Config{
		Prompt: ">>>",
	})
}

// NewWithConfig returns new VMCLI instance using provided config.
func NewWithConfig(printLogo bool, onExit func(int), c *readline.Config) *VMCLI {
	vmcli := VMCLI{
		vm:        vm.New(),
		shell:     ishell.NewWithConfig(c),
		printLogo: printLogo,
	}
	vmcli.shell.Set(vmKey, vmcli.vm)
	vmcli.shell.Set(manifestKey, new(manifest.Manifest))
	vmcli.shell.Set(exitFunc, onExit)
	for _, c := range commands {
		vmcli.shell.AddCmd(c)
	}
	changePrompt(vmcli.shell, vmcli.vm)
	return &vmcli
}

func getVMFromContext(c *ishell.Context) *vm.VM {
	return c.Get(vmKey).(*vm.VM)
}

func getManifestFromContext(c *ishell.Context) *manifest.Manifest {
	return c.Get(manifestKey).(*manifest.Manifest)
}

func setManifestInContext(c *ishell.Context, m *manifest.Manifest) {
	old := getManifestFromContext(c)
	*old = *m
}

func checkVMIsReady(c *ishell.Context) bool {
	v := getVMFromContext(c)
	if v == nil || !v.Ready() {
		c.Err(errors.New("VM is not ready: no program loaded"))
		return false
	}
	return true
}

func handleExit(c *ishell.Context) {
	c.Println("Bye!")
	c.Get(exitFunc).(func(int))(0)
}

func handleIP(c *ishell.Context) {
	if !checkVMIsReady(c) {
		return
	}
	v := getVMFromContext(c)
	ctx := v.Context()
	if ctx.NextIP() < ctx.LenInstr() {
		ip, opcode := v.Context().NextInstr()
		c.Printf("instruction pointer at %d (%s)\n", ip, opcode)
	} else {
		c.Println("execution has finished")
	}
}

func handleBreak(c *ishell.Context) {
	if !checkVMIsReady(c) {
		return
	}
	v := getVMFromContext(c)
	if len(c.Args) != 1 {
		c.Err(fmt.Errorf("%w: <ip>", ErrMissingParameter))
		return
	}
	n, err := strconv.Atoi(c.Args[0])
	if err != nil {
		c.Err(fmt.Errorf("%w: %v", ErrInvalidParameter, err))
		return
	}

	v.AddBreakPoint(n)
	c.Printf("breakpoint added at instruction %d\n", n)
}

func handleXStack(c *ishell.Context) {
	v := getVMFromContext(c)
	c.Println(v.Stack(c.Cmd.Name))
}

func handleLoadNEF(c *ishell.Context) {
	v := getVMFromContext(c)
	if len(c.Args) < 2 {
		c.Err(fmt.Errorf("%w: <file> <manifest>", ErrMissingParameter))
		return
	}
	if err := v.LoadFileWithFlags(c.Args[0], callflag.All); err != nil {
		c.Err(err)
		return
	}
	m, err := getManifestFromFile(c.Args[1])
	if err != nil {
		c.Err(err)
		return
	}
	c.Printf("READY: loaded %d instructions\n", v.Context().LenInstr())
	setManifestInContext(c, m)
	changePrompt(c, v)
}

func handleLoadBase64(c *ishell.Context) {
	v := getVMFromContext(c)
	if len(c.Args) < 1 {
		c.Err(fmt.Errorf("%w: <string>", ErrMissingParameter))
		return
	}
	b, err := base64.StdEncoding.DecodeString(c.Args[0])
	if err != nil {
		c.Err(fmt.Errorf("%w: %v", ErrInvalidParameter, err))
		return
	}
	v.LoadWithFlags(b, callflag.All)
	c.Printf("READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c, v)
}

func handleLoadHex(c *ishell.Context) {
	v := getVMFromContext(c)
	if len(c.Args) < 1 {
		c.Err(fmt.Errorf("%w: <string>", ErrMissingParameter))
		return
	}
	b, err := hex.DecodeString(c.Args[0])
	if err != nil {
		c.Err(fmt.Errorf("%w: %v", ErrInvalidParameter, err))
		return
	}
	v.LoadWithFlags(b, callflag.All)
	c.Printf("READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c, v)
}

func handleLoadGo(c *ishell.Context) {
	v := getVMFromContext(c)
	if len(c.Args) < 1 {
		c.Err(fmt.Errorf("%w: <file>", ErrMissingParameter))
		return
	}
	b, di, err := compiler.CompileWithDebugInfo(c.Args[0], nil)
	if err != nil {
		c.Err(err)
		return
	}

	// Don't perform checks, just load.
	m, err := di.ConvertToManifest(&compiler.Options{})
	if err != nil {
		c.Err(fmt.Errorf("can't create manifest: %w", err))
		return
	}
	setManifestInContext(c, m)

	v.LoadWithFlags(b, callflag.All)
	c.Printf("READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c, v)
}

func getManifestFromFile(name string) (*manifest.Manifest, error) {
	bs, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("%w: can't read manifest", ErrInvalidParameter)
	}

	var m manifest.Manifest
	if err := json.Unmarshal(bs, &m); err != nil {
		return nil, fmt.Errorf("%w: can't unmarshal manifest", ErrInvalidParameter)
	}
	return &m, nil
}

func handleRun(c *ishell.Context) {
	v := getVMFromContext(c)
	m := getManifestFromContext(c)
	if len(c.Args) != 0 {
		var (
			params     []stackitem.Item
			offset     int
			err        error
			runCurrent = c.Args[0] != "_"
		)

		params, err = parseArgs(c.Args[1:])
		if err != nil {
			c.Err(err)
			return
		}
		if runCurrent {
			md := m.ABI.GetMethod(c.Args[0], len(params))
			if md == nil {
				c.Err(fmt.Errorf("%w: method not found", ErrInvalidParameter))
				return
			}
			offset = md.Offset
		}
		for i := len(params) - 1; i >= 0; i-- {
			v.Estack().PushVal(params[i])
		}
		if runCurrent {
			v.Jump(v.Context(), offset)
			if initMD := m.ABI.GetMethod(manifest.MethodInit, 0); initMD != nil {
				v.Call(v.Context(), initMD.Offset)
			}
		}
	}
	runVMWithHandling(c, v)
	changePrompt(c, v)
}

// runVMWithHandling runs VM with handling errors and additional state messages.
func runVMWithHandling(c *ishell.Context, v *vm.VM) {
	err := v.Run()
	if err != nil {
		c.Err(err)
	}

	var message string
	switch {
	case v.HasFailed():
		message = "" // the error will be printed on return
	case v.HasHalted():
		message = v.Stack("estack")
	case v.AtBreakpoint():
		ctx := v.Context()
		if ctx.NextIP() < ctx.LenInstr() {
			i, op := ctx.NextInstr()
			message = fmt.Sprintf("at breakpoint %d (%s)", i, op)
		} else {
			message = "execution has finished"
		}
	}
	if message != "" {
		c.Println(message)
	}
}

func handleCont(c *ishell.Context) {
	if !checkVMIsReady(c) {
		return
	}
	v := getVMFromContext(c)
	runVMWithHandling(c, v)
	changePrompt(c, v)
}

func handleStep(c *ishell.Context) {
	var (
		n   = 1
		err error
	)

	if !checkVMIsReady(c) {
		return
	}
	v := getVMFromContext(c)
	if len(c.Args) > 0 {
		n, err = strconv.Atoi(c.Args[0])
		if err != nil {
			c.Err(fmt.Errorf("%w: %v", ErrInvalidParameter, err))
			return
		}
	}
	v.AddBreakPointRel(n)
	runVMWithHandling(c, v)
	changePrompt(c, v)
}

func handleStepInto(c *ishell.Context) {
	handleStepType(c, "into")
}

func handleStepOut(c *ishell.Context) {
	handleStepType(c, "out")
}

func handleStepOver(c *ishell.Context) {
	handleStepType(c, "over")
}

func handleStepType(c *ishell.Context, stepType string) {
	if !checkVMIsReady(c) {
		return
	}
	v := getVMFromContext(c)
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
		c.Err(err)
	} else {
		handleIP(c)
	}
	changePrompt(c, v)
}

func handleOps(c *ishell.Context) {
	if !checkVMIsReady(c) {
		return
	}
	v := getVMFromContext(c)
	out := bytes.NewBuffer(nil)
	v.PrintOps(out)
	c.Println(out.String())
}

func changePrompt(c ishell.Actions, v *vm.VM) {
	if v.Ready() && v.Context().NextIP() >= 0 && v.Context().NextIP() < v.Context().LenInstr() {
		c.SetPrompt(fmt.Sprintf("NEO-GO-VM %d > ", v.Context().NextIP()))
	} else {
		c.SetPrompt("NEO-GO-VM > ")
	}
}

// Run waits for user input from Stdin and executes the passed command.
func (c *VMCLI) Run() error {
	if c.printLogo {
		printLogo(c.shell)
	}
	c.shell.Run()
	return nil
}

func handleParse(c *ishell.Context) {
	res, err := Parse(c.Args)
	if err != nil {
		c.Err(err)
		return
	}
	c.Print(res)
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

func printLogo(c *ishell.Shell) {
	c.Print(logo)
	c.Println()
	c.Println()
}
