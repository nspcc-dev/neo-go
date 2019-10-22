package vm

import (
	"errors"
	"io/ioutil"

	"github.com/CityOfZion/neo-go/pkg/vm"
	vmcli "github.com/CityOfZion/neo-go/pkg/vm/cli"
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
		Subcommands: []cli.Command{
			{
				Name:   "inspect",
				Usage:  "dump instructions of the avm file given",
				Action: inspect,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "in, i",
						Usage: "input file of the program (AVM)",
					},
				},
			},
		},
	}
}

func startVMPrompt(ctx *cli.Context) error {
	p := vmcli.New()
	return p.Run()
}

func inspect(ctx *cli.Context) error {
	avm := ctx.String("in")
	if len(avm) == 0 {
		return cli.NewExitError(errors.New("no input file given"), 1)
	}
	b, err := ioutil.ReadFile(avm)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	v := vm.New()
	v.LoadScript(b)
	v.PrintOps()
	return nil
}
