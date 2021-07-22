package query

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/urfave/cli"
)

// NewCommands returns 'query' command.
func NewCommands() []cli.Command {
	queryTxFlags := append([]cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "Output full tx info and execution logs",
		},
	}, options.RPC...)
	return []cli.Command{{
		Name:  "query",
		Usage: "Query data from RPC node",
		Subcommands: []cli.Command{
			{
				Name:   "candidates",
				Usage:  "Get candidates and votes",
				Action: queryCandidates,
				Flags:  options.RPC,
			},
			{
				Name:   "committee",
				Usage:  "Get committee list",
				Action: queryCommittee,
				Flags:  options.RPC,
			},
			{
				Name:   "tx",
				Usage:  "Query transaction status",
				Action: queryTx,
				Flags:  queryTxFlags,
			},
		},
	}}
}

func queryTx(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return cli.NewExitError("Transaction hash is missing", 1)
	}

	txHash, err := util.Uint256DecodeStringLE(strings.TrimPrefix(args[0], "0x"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Invalid tx hash: %s", args[0]), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	txOut, err := c.GetRawTransactionVerbose(txHash)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	var res *result.ApplicationLog
	if !txOut.Blockhash.Equals(util.Uint256{}) {
		res, err = c.GetApplicationLog(txHash, nil)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	dumpApplicationLog(ctx, res, txOut)
	return nil
}

func dumpApplicationLog(ctx *cli.Context, res *result.ApplicationLog, tx *result.TransactionOutputRaw) {
	verbose := ctx.Bool("verbose")
	buf := bytes.NewBuffer(nil)

	// Ignore the errors below because `Write` to buffer doesn't return error.
	tw := tabwriter.NewWriter(buf, 0, 4, 4, '\t', 0)
	_, _ = tw.Write([]byte("Hash:\t" + tx.Hash().StringLE() + "\n"))
	_, _ = tw.Write([]byte(fmt.Sprintf("OnChain:\t%t\n", res != nil)))
	if res == nil {
		_, _ = tw.Write([]byte("ValidUntil:\t" + strconv.FormatUint(uint64(tx.ValidUntilBlock), 10) + "\n"))
	} else {
		_, _ = tw.Write([]byte("BlockHash:\t" + tx.Blockhash.StringLE() + "\n"))
		if len(res.Executions) != 1 {
			_, _ = tw.Write([]byte("Success:\tunknown (no execution data)\n"))
		} else {
			_, _ = tw.Write([]byte(fmt.Sprintf("Success:\t%t\n", res.Executions[0].VMState == vm.HaltState)))
		}
	}
	if verbose {
		for _, sig := range tx.Signers {
			_, _ = tw.Write([]byte(fmt.Sprintf("Signer:\t%s (%s)",
				address.Uint160ToString(sig.Account),
				sig.Scopes) + "\n"))
		}
		_, _ = tw.Write([]byte("SystemFee:\t" + fixedn.Fixed8(tx.SystemFee).String() + " GAS\n"))
		_, _ = tw.Write([]byte("NetworkFee:\t" + fixedn.Fixed8(tx.NetworkFee).String() + " GAS\n"))
		_, _ = tw.Write([]byte("Script:\t" + base64.StdEncoding.EncodeToString(tx.Script) + "\n"))
		if res != nil {
			for _, e := range res.Executions {
				if e.VMState != vm.HaltState {
					_, _ = tw.Write([]byte("Exception:\t" + e.FaultException + "\n"))
				}
			}
		}
	}
	_ = tw.Flush()
	fmt.Fprint(ctx.App.Writer, buf.String())
}

func queryCandidates(ctx *cli.Context) error {
	var err error

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	vals, err := c.GetNextBlockValidators()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	comm, err := c.GetCommittee()
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	sort.Slice(vals, func(i, j int) bool {
		if vals[i].Active != vals[j].Active {
			return vals[i].Active
		}
		if vals[i].Votes != vals[j].Votes {
			return vals[i].Votes > vals[j].Votes
		}
		return vals[i].PublicKey.Cmp(&vals[j].PublicKey) == -1
	})
	buf := bytes.NewBuffer(nil)
	tw := tabwriter.NewWriter(buf, 0, 2, 2, ' ', 0)
	_, _ = tw.Write([]byte("Key\tVotes\tCommittee\tConsensus\n"))
	for _, val := range vals {
		_, _ = tw.Write([]byte(fmt.Sprintf("%s\t%d\t%t\t%t\n", hex.EncodeToString(val.PublicKey.Bytes()), val.Votes, comm.Contains(&val.PublicKey), val.Active)))
	}
	_ = tw.Flush()
	fmt.Fprint(ctx.App.Writer, buf.String())
	return nil
}

func queryCommittee(ctx *cli.Context) error {
	var err error

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	comm, err := c.GetCommittee()
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	for _, k := range comm {
		fmt.Fprintln(ctx.App.Writer, hex.EncodeToString(k.Bytes()))
	}
	return nil
}
