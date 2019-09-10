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

	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
	"gopkg.in/abiosoft/ishell.v2"
)

const vmKey = "vm"

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
> load /path/to/script.avm`,
		Func: handleLoadAVM,
	},
	{
		Name: "loadhex",
		Help: "Load a hex-encoded script string into the VM",
		LongHelp: `Usage: loadhex <string>
<string> is mandatory parameter, example:
> load 006166`,
		Func: handleLoadHex,
	},
	{
		Name: "loadgo",
		Help: "Compile and load a Go file into the VM",
		LongHelp: `Usage: loadhex <file>
<file> is mandatory parameter, example:
> load /path/to/file.go`,
		Func: handleLoadGo,
	},
	{
		Name: "run",
		Help: "Execute the current loaded script",
		LongHelp: `Usage: run [<operation> [<parameter>...]]

<operation> is an operation name, passed as a first parameter to Main() (and it
        can't be 'help' at the moment)
<parameter> is a parameter (can be repeated multiple times) specified
        as <type>:<value>, where type can be:
        'bool': supports 'false' and 'true' values
        'int': supports integers as values
        'string': supports strings as values (that are pushed as a byte array
                  values to the stack)

Passing parameters without operation is not supported. Parameters are packed
into array before they're passed to the script, so effectively 'run' only
supports contracts with signatures like this:
   func Main(operation string, args []interface{}) interface{}

Example:
> run put string:"Something to put"`,
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
		vm:    vm.New(0),
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
	ip, opcode := v.Context().CurrInstr()
	c.Printf("instruction pointer at %d (%s)\n", ip, opcode)
}

func handleBreak(c *ishell.Context) {
	if !checkVMIsReady(c) {
		return
	}
	v := getVMFromContext(c)
	if len(c.Args) != 1 {
		c.Err(errors.New("Missing parameter <ip>"))
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
	b, err := compiler.Compile(bytes.NewReader(fb), &compiler.Options{})
	if err != nil {
		c.Err(err)
		return
	}

	v.Load(b)
	c.Printf("READY: loaded %d instructions\n", v.Context().LenInstr())
	changePrompt(c, v)
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
	v.Run()
	changePrompt(c, v)
}

func handleCont(c *ishell.Context) {
	if !checkVMIsReady(c) {
		return
	}
	v := getVMFromContext(c)
	v.Run()
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
	v.AddBreakPointRel(n)
	v.Run()
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
	if v.Ready() && v.Context().IP()-1 >= 0 {
		c.SetPrompt(fmt.Sprintf("NEO-GO-VM %d > ", v.Context().IP()-1))
	} else {
		c.SetPrompt("NEO-GO-VM > ")
	}
}

// Run waits for user input from Stdin and executes the passed command.
func (c *VMCLI) Run() error {
	printLogo()
	c.shell.Run()
	return nil
}

func isMethodArg(s string) bool {
	return len(strings.Split(s, ":")) == 1
}

func parseArgs(args []string) ([]vm.StackItem, error) {
	items := make([]vm.StackItem, len(args))
	for i, arg := range args {
		typeAndVal := strings.Split(arg, ":")
		if len(typeAndVal) < 2 {
			return nil, errors.New("arguments need to be specified as <typ:val>")
		}

		typ := typeAndVal[0]
		value := typeAndVal[1]

		switch typ {
		case "bool":
			if value == "false" {
				items[i] = vm.NewBoolItem(false)
			} else if value == "true" {
				items[i] = vm.NewBoolItem(true)
			} else {
				return nil, errors.New("failed to parse bool parameter")
			}
		case "int":
			val, err := strconv.Atoi(value)
			if err != nil {
				return nil, err
			}
			items[i] = vm.NewBigIntegerItem(val)
		case "string":
			items[i] = vm.NewByteArrayItem([]byte(value))
		}
	}

	return items, nil
}

func printLogo() {
	logo := `
    _   ____________        __________      _    ____  ___
   / | / / ____/ __ \      / ____/ __ \    | |  / /  |/  /
  /  |/ / __/ / / / /_____/ / __/ / / /____| | / / /|_/ / 
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /_____/ |/ / /  / /  
/_/ |_/_____/\____/      \____/\____/      |___/_/  /_/   
`
	fmt.Print(logo)
	fmt.Println()
	fmt.Println()
}
