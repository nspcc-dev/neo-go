package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/urfave/cli"
)

var errStateMatches = errors.New("state matches")

func initClient(addr string, name string) (*rpcclient.Client, uint32, error) {
	c, err := rpcclient.New(context.Background(), addr, rpcclient.Options{})
	if err != nil {
		return nil, 0, fmt.Errorf("RPC %s: %w", name, err)
	}
	err = c.Init()
	if err != nil {
		return nil, 0, fmt.Errorf("RPC %s init: %w", name, err)
	}
	h, err := c.GetBlockCount()
	if err != nil {
		return nil, 0, fmt.Errorf("RPC %s block count: %w", name, err)
	}
	return c, h, nil
}

func getRoots(ca *rpcclient.Client, cb *rpcclient.Client, h uint32) (util.Uint256, util.Uint256, error) {
	ra, err := ca.GetStateRootByHeight(h)
	if err != nil {
		return util.Uint256{}, util.Uint256{}, fmt.Errorf("getstateroot from A for %d: %w", h, err)
	}
	rb, err := cb.GetStateRootByHeight(h)
	if err != nil {
		return util.Uint256{}, util.Uint256{}, fmt.Errorf("getstateroot from B for %d: %w", h, err)
	}
	return ra.Root, rb.Root, nil
}

func bisectState(ca *rpcclient.Client, cb *rpcclient.Client, h uint32) (uint32, error) {
	ra, rb, err := getRoots(ca, cb, 0)
	if err != nil {
		return 0, err
	}
	fmt.Printf("at %d: %s vs %s\n", 0, ra.StringLE(), rb.StringLE())
	if ra != rb {
		return 0, nil
	}
	good := uint32(0)
	ra, rb, err = getRoots(ca, cb, h)
	if err != nil {
		return 0, err
	}
	fmt.Printf("at %d: %s vs %s\n", h, ra.StringLE(), rb.StringLE())
	if ra.Equals(rb) {
		return 0, fmt.Errorf("%w at %d", errStateMatches, h)
	}
	bad := h
	for bad-good > 1 {
		next := good + (bad-good)/2
		ra, rb, err = getRoots(ca, cb, next)
		if err != nil {
			return 0, err
		}
		fmt.Printf("at %d: %s vs %s\n", next, ra.StringLE(), rb.StringLE())
		if ra == rb {
			good = next
		} else {
			bad = next
		}
	}
	return bad, nil
}

func cliMain(c *cli.Context) error {
	a := c.Args().Get(0)
	b := c.Args().Get(1)
	if a == "" {
		return errors.New("no arguments given")
	}
	if b == "" {
		return errors.New("missing second argument")
	}
	ca, ha, err := initClient(a, "A")
	if err != nil {
		return err
	}
	cb, hb, err := initClient(b, "B")
	if err != nil {
		return err
	}
	var refHeight = ha
	if ha != hb {
		var diff = hb - ha
		if ha > hb {
			refHeight = hb
			diff = ha - hb
		}
		if diff > 10 && !c.Bool("ignore-height") { // Allow some height drift.
			return fmt.Errorf("chains have different heights: %d vs %d", ha, hb)
		}
	}
	h, err := bisectState(ca, cb, refHeight-1)
	if err != nil {
		if errors.Is(err, errStateMatches) {
			return nil
		}
		return err
	}
	blk, err := ca.GetBlockByIndex(h)
	if err != nil {
		return err
	}
	fmt.Printf("state differs at %d, block %s\n", h, blk.Hash().StringLE())
	dumpApplogDiff(true, blk.Hash(), a, b, ca, cb)
	for _, t := range blk.Transactions {
		dumpApplogDiff(false, t.Hash(), a, b, ca, cb)
	}
	return errors.New("different state found")
}

func dumpApplogDiff(isBlock bool, container util.Uint256, a string, b string, ca *rpcclient.Client, cb *rpcclient.Client) error {
	if isBlock {
		fmt.Printf("block %s:\n", container.StringLE())
	} else {
		fmt.Printf("transaction %s:\n", container.StringLE())
	}
	la, err := ca.GetApplicationLog(container, nil)
	if err != nil {
		return err
	}
	lb, err := cb.GetApplicationLog(container, nil)
	if err != nil {
		return err
	}
	da := spew.Sdump(la)
	db := spew.Sdump(lb)
	diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(da),
		B:        difflib.SplitLines(db),
		FromFile: a,
		FromDate: "",
		ToFile:   b,
		ToDate:   "",
		Context:  1,
	})
	fmt.Println(diff)
	return nil
}

func main() {
	ctl := cli.NewApp()
	ctl.Name = "compare-states"
	ctl.Version = "1.0"
	ctl.Usage = "compare-states RPC_A RPC_B"
	ctl.Action = cliMain
	ctl.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "ignore-height, g",
			Usage: "ignore height difference",
		},
	}

	if err := ctl.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
