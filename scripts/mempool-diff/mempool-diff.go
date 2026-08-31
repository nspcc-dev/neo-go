package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/scripts/rpcutil"
	"github.com/urfave/cli/v2"
)

var intervalFlag = &cli.DurationFlag{
	Name:    "interval",
	Aliases: []string{"i"},
	Usage:   "interval between mempool comparisons",
	Value:   10 * time.Second,
}

var outFlag = &cli.StringFlag{
	Name:    "out",
	Aliases: []string{"o"},
	Usage:   "file to append comparison results to (default: stdout)",
}

type node struct {
	addr   string
	client *rpcclient.Client
}

func cliMain(c *cli.Context) error {
	addrs := c.Args().Slice()
	if len(addrs) < 2 {
		return errors.New("at least two RPC addresses must be given")
	}

	nodes := make([]node, 0, len(addrs))
	for i, a := range addrs {
		cl, _, err := rpcutil.InitClient(a, fmt.Sprintf("#%d (%s)", i, a))
		if err != nil {
			return err
		}
		nodes = append(nodes, node{addr: a, client: cl})
	}

	out := io.Writer(os.Stdout)
	if outPath := c.String(outFlag.Name); outPath != "" {
		f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ticker := time.NewTicker(c.Duration(intervalFlag.Name))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			compare(out, nodes)
		}
	}
}

func compare(out io.Writer, nodes []node) {
	i, j := pickTwo(len(nodes))
	a, b := nodes[i], nodes[j]

	mpA, err := a.client.GetRawMemPool()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to get mempool: %v\n", a.addr, err)
		return
	}
	mpB, err := b.client.GetRawMemPool()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: failed to get mempool: %v\n", b.addr, err)
		return
	}

	index, intersection, union := jaccardIndex(mpA, mpB)
	fmt.Fprintf(out, "[%s] %s (%d txs) vs %s (%d txs): intersection=%d union=%d jaccard=%.4f\n",
		time.Now().Format(time.RFC3339), a.addr, len(mpA), b.addr, len(mpB), intersection, union, index)
}

func pickTwo(n int) (int, int) {
	i := rand.Intn(n)
	j := rand.Intn(n)
	for i == j {
		j = rand.Intn(n)
	}
	return i, j
}

func jaccardIndex(a, b []util.Uint256) (index float64, intersection, union int) {
	setA := make(map[util.Uint256]struct{}, len(a))
	for _, h := range a {
		setA[h] = struct{}{}
	}
	setB := make(map[util.Uint256]struct{}, len(b))
	for _, h := range b {
		setB[h] = struct{}{}
	}
	for h := range setA {
		if _, ok := setB[h]; ok {
			intersection++
		}
	}
	union = len(setA) + len(setB) - intersection
	if union == 0 {
		return 0, 0, 0
	}
	return float64(intersection) / float64(union), intersection, union
}

func main() {
	ctl := cli.NewApp()
	ctl.Name = "mempool-diff"
	ctl.Version = "1.0"
	ctl.Usage = "mempool-diff [--interval 10s] [--out FILE] RPC_ADDR [RPC_ADDR...]"
	ctl.Action = cliMain
	ctl.Flags = []cli.Flag{intervalFlag, outFlag}

	if err := ctl.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
