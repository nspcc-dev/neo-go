package smartcontract

import (
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
	"github.com/urfave/cli"
)

const (
	ErrNoInput = "Input file is mandatory and should be passed using -i flag."
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
						Name:  "in, i",
						Usage: "Input file for the smart contract to be compiled",
					},
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
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "in, i",
						Usage: "Input file for the smart contract",
					},
				},
			},
		},
	}
}

func contractCompile(ctx *cli.Context) error {
	src := ctx.String("in")
	if len(src) == 0 {
		return cli.NewExitError(ErrNoInput, 1)
	}

	o := &compiler.Options{
		Outfile: ctx.String("out"),
		Debug:   true,
	}

	if err := compiler.CompileAndSave(src, o); err != nil {
		return cli.NewExitError(err, 1)
	}
	return nil
}

func contractDumpOpcode(ctx *cli.Context) error {
	src := ctx.String("in")
	if len(src) == 0 {
		return cli.NewExitError(ErrNoInput, 1)
	}
	if err := compiler.DumpOpcode(src); err != nil {
		return cli.NewExitError(err, 1)
	}
	return nil
}
