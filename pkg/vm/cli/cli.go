package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/CityOfZion/neo-go/pkg/vm"
)

// command describes a VM command.
type command struct {
	// number of minimun arguments the command needs.
	args int

	// description of the command.
	usage string
}

var commands = map[string]command{
	"help":   {0, "show available commands"},
	"exit":   {0, "exit the VM prompt"},
	"ip":     {0, "show the current instruction"},
	"break":  {1, "place a breakpoint (> break 1)"},
	"stack":  {0, "show stack details"},
	"load":   {1, "load a script into the VM (> load /path/to/script.avm)"},
	"run":    {0, "execute the current loaded script"},
	"resume": {0, "resume the current loaded script"},
	"step":   {0, "step (n) instruction in the program (> step 10)"},
}

// VMCLI object for interacting with the VM.
type VMCLI struct {
	vm *vm.VM
}

// New returns a new VMCLI object.
func New() *VMCLI {
	return &VMCLI{
		vm: vm.New(nil),
	}
}

func (c *VMCLI) handleCommand(cmd string, args ...string) {
	com, ok := commands[cmd]
	if !ok {
		fmt.Printf("unknown command (%s)\n", cmd)
		return
	}
	if len(args) < com.args {
		fmt.Printf("command (%s) takes at least %d arguments\n", cmd, com.args)
		return
	}

	switch cmd {
	case "help":
		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
		fmt.Fprintln(w, "COMMAND\tUSAGE")
		for name, details := range commands {
			fmt.Fprintf(w, "%s\t%s\n", name, details.usage)
		}
		w.Flush()
		fmt.Println()

	case "exit":
		fmt.Println("Bye!")
		os.Exit(0)

	case "ip":
		if !c.vm.Ready() {
			fmt.Println("no program loaded")
		} else {
			ip, opcode := c.vm.Context().CurrInstr()
			fmt.Printf("instruction pointer at %d (%s)\n", ip, opcode)
		}

	case "break":
		if !c.vm.Ready() {
			fmt.Println("no program loaded")
			return
		}

		n, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Printf("argument conversion error: %s\n", err)
			return
		}

		c.vm.AddBreakPoint(n)
		fmt.Printf("breakpoint added at instruction %d\n", n)

	case "stack":
		fmt.Println(c.vm.Stack())

	case "load":
		if err := c.vm.Load(args[0]); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("READY: loaded %d instructions\n", c.vm.Context().LenInstr())
		}

	case "run", "resume":
		c.vm.Run()

	case "step":
		if !c.vm.Ready() {
			fmt.Println("no program loaded")
			return
		}

		if len(args) == 0 {
			c.vm.Step()
		} else {
			n, _ := strconv.Atoi(args[0])
			c.vm.AddBreakPointRel(n)
			c.vm.Run()
		}
	}
}

// Run waits for user input from Stdin and executes the passed command.
func (c *VMCLI) Run() error {
	printLogo()
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("NEO-GO-VM > ")
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
