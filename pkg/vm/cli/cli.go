package cli

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
)

// command describes a VM command.
type command struct {
	// number of minimun arguments the command needs.
	args int
	// description of the command.
	usage string
	// whether the VM needs to be "ready" to execute this command.
	ready bool
}

var commands = map[string]command{
	"help":    {0, "show available commands", false},
	"exit":    {0, "exit the VM prompt", false},
	"ip":      {0, "show the current instruction", true},
	"break":   {1, "place a breakpoint (> break 1)", true},
	"estack":  {0, "show evaluation stack details", false},
	"astack":  {0, "show alt stack details", false},
	"istack":  {0, "show invocation stack details", false},
	"loadavm": {1, "load an avm script into the VM (> load /path/to/script.avm)", false},
	"loadhex": {1, "load a hex string into the VM (> loadhex 006166 )", false},
	"loadgo":  {1, "compile and load a .go file into the VM (> load /path/to/file.go)", false},
	"run":     {0, "execute the current loaded script", true},
	"cont":    {0, "continue execution of the current loaded script", true},
	"step":    {0, "step (n) instruction in the program (> step 10)", true},
	"ops":     {0, "show the opcodes of the current loaded program", true},
}

// VMCLI object for interacting with the VM.
type VMCLI struct {
	vm *vm.VM
}

// New returns a new VMCLI object.
func New() *VMCLI {
	return &VMCLI{
		vm: vm.New(0),
	}
}

func (c *VMCLI) handleCommand(cmd string, args ...string) {
	com, ok := commands[cmd]
	if !ok {
		fmt.Printf("unknown command (%s)\n", cmd)
		return
	}
	if (len(args) < com.args || len(args) > com.args) && cmd != "run" {
		fmt.Printf("command (%s) takes at least %d arguments\n", cmd, com.args)
		return
	}
	if com.ready && !c.vm.Ready() {
		fmt.Println("VM is not ready: no program loaded")
		return
	}

	switch cmd {
	case "help":
		printHelp()

	case "exit":
		fmt.Println("Bye!")
		os.Exit(0)

	case "ip":
		ip, opcode := c.vm.Context().CurrInstr()
		fmt.Printf("instruction pointer at %d (%s)\n", ip, opcode)

	case "break":
		n, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Printf("argument conversion error: %s\n", err)
			return
		}

		c.vm.AddBreakPoint(n)
		fmt.Printf("breakpoint added at instruction %d\n", n)

	case "estack", "istack", "astack":
		fmt.Println(c.vm.Stack(cmd))

	case "loadavm":
		if err := c.vm.LoadFile(args[0]); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("READY: loaded %d instructions\n", c.vm.Context().LenInstr())
		}

	case "loadhex":
		b, err := hex.DecodeString(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		c.vm.Load(b)
		fmt.Printf("READY: loaded %d instructions\n", c.vm.Context().LenInstr())

	case "loadgo":
		fb, err := ioutil.ReadFile(args[0])
		if err != nil {
			fmt.Println(err)
			return
		}
		b, err := compiler.Compile(bytes.NewReader(fb), &compiler.Options{})
		if err != nil {
			fmt.Println(err)
			return
		}
		c.vm.Load(b)
		fmt.Printf("READY: loaded %d instructions\n", c.vm.Context().LenInstr())

	case "run":
		var (
			method []byte
			params []vm.StackItem
			err    error
			start  int
		)

		if len(args) == 0 {
			c.vm.Run()
		} else {
			if isMethodArg(args[0]) {
				method = []byte(args[0])
				start = 1
			}
			params, err = parseArgs(args[start:])
			if err != nil {
				fmt.Println(err)
				return
			}
		}
		c.vm.LoadArgs(method, params)
		c.vm.Run()

	case "cont":
		c.vm.Run()

	case "step":
		var (
			n   = 1
			err error
		)
		if len(args) > 0 {
			n, err = strconv.Atoi(args[0])
			if err != nil {
				fmt.Printf("argument conversion error: %s\n", err)
				return
			}
		}
		c.vm.AddBreakPointRel(n)
		c.vm.Run()

	case "ops":
		c.vm.PrintOps()
	}
}

// Run waits for user input from Stdin and executes the passed command.
func (c *VMCLI) Run() error {
	printLogo()
	reader := bufio.NewReader(os.Stdin)
	for {
		if c.vm.Ready() && c.vm.Context().IP()-1 >= 0 {
			fmt.Printf("NEO-GO-VM %d > ", c.vm.Context().IP()-1)
		} else {
			fmt.Print("NEO-GO-VM > ")
		}
		input, _ := reader.ReadString('\n')
		input = strings.Trim(input, "\n")
		if len(input) != 0 {
			parts := strings.Split(input, " ")
			cmd := parts[0]
			args := []string{}
			if len(parts) > 1 {
				args = parts[1:]
			}
			c.handleCommand(cmd, args...)
		}
	}
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

func printHelp() {
	names := make([]string, len(commands))
	i := 0
	for name, _ := range commands {
		names[i] = name
		i++
	}
	sort.Strings(names)

	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprintln(w, "COMMAND\tUSAGE")
	for _, name := range names {
		fmt.Fprintf(w, "%s\t%s\n", name, commands[name].usage)
	}
	w.Flush()
	fmt.Println()
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
