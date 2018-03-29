package vm

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/urfave/cli"
)

// NewCommand creates a new VM command.
func NewCommand() cli.Command {
	return cli.Command{
		Name:   "vm",
		Usage:  "start the virtual machine",
		Action: startVMPrompt,
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "debug, d"},
		},
	}
}

var vmCommands = map[string]int{
	"exit":   0,
	"load":   1,
	"run":    0,
	"step":   1,
	"resume": 0,
	"ip":     0,
	"stack":  0,
	"break":  1,
}

type VMCLI struct {
	vm *vm.VM
}

func (c *VMCLI) handleCommand(cmd string, args ...string) error {
	if n, ok := vmCommands[cmd]; !ok && n != len(args) {
		fmt.Println("unknown command")
	}

	switch cmd {
	case "exit":
		fmt.Println("Bye o/")
		os.Exit(0)

	case "ip":
		if ctx := c.vm.Context(); ctx != nil {
			ip, opcode := c.vm.Context().CurrInstr()
			fmt.Printf("instruction pointer at %d (%s)\n", ip, opcode)
		} else {
			fmt.Println("no program loaded!")
		}

	case "break":
		n, _ := strconv.Atoi(args[0])
		c.vm.AddBreakPoint(n)
		fmt.Printf("breakpoint added at instruction %d\n", n)

	case "stack":
		fmt.Println(c.vm.Stack())

	case "load":
		if err := c.vm.Load(args[0]); err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("READY")
		}

	case "run", "resume":
		c.vm.Run()

	case "step":
		if len(args) == 0 {
			c.vm.Step()
		} else {
			n, _ := strconv.Atoi(args[0])
			c.vm.AddBreakPointRel(n)
			c.vm.Run()
		}
	}

	return nil
}

func startVMPrompt(ctx *cli.Context) error {
	printLogo()

	c := &VMCLI{
		vm: vm.New(nil),
	}

	inputLoop(c)

	return nil
}

func inputLoop(c *VMCLI) {
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
	fmt.Println("\n\n")
}
