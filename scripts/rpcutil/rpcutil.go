package rpcutil

import (
	"context"
	"fmt"

	ojson "github.com/nspcc-dev/go-ordered-json"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/urfave/cli/v2"
)

// IgnoreHeightFlag ignores blockchain height difference.
var IgnoreHeightFlag = &cli.BoolFlag{
	Name:    "ignore-height",
	Aliases: []string{"g"},
	Usage:   "Ignore height difference",
}

// InitClient initializes an RPC client and returns the current blockchain height.
func InitClient(addr string, name string) (*rpcclient.Client, uint32, error) {
	c, err := rpcclient.New(context.Background(), addr, rpcclient.Options{})
	if err != nil {
		return nil, 0, fmt.Errorf("RPC %s: %w", name, err)
	}
	if err = c.Init(); err != nil {
		return nil, 0, fmt.Errorf("RPC %s init: %w", name, err)
	}
	h, err := c.GetBlockCount()
	if err != nil {
		return nil, 0, fmt.Errorf("RPC %s block count: %w", name, err)
	}
	return c, h, nil
}

// CompareNetwork verifies that both RPC nodes belong to the same network.
func CompareNetwork(ca, cb *rpcclient.Client) error {
	na := ca.Network()
	nb := cb.Network()
	if na != nb {
		return fmt.Errorf("different network magic: A=%d (0x%X), B=%d (0x%X)", na, uint32(na), nb, uint32(nb))
	}
	return nil
}

// GetApplicationLog returns a normalized JSON representation of an application log.
func GetApplicationLog(c *rpcclient.Client, h util.Uint256) ([]byte, error) {
	l, err := c.GetApplicationLog(h, nil)
	if err != nil {
		return nil, fmt.Errorf("can't get ApplicationLog for %s: %w", h.StringLE(), err)
	}
	// Ignore FaultException and Invocations because its message may differ between implementations.
	for i := range l.Executions {
		l.Executions[i].FaultException = ""
		l.Executions[i].Invocations = nil
	}
	d, err := ojson.MarshalIndent(l, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ApplicationLog for %s: %w", h.StringLE(), err)
	}
	return d, nil
}

// DumpApplicationLogDiff prints a unified diff for two application logs.
func DumpApplicationLogDiff(a string, b string, da []byte, db []byte) {
	diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(da)),
		B:        difflib.SplitLines(string(db)),
		FromFile: a,
		ToFile:   b,
		Context:  1,
	})
	fmt.Println(diff)
}

// CheckHeights checks chain height difference and returns the minimum height.
func CheckHeights(ha, hb uint32, ignore bool) (uint32, error) {
	const maxHeightDrift = 10

	refHeight := ha
	diff := hb - ha
	if ha > hb {
		refHeight = hb
		diff = ha - hb
	}

	if diff > maxHeightDrift && !ignore {
		return 0, fmt.Errorf("chains have different heights: %d vs %d", ha, hb)
	}
	return refHeight, nil
}
