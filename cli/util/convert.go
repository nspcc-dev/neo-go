package util

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/txctx"
	vmcli "github.com/nspcc-dev/neo-go/cli/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/urfave/cli/v2"
)

// NewCommands returns util commands for neo-go CLI.
func NewCommands() []*cli.Command {
	// By default, RPC flag is required. sendtx and txdump may be called without provided rpc-endpoint.
	rpcFlagOriginal, _ := options.RPC[0].(*cli.StringFlag)
	rpcFlag := *rpcFlagOriginal
	rpcFlag.Required = false
	txDumpFlags := append([]cli.Flag{&rpcFlag}, options.RPC[1:]...)
	txSendFlags := append(txDumpFlags, txctx.AwaitFlag)
	txCancelFlags := append([]cli.Flag{
		&flags.AddressFlag{
			Name:    "address",
			Aliases: []string{"a"},
			Usage:   "Address to use as conflicting transaction signee (and gas source)",
		},
		txctx.GasFlag,
		txctx.AwaitFlag,
	}, options.RPC...)
	txCancelFlags = append(txCancelFlags, options.Wallet...)
	return []*cli.Command{
		{
			Name:  "util",
			Usage: "Various helper commands",
			Subcommands: []*cli.Command{
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
					UsageText: "sendtx [-r <endpoint>] [--await] <file.in>",
					Description: `Sends the transaction from the given context file to the given RPC node if it's
   completely signed and ready. This command expects a ContractParametersContext
   JSON file for input, it can't handle binary (or hex- or base64-encoded)
   transactions. If the --await flag is included, the command waits for the
   transaction to be included in a block before exiting.
`,
					Action: sendTx,
					Flags:  txSendFlags,
				},
				{
					Name:      "canceltx",
					Usage:     "Cancel transaction by sending conflicting transaction",
					UsageText: "canceltx -r <endpoint> --wallet <wallet> [--account <account>] [--wallet-config <path>] [--gas <gas>] [--await] <txid>",
					Description: `Aims to prevent a transaction from being added to the blockchain by dispatching a more 
   prioritized conflicting transaction to the specified RPC node. The input for this command should 
   be the transaction hash. If another account is not specified, the conflicting transaction is 
   automatically generated and signed by the default account in the wallet. If the target transaction 
   is in the memory pool of the provided RPC node, the NetworkFee value of the conflicting transaction 
   is set to the target transaction's NetworkFee value plus one (if it's sufficient for the 
   conflicting transaction itself), the ValidUntilBlock value of the conflicting transaction is set to the
   target transaction's ValidUntilBlock value. If the target transaction is not in the memory pool, standard 
   NetworkFee calculations are performed based on the calculatenetworkfee RPC request. If the --gas 
   flag is included, the specified value is added to the resulting conflicting transaction network fee 
   in both scenarios. When the --await flag is included, the command waits for one of the conflicting 
   or target transactions to be included in a block.
`,
					Action: cancelTx,
					Flags:  txCancelFlags,
				},
				{
					Name:      "txdump",
					Usage:     "Dump transaction stored in file",
					UsageText: "txdump [-r <endpoint>] <file.in>",
					Action:    txDump,
					Flags:     txDumpFlags,
					Description: `Dumps the transaction from the given parameter context file to 
   the output. This command expects a ContractParametersContext JSON file for input, it can't handle
   binary (or hex- or base64-encoded) transactions. If --rpc-endpoint flag is specified the result 
   of the given script after running it true the VM will be printed. Otherwise only transaction will
   be printed.
`,
				},
				{
					Name:      "ops",
					Usage:     "Pretty-print VM opcodes of the given base64- or hex- encoded script (base64 is checked first). If the input file is specified, then the script is taken from the file.",
					UsageText: "ops [-i path-to-file] [--hex] <base64/hex-encoded script>",
					Action:    handleOps,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "in",
							Aliases: []string{"i"},
							Usage:   "Input file containing base64- or hex- encoded script representation",
						},
						&cli.BoolFlag{
							Name:  "hex",
							Usage: "Use hex encoding and do not check base64",
						},
					},
				},
			},
		},
	}
}

func handleParse(ctx *cli.Context) error {
	res, err := vmcli.Parse(ctx.Args().Slice())
	if err != nil {
		return cli.Exit(err, 1)
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
			return cli.Exit(fmt.Errorf("failed to read file: %w", err), 1)
		}
		s = string(b)
	} else {
		if !ctx.Args().Present() {
			return cli.Exit("missing script", 1)
		}
		s = ctx.Args().Slice()[0]
	}
	b, err = base64.StdEncoding.DecodeString(s)
	if err != nil || ctx.Bool("hex") {
		b, err = hex.DecodeString(s)
	}
	if err != nil {
		return cli.Exit("unknown encoding: base64 or hex are supported", 1)
	}
	v := vm.New()
	v.LoadScript(b)
	v.PrintOps(ctx.App.Writer)
	return nil
}
