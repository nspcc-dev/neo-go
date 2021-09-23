package util

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/options"
	vmcli "github.com/nspcc-dev/neo-go/pkg/vm/cli"
	"github.com/urfave/cli"
)

// NewCommands returns util commands for neo-go CLI.
func NewCommands() []cli.Command {
	txDumpFlags := append([]cli.Flag{}, options.RPC...)
	return []cli.Command{
		{
			Name:  "util",
			Usage: "Various helper commands",
			Subcommands: []cli.Command{
				{
					Name:  "convert",
					Usage: "Convert provided argument into other possible formats",
					UsageText: `convert <arg>

<arg> is an argument which is tried to be interpreted as an item of different types
        and converted to other formats. Strings are escaped and output in quotes.`,
					Action: handleParse,
				},
				{
					Name:      "txdump",
					Usage:     "Dump transaction stored in file",
					UsageText: "txdump [-r <endpoint>] <file.in>",
					Action:    txDump,
					Flags:     txDumpFlags,
				},
			},
		},
	}
}

func handleParse(ctx *cli.Context) error {
	res, err := vmcli.Parse(ctx.Args())
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	fmt.Fprint(ctx.App.Writer, res)
	return nil
}
