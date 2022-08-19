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

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/neo"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
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
				Name:      "candidates",
				Usage:     "Get candidates and votes",
				UsageText: "neo-go query candidates -r endpoint [-s timeout]",
				Action:    queryCandidates,
				Flags:     options.RPC,
			},
			{
				Name:      "committee",
				Usage:     "Get committee list",
				UsageText: "neo-go query committee -r endpoint [-s timeout]",
				Action:    queryCommittee,
				Flags:     options.RPC,
			},
			{
				Name:      "height",
				Usage:     "Get node height",
				UsageText: "neo-go query height -r endpoint [-s timeout]",
				Action:    queryHeight,
				Flags:     options.RPC,
			},
			{
				Name:      "tx",
				Usage:     "Query transaction status",
				UsageText: "neo-go query tx <hash> -r endpoint [-s timeout] [-v]",
				Action:    queryTx,
				Flags:     queryTxFlags,
			},
			{
				Name:      "voter",
				Usage:     "Print NEO holder account state",
				UsageText: "neo-go query voter <address> -r endpoint [-s timeout]",
				Action:    queryVoter,
				Flags:     options.RPC,
			},
		},
	}}
}

func queryTx(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return cli.NewExitError("Transaction hash is missing", 1)
	} else if len(args) > 1 {
		return cli.NewExitError("only one transaction hash is accepted", 1)
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

	DumpApplicationLog(ctx, res, &txOut.Transaction, &txOut.TransactionMetadata, ctx.Bool("verbose"))
	return nil
}

func DumpApplicationLog(
	ctx *cli.Context,
	res *result.ApplicationLog,
	tx *transaction.Transaction,
	txMeta *result.TransactionMetadata,
	verbose bool) {
	buf := bytes.NewBuffer(nil)

	// Ignore the errors below because `Write` to buffer doesn't return error.
	tw := tabwriter.NewWriter(buf, 0, 4, 4, '\t', 0)
	_, _ = tw.Write([]byte("Hash:\t" + tx.Hash().StringLE() + "\n"))
	_, _ = tw.Write([]byte(fmt.Sprintf("OnChain:\t%t\n", res != nil)))
	if res == nil {
		_, _ = tw.Write([]byte("ValidUntil:\t" + strconv.FormatUint(uint64(tx.ValidUntilBlock), 10) + "\n"))
	} else {
		if txMeta != nil {
			_, _ = tw.Write([]byte("BlockHash:\t" + txMeta.Blockhash.StringLE() + "\n"))
		}
		if len(res.Executions) != 1 {
			_, _ = tw.Write([]byte("Success:\tunknown (no execution data)\n"))
		} else {
			_, _ = tw.Write([]byte(fmt.Sprintf("Success:\t%t\n", res.Executions[0].VMState == vmstate.Halt)))
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
		v := vm.New()
		v.Load(tx.Script)
		v.PrintOps(tw)
		if res != nil {
			for _, e := range res.Executions {
				if e.VMState != vmstate.Halt {
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

	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	vals, err := c.GetCandidates()
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

	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}

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

func queryHeight(ctx *cli.Context) error {
	var err error

	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	blockCount, err := c.GetBlockCount()
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	blockHeight := blockCount - 1 // GetBlockCount returns block count (including 0), not the highest block index.

	fmt.Fprintf(ctx.App.Writer, "Latest block: %d\n", blockHeight)

	stateHeight, err := c.GetStateHeight()
	if err == nil { // We can be talking to a node without getstateheight request support.
		fmt.Fprintf(ctx.App.Writer, "Validated state: %d\n", stateHeight.Validated)
	}

	return nil
}

func queryVoter(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return cli.NewExitError("No address specified", 1)
	} else if len(args) > 1 {
		return cli.NewExitError("this command only accepts one address", 1)
	}

	addr, err := flags.ParseAddress(args[0])
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("wrong address: %s", args[0]), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()
	c, exitErr := options.GetRPCClient(gctx, ctx)
	if exitErr != nil {
		return exitErr
	}

	neoToken := neo.NewReader(invoker.New(c, nil))

	st, err := neoToken.GetAccountState(addr)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if st == nil {
		st = new(state.NEOBalance)
	}
	dec, err := neoToken.Decimals()
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to get decimals: %w", err), 1)
	}
	voted := "null"
	if st.VoteTo != nil {
		voted = fmt.Sprintf("%s (%s)", hex.EncodeToString(st.VoteTo.Bytes()), address.Uint160ToString(st.VoteTo.GetScriptHash()))
	}
	fmt.Fprintf(ctx.App.Writer, "\tVoted: %s\n", voted)
	fmt.Fprintf(ctx.App.Writer, "\tAmount : %s\n", fixedn.ToString(&st.Balance, int(dec)))
	fmt.Fprintf(ctx.App.Writer, "\tBlock: %d\n", st.BalanceHeight)
	return nil
}
