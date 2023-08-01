package util

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/cli/options"
	vmcli "github.com/nspcc-dev/neo-go/cli/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm"
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
					Name:      "sendtx",
					Usage:     "Send complete transaction stored in a context file",
					UsageText: "sendtx [-r <endpoint>] <file.in>",
					Description: `Sends the transaction from the given context file to the given RPC node if it's
   completely signed and ready. This command expects a ContractParametersContext
   JSON file for input, it can't handle binary (or hex- or base64-encoded)
   transactions.
`,
					Action: sendTx,
					Flags:  txDumpFlags,
				},
				{
					Name:      "txdump",
					Usage:     "Dump transaction stored in file",
					UsageText: "txdump [-r <endpoint>] <file.in>",
					Action:    txDump,
					Flags:     txDumpFlags,
				},
				{
					Name:      "ops",
					Usage:     "Pretty-print VM opcodes of the given base64- or hex- encoded script (base64 is checked first). If the input file is specified, then the script is taken from the file.",
					UsageText: "ops <base64/hex-encoded script> [-i path-to-file] [--hex]",
					Action:    handleOps,
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "in, i",
							Usage: "input file containing base64- or hex- encoded script representation",
						},
						cli.BoolFlag{
							Name:  "hex",
							Usage: "use hex encoding and do not check base64",
						},
					},
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

func handleOps(ctx *cli.Context) error {
	var (
		s   string
		err error
		b   []byte
	)
	in := ctx.String("in")
	if len(in) != 0 {
		b, err := os.ReadFile(in)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to read file: %w", err), 1)
		}
		s = string(b)
	} else {
		if !ctx.Args().Present() {
			return cli.NewExitError("missing script", 1)
		}
		s = ctx.Args()[0]
	}
	b, err = base64.StdEncoding.DecodeString(s)
	if err != nil || ctx.Bool("hex") {
		b, err = hex.DecodeString(s)
	}
	if err != nil {
		return cli.NewExitError("unknown encoding: base64 or hex are supported", 1)
	}
	v := vm.New()
	v.LoadScript(b)
	v.PrintOps(ctx.App.Writer)
	return nil
}
