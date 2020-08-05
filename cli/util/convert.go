package util

import (
	"fmt"

	vmcli "github.com/nspcc-dev/neo-go/pkg/vm/cli"
	"github.com/urfave/cli"
)

func NewCommands() []cli.Command {
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
			},
		},
	}
}

func handleParse(ctx *cli.Context) error {
	res, err := vmcli.Parse(ctx.Args())
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	fmt.Print(res)
	return nil
}
