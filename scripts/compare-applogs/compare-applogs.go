package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/scripts/rpcutil"
	"github.com/urfave/cli/v2"
)

func cliMain(c *cli.Context) error {
	const reportInterval = 500_000

	a := c.Args().Get(0)
	b := c.Args().Get(1)
	if a == "" || b == "" {
		return errors.New("usage: compare-applogs RPC_A RPC_B [-s start] [-e end]")
	}

	ca, ha, err := rpcutil.InitClient(a, "A")
	if err != nil {
		return err
	}
	cb, hb, err := rpcutil.InitClient(b, "B")
	if err != nil {
		return err
	}
	refHeight, err := rpcutil.CheckHeights(ha, hb, c.Bool(rpcutil.IgnoreHeightFlag.Name))
	if err != nil {
		return err
	}

	if err := rpcutil.CompareNetwork(ca, cb); err != nil {
		return err
	}

	end := uint32(c.Uint("end"))
	if end == 0 {
		end = refHeight
	} else if end > refHeight {
		return fmt.Errorf("end %d exceeds chain height: A has %d blocks, B has %d blocks", end, ha, hb)
	}

	start := uint32(c.Uint("start"))
	if start >= end {
		return fmt.Errorf("invalid block range: [%d, %d)", start, end)
	}

	for i := start; i < end; i++ {
		if (i-start)%reportInterval == 0 {
			fmt.Printf("Processing blocks %d-%d\n", i, min(i+reportInterval, end))
		}
		blk, err := ca.GetBlockByIndex(i)
		if err != nil {
			return fmt.Errorf("can't get block %d from A: %w", i, err)
		}
		for _, tx := range blk.Transactions {
			da, err := rpcutil.GetApplicationLog(ca, tx.Hash())
			if err != nil {
				return fmt.Errorf("can't get ApplicationLog bytes for tx %s at height %d from A: %w", tx.Hash().StringLE(), i, err)
			}
			db, err := rpcutil.GetApplicationLog(cb, tx.Hash())
			if err != nil {
				return fmt.Errorf("can't get ApplicationLog bytes for tx %s at height %d from B: %w", tx.Hash().StringLE(), i, err)
			}
			if !bytes.Equal(da, db) {
				fmt.Printf("applogs differ at %d, block %s, tx %s\n", i, blk.Hash().StringLE(), tx.Hash().StringLE())
				rpcutil.DumpApplicationLogDiff(a, b, da, db)
				return errors.New("application log mismatch found")
			}
		}
	}

	return nil
}

func main() {
	ctl := cli.NewApp()
	ctl.Name = "compare-applogs"
	ctl.Version = "1.0"
	ctl.Usage = "compare-applogs [--ignore-height] RPC_A RPC_B [-s start] [-e end]"
	ctl.Action = cliMain
	ctl.Flags = []cli.Flag{
		&cli.UintFlag{
			Name:    "start",
			Aliases: []string{"s"},
			Usage:   "Block number to start from (inclusive)",
		},
		&cli.UintFlag{
			Name:    "end",
			Aliases: []string{"e"},
			Usage:   "Block number to end at (exclusive)",
		},
		rpcutil.IgnoreHeightFlag,
	}

	if err := ctl.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
