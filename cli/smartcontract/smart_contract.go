package smartcontract

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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

	src := ctx.Args()[0]
	c := compiler.New()
	if err := c.CompileSource(src); err != nil {
		return err
	}

	filename := strings.Split(src, ".")[0]
	filename = filename + ".avm"

	out := ctx.String("out")
	if len(out) > 0 {
		filename = out
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}

	hx := hex.EncodeToString(c.Buffer().Bytes())
	fmt.Println(hx)

	_, err = io.Copy(f, c.Buffer())
	return err
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
