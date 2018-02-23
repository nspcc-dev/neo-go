package smartcontract

import (
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
	"github.com/urfave/cli"
)

const (
	ErrNoArgument = "Not enough arguments. Expected the location of the file to be compiled as an argument."
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
	if ctx.NArg() == 0 {
		return cli.NewExitError(ErrNoArgument, 1)
	}

	o := &compiler.Options{
		Outfile: ctx.String("out"),
		Debug:   true,
	}

	src := ctx.Args()[0]
	err := compiler.CompileAndSave(src, o)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	return nil
}

func contractDumpOpcode(ctx *cli.Context) error {
	if ctx.NArg() == 0 {
		return cli.NewExitError(ErrNoArgument, 1)
	}
	src := ctx.Args()[0]
	err := compiler.DumpOpcode(src)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	return nil
}
