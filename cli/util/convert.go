package util

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/txctx"
	vmcli "github.com/nspcc-dev/neo-go/cli/vm"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/urfave/cli/v2"
)

var neoFSFlags = append([]cli.Flag{
	&cli.StringFlag{
		Name:     "container",
		Aliases:  []string{"cid"},
		Usage:    "NeoFS container ID to upload objects to",
		Required: true,
		Action:   cmdargs.EnsureNotEmpty("container"),
	},
	&flags.AddressFlag{
		Name:  "address",
		Usage: "Address to use for signing the uploading and searching transactions in NeoFS",
	},
	&cli.UintFlag{
		Name:  "retries",
		Usage: "Maximum number of NeoFS node request retries",
		Value: neofs.MaxRetries,
		Action: func(context *cli.Context, u uint) error {
			if u < 1 {
				return cli.Exit("retries should be greater than 0", 1)
			}
			return nil
		},
	},
	&cli.UintFlag{
		Name:  "searchers",
		Usage: "Number of concurrent searches for objects",
		Value: 100,
	}}, options.NeoFSRPC...)

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
	uploadBinFlags := append([]cli.Flag{
		&cli.StringFlag{
			Name:   "block-attribute",
			Usage:  "Attribute key of the block object",
			Value:  neofs.DefaultBlockAttribute,
			Action: cmdargs.EnsureNotEmpty("block-attribute"),
		},
		&cli.StringFlag{
			Name:   "index-attribute",
			Usage:  "Attribute key of the index file object",
			Value:  neofs.DefaultIndexFileAttribute,
			Action: cmdargs.EnsureNotEmpty("index-attribute"),
		},
		&cli.UintFlag{
			Name:  "index-file-size",
			Usage: "Size of index file",
			Value: neofs.DefaultIndexFileSize,
		},
		&cli.UintFlag{
			Name:  "workers",
			Usage: "Number of workers to fetch and upload blocks concurrently",
			Value: 20,
		},
		options.Debug,
		options.ForceTimestampLogs,
	}, options.RPC...)
	uploadBinFlags = append(uploadBinFlags, options.Wallet...)
	uploadBinFlags = append(uploadBinFlags, neoFSFlags...)

	uploadStateFlags := append([]cli.Flag{
		&cli.StringFlag{
			Name:   "state-attribute",
			Usage:  "Attribute key of the state object",
			Value:  neofs.DefaultStateAttribute,
			Action: cmdargs.EnsureNotEmpty("state-attribute"),
		},
		options.Debug, options.ForceTimestampLogs, options.Config, options.ConfigFile, options.RelativePath,
	}, options.Wallet...)
	uploadStateFlags = append(uploadStateFlags, options.Network...)
	uploadStateFlags = append(uploadStateFlags, neoFSFlags...)
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
					UsageText: "canceltx -r <endpoint> --wallet <wallet> [--address <account>] [--wallet-config <path>] [--gas <gas>] [--await] <txid>",
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
				{
					Name:      "upload-bin",
					Usage:     "Fetch blocks from RPC node and upload them to the NeoFS container",
					UsageText: "neo-go util upload-bin --fs-rpc-endpoint <address1>[,<address2>[...]] --container <cid> --block-attribute block --index-attribute index --rpc-endpoint <node> [--timeout <time>] --wallet <wallet> [--wallet-config <config>] [--address <address>] [--workers <num>] [--searchers <num>] [--index-file-size <size>] [--retries <num>] [--debug]",
					Action:    uploadBin,
					Flags:     uploadBinFlags,
				},
				{
					Name:      "upload-state",
					Usage:     "Start the node, traverse MPT and upload MPT nodes to the NeoFS container at every StateSyncInterval number of blocks",
					UsageText: "neo-go util upload-state --fs-rpc-endpoint <address1>[,<address2>[...]] --container <cid> --state-attribute state --wallet <wallet> [--wallet-config <config>] [--address <address>] [--searchers <num>] [--retries <num>] [--debug] [--config-path path] [-p/-m/-t] [--config-file file] [--force-timestamp-logs]",
					Action:    uploadState,
					Flags:     uploadStateFlags,
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
