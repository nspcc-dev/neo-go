package smartcontract

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
	"github.com/urfave/cli"
)

// NewCommand returns a new contract command.
func NewCommand() cli.Command {
	return cli.Command{
		Name:  "contract",
		Usage: "compile - debug - deploy smart contracts",
		Subcommands: []cli.Command{
			{
				Name:   "compile",
				Usage:  "compile a smart contract to a .avm file",
				Action: contractCompile,
			},
			{
				Name:   "opdump",
				Usage:  "dump the opcode of a .go file",
				Action: contractDumpOpcode,
			},
		},
	}
}

func contractCompile(ctx *cli.Context) error {
	fmt.Println("compile")
	return nil
}

func contractDumpOpcode(ctx *cli.Context) error {
	src := ctx.Args()[0]

	c := compiler.New()
	if err := c.CompileSource(src); err != nil {
		return err
	}
	c.DumpOpcode()
	return nil
}
