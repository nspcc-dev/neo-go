package vm

import (
	vmcli "github.com/CityOfZion/neo-go/pkg/vm/cli"
	"github.com/urfave/cli"
)

// NewCommands returns 'vm' command.
func NewCommands() []cli.Command {
	return []cli.Command{{
		Name:   "vm",
		Usage:  "start the virtual machine",
		Action: startVMPrompt,
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "debug, d"},
		},
	}}
}

func startVMPrompt(ctx *cli.Context) error {
	p := vmcli.New()
	return p.Run()
}
