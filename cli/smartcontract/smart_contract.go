package smartcontract

import (
	"errors"

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
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "out, o",
						Usage: "Output of the compiled contract",
					},
				},
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
	if len(ctx.Args()) == 0 {
		return errors.New("not enough arguments")
	}

	o := &compiler.Options{
		Outfile: ctx.String("out"),
		Debug:   true,
	}

	src := ctx.Args()[0]
	return compiler.CompileAndSave(src, o)
}

func contractDumpOpcode(ctx *cli.Context) error {
	if len(ctx.Args()) == 0 {
		return errors.New("not enough arguments")
	}
	src := ctx.Args()[0]
	return compiler.DumpOpcode(src)
}
