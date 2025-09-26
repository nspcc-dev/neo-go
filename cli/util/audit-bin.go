package util

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/urfave/cli/v2"
)

func auditBin(ctx *cli.Context) error {
	const step = 1000
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	retries := ctx.Uint("retries")
	cnrID := ctx.String("container")
	debug := ctx.Bool("debug")
	dryRun := ctx.Bool("dry-run")
	blockAttr := ctx.String("block-attribute")
	skip := ctx.Int("skip")

	acc, _, err := options.GetAccFromContext(ctx)
	if err != nil {
		if errors.Is(err, options.ErrNoWallet) {
			acc, err = wallet.NewAccount()
			if err != nil {
				return cli.Exit(fmt.Errorf("no wallet provided and failed to create account for NeoFS interaction: %w", err), 1)
			}
		} else {
			return cli.Exit(fmt.Errorf("failed to load account: %w", err), 1)
		}
	}
	signer, neoFSPool, err := options.GetNeoFSClientPool(ctx, acc)
	if err != nil {
		return cli.Exit(err, 1)
	}
	defer neoFSPool.Close()

	var containerID cid.ID
	if err = containerID.DecodeString(cnrID); err != nil {
		return cli.Exit(fmt.Errorf("failed to decode container ID: %w", err), 1)
	}
	if _, err = neoFSPool.ContainerGet(ctx.Context, containerID, client.PrmContainerGet{}); err != nil {
		return cli.Exit(fmt.Errorf("failed to get container %s: %w", containerID, err), 1)
	}

	if skip > 0 {
		fmt.Fprintf(ctx.App.Writer, "Skipping first %d blocks\n", skip)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()
	rpc, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to create RPC client: %v", err), 1)
	}

	currentBlockHeight, err := rpc.GetBlockCount()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to get current block height from RPC: %v", err), 1)
	}
	for startHeight := uint32(skip); startHeight < currentBlockHeight; startHeight += step {
		f := object.NewSearchFilters()
		f.AddFilter(blockAttr, strconv.FormatUint(uint64(startHeight), 10), object.MatchNumGE)
		f.AddFilter(blockAttr, strconv.FormatUint(uint64(min(startHeight+step, currentBlockHeight)), 10), object.MatchNumLT)

		var (
			cursor         string
			expectedHeight = uint64(startHeight)
			origHeight     uint64
			originalOID    oid.ID
		)

		for {
			var (
				page       []client.SearchResultItem
				nextCursor string
			)

			err = retry(func() error {
				var e error
				page, nextCursor, e = neoFSPool.SearchObjects(ctx.Context, containerID, f, []string{blockAttr}, cursor, signer, client.SearchObjectsOptions{})
				if e != nil {
					return fmt.Errorf("failed to search objects: %w", e)
				}
				return nil
			}, retries, debug)
			if err != nil {
				return cli.Exit(fmt.Sprintf("search block objects: %v", err), 1)
			}

			for _, itm := range page {
				select {
				case <-ctx.Done():
					return cli.Exit("context cancelled", 1)
				default:
				}

				blockHeight, err := strconv.ParseUint(itm.Attributes[0], 10, 64)
				if err != nil {
					return cli.Exit(fmt.Errorf("failed to parse block ID (%s): %w", itm.ID, err), 1)
				}

				if !originalOID.IsZero() && origHeight == blockHeight {
					if dryRun {
						fmt.Fprintf(ctx.App.Writer, "[dry-run] block duplicate %s / %s (%d)\n", itm.ID, originalOID, origHeight)
					} else {
						err = retry(func() error {
							_, e := neoFSPool.ObjectDelete(ctx.Context, containerID, itm.ID, signer, client.PrmObjectDelete{})
							return e
						}, retries, debug)
						if err != nil {
							return cli.Exit(fmt.Errorf("failed to remove block duplicate %s / %s (%d): %w", itm.ID, originalOID, origHeight, err), 1)
						}
						fmt.Fprintf(ctx.App.Writer, "block duplicate %s / %s (%d) is removed\n", itm.ID, originalOID, origHeight)
					}
					continue
				}

				for expectedHeight < blockHeight {
					var blk *block.Block
					err = retry(func() error {
						var e error
						blk, e = rpc.GetBlockByIndex(uint32(expectedHeight))
						return e
					}, retries, debug)
					if err != nil {
						return fmt.Errorf("failed to fetch block %d: %w", expectedHeight, err)
					}

					bw := io.NewBufBinWriter()
					blk.EncodeBinary(bw.BinWriter)

					attrs := []object.Attribute{
						object.NewAttribute(blockAttr, strconv.FormatUint(uint64(blk.Index), 10)),
						object.NewAttribute("Primary", strconv.FormatUint(uint64(blk.PrimaryIndex), 10)),
						object.NewAttribute("Hash", blk.Hash().StringLE()),
						object.NewAttribute("PrevHash", blk.PrevHash.StringLE()),
						object.NewAttribute("BlockTime", strconv.FormatUint(blk.Timestamp, 10)),
						object.NewAttribute("Timestamp", strconv.FormatInt(time.Now().Unix(), 10)),
					}

					var objBytes = bw.Bytes()
					err = retry(func() error {
						var e error
						_, e = uploadObj(ctx.Context, neoFSPool, signer, containerID, objBytes, attrs)
						return e
					}, retries, debug)
					if err != nil {
						return fmt.Errorf("failed to upload block %d: %w", expectedHeight, err)
					}
					expectedHeight++
				}

				origHeight = blockHeight
				originalOID = itm.ID
				expectedHeight++
			}

			if nextCursor == "" {
				break
			}
			cursor = nextCursor
		}
	}

	fmt.Fprintln(ctx.App.Writer, "Audit is completed.")
	return nil
}
