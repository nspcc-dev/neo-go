package cli

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"gopkg.in/abiosoft/ishell.v2"
)

const (
	vmKey      = "vm"
	boolType   = "bool"
	boolFalse  = "false"
	boolTrue   = "true"
	intType    = "int"
	stringType = "string"
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
		Name: "loadavm",
		Help: "Load an avm script into the VM",
		LongHelp: `Usage: loadavm <file>
<file> is mandatory parameter, example:
> loadavm /path/to/script.avm`,
		Func: handleLoadAVM,
	},
	{
		Name: "loadhex",
		Help: "Load a hex-encoded script string into the VM",
		LongHelp: `Usage: loadhex <string>
<string> is mandatory parameter, example:
> loadhex 006166`,
		Func: handleLoadHex,
	},
	{
		Name: "loadgo",
		Help: "Compile and load a Go file into the VM",
		LongHelp: `Usage: loadgo <file>
<file> is mandatory parameter, example:
> loadgo /path/to/file.go`,
		Func: handleLoadGo,
	},
	{
		Name: "push",
		Help: "Push given item to the estack",
		LongHelp: `Usage: push <parameter>
<parameter> is mandatory, example:
> push methodstring

See run command help for parameter syntax.`,
		Func: handlePush,
	},
	{
		Name: "run",
		Help: "Execute the current loaded script",
		LongHelp: `Usage: run [<operation> [<parameter>...]]

<operation> is an operation name, passed as a first parameter to Main() (and it
        can't be 'help' at the moment)
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

Passing parameters without operation is not supported. Parameters are packed
into array before they're passed to the script, so effectively 'run' only
supports contracts with signatures like this:
   func Main(operation string, args []interface{}) interface{}

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

// VMCLI object for interacting with the VM.
type VMCLI struct {
	vm    *vm.VM
	shell *ishell.Shell
}

// New returns a new VMCLI object.
func New() *VMCLI {
	vmcli := VMCLI{
		vm:    vm.New(),
		shell: ishell.New(),
	}
	vmcli.shell.Set(vmKey, vmcli.vm)
	for _, c := range commands {
		vmcli.shell.AddCmd(c)
	}
	changePrompt(vmcli.shell, vmcli.vm)
	return &vmcli
}

func getVMFromContext(c *ishell.Context) *vm.VM {
	return c.Get(vmKey).(*vm.VM)
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
	os.Exit(0)
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
		c.Err(errors.New("missing parameter <ip>"))
	}
	n, err := strconv.Atoi(c.Args[0])
	if err != nil {
		c.Err(fmt.Errorf("argument conversion error: %s", err))
		return
	}

	v.AddBreakPoint(n)
	c.Printf("breakpoint added at instruction %d\n", n)
}

func handleXStack(c *ishell.Context) {
	v := getVMFromContext(c)
	c.Println(v.Stack(c.Cmd.Name))
}

func handleLoadAVM(c *ishell.Context) {
	v := getVMFromContext(c)
	if err := v.LoadFile(c.Args[0]); err != nil {
		c.Err(err)
	} else {
		c.Printf("READY: loaded %d instructions\n", v.Context().LenInstr())
	}
	changePrompt(c, v)
}

func handleLoadHex(c *ishell.Context) {
	v := getVMFromContext(c)
	b, err := hex.DecodeString(c.Args[0])
	if err != nil {
		c.Err(err)
		return
	}
	v.Load(b)
	c.Printf("READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c, v)
}

func handleLoadGo(c *ishell.Context) {
	v := getVMFromContext(c)
	fb, err := ioutil.ReadFile(c.Args[0])
	if err != nil {
		c.Err(err)
		return
	}
	b, err := compiler.Compile(bytes.NewReader(fb))
	if err != nil {
		c.Err(err)
		return
	}

	v.Load(b)
	c.Printf("READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c, v)
}

func handlePush(c *ishell.Context) {
	v := getVMFromContext(c)
	if len(c.Args) == 0 {
		c.Err(errors.New("missing parameter"))
		return
	}
	param, err := parseArg(c.Args[0])
	if err != nil {
		c.Err(err)
		return
	}
	v.Estack().PushVal(param)
}

func handleRun(c *ishell.Context) {
	v := getVMFromContext(c)
	if len(c.Args) != 0 {
		var (
			method []byte
			params []vm.StackItem
			err    error
		)
		method = []byte(c.Args[0])
		params, err = parseArgs(c.Args[1:])
		if err != nil {
			c.Err(err)
			return
		}
		v.LoadArgs(method, params)
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
	checkAndPrintVMState(c, v)
}

// checkAndPrintVMState checks VM state and outputs it to the user if it's
// failed, halted or at breakpoint. No message is printed if VM is running
// normally.
func checkAndPrintVMState(c *ishell.Context, v *vm.VM) {
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
			c.Err(fmt.Errorf("argument conversion error: %s", err))
			return
		}
	}
	for i := 0; i < n; i++ {
		err := v.Step()
		if err != nil {
			c.Err(err)
			return
		}
		checkAndPrintVMState(c, v)
	}
	ctx := v.Context()
	i, op := ctx.CurrInstr()
	c.Printf("at %d (%s)\n", i, op.String())
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
	v.PrintOps()
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
	printLogo(c.shell)
	c.shell.Run()
	return nil
}

func isMethodArg(s string) bool {
	return len(strings.Split(s, ":")) == 1
}

func parseArgs(args []string) ([]vm.StackItem, error) {
	items := make([]vm.StackItem, len(args))
	for i, arg := range args {
		item, err := parseArg(arg)
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return items, nil
}

func parseArg(arg string) (vm.StackItem, error) {
	var typ, value string
	var item vm.StackItem

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
			item = vm.NewBoolItem(false)
		} else if value == boolTrue {
			item = vm.NewBoolItem(true)
		} else {
			return nil, errors.New("failed to parse bool parameter")
		}
	case intType:
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		item = vm.NewBigIntegerItem(val)
	case stringType:
		item = vm.NewByteArrayItem([]byte(value))
	}
	return item, nil
}

func printLogo(c *ishell.Shell) {
	logo := `
    _   ____________        __________      _    ____  ___
   / | / / ____/ __ \      / ____/ __ \    | |  / /  |/  /
  /  |/ / __/ / / / /_____/ / __/ / / /____| | / / /|_/ / 
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /_____/ |/ / /  / /  
/_/ |_/_____/\____/      \____/\____/      |___/_/  /_/   
`
	c.Print(logo)
	c.Println()
	c.Println()
}
