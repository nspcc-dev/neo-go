package util

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/nspcc-dev/neofs-sdk-go/pool"
	"github.com/nspcc-dev/neofs-sdk-go/user"
	"github.com/urfave/cli/v2"
)

func auditBin(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	var (
		numWorkers = ctx.Uint("workers")

		err        error
		errs       = make(chan error, numWorkers)
		haveErrors bool
		tasks      = make(chan func() error)
		wg         sync.WaitGroup
	)
	for range numWorkers {
		wg.Add(1)
		go func() {
			for f := range tasks {
				err := f()
				if err != nil {
					errs <- err
					break
				}
			}
			wg.Done()
		}()
	}

	err = auditBinInt(ctx, tasks, errs)
	close(tasks)
	wg.Wait()

drainErrors:
	for {
		select {
		case anotherErr := <-errs:
			fmt.Fprintf(ctx.App.Writer, "error in worker thread: %s\n", anotherErr)
			haveErrors = true
		default:
			break drainErrors
		}
	}
	if err == nil {
		if haveErrors {
			err = cli.Exit(errors.New("audit failed"), 1) // Change return code to signal thread errors.
		} else {
			fmt.Fprintln(ctx.App.Writer, "Audit is completed.")
		}
	}
	return err
}

func auditBinInt(ctx *cli.Context, tasks chan func() error, errs chan error) error {
	retries := ctx.Uint("retries")
	cnrID := ctx.String("container")
	debug := ctx.Bool("debug")
	dryRun := ctx.Bool("dry-run")
	blockAttr := ctx.String("block-attribute")
	curH := uint64(ctx.Uint("skip"))

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

	if curH != 0 {
		fmt.Fprintf(ctx.App.Writer, "Skipping first %d blocks\n", curH)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()
	rpc, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to create RPC client: %w", err), 1)
	}

	var (
		prevH  uint64
		cursor string
		curOID oid.ID
		f      = object.NewSearchFilters()
	)
	f.AddFilter(blockAttr, strconv.FormatUint(curH, 10), object.MatchNumGE)

	for {
		var page []client.SearchResultItem
		err = retry(func() error {
			page, cursor, err = neoFSPool.SearchObjects(ctx.Context, containerID, f, []string{blockAttr}, cursor, signer, client.SearchObjectsOptions{})
			if err != nil {
				return fmt.Errorf("failed to search objects: %w", err)
			}
			return nil
		}, retries, debug)
		if err != nil {
			return cli.Exit(fmt.Errorf("search block objects: %w", err), 1)
		}

		if debug {
			fmt.Fprintf(ctx.App.Writer, "search returned %d results, previous height %d\n", len(page), prevH)
		}

		for _, itm := range page {
			select {
			case <-ctx.Done():
				return cli.Exit("context cancelled", 1)
			case err := <-errs:
				return cli.Exit(fmt.Errorf("error in worker thread: %w", err), 1)
			default:
			}
			if len(itm.Attributes) != 1 {
				fmt.Fprintf(ctx.App.Writer, "invalid number attributes for %s: expected %d, got %d", itm.ID, 1, len(itm.Attributes))
				continue
			}
			h, err := strconv.ParseUint(itm.Attributes[0], 10, 64)
			if err != nil {
				return cli.Exit(fmt.Errorf("failed to parse block OID (%s): %w", itm.ID, err), 1)
			}

			if h < prevH {
				return cli.Exit(fmt.Errorf("expected >%d height, received %d", prevH, h), 1)
			}

			if !curOID.IsZero() && prevH == h {
				if dryRun {
					fmt.Fprintf(ctx.App.Writer, "[dry-run] block duplicate %s / %s (%d)\n", itm.ID, curOID, prevH)
				} else {
					tasks <- wrapDropBlock(ctx, neoFSPool, signer, containerID, itm.ID, retries, prevH, curOID, debug)
				}
				continue
			}

			for ; curH < h; curH++ {
				if dryRun {
					fmt.Fprintf(ctx.App.Writer, "[dry-run] block %d is missing\n", curH)
				} else {
					tasks <- wrapRestoreMissingBlock(ctx, rpc, neoFSPool, signer, containerID, blockAttr, retries, curH, debug)
				}
			}
			curOID = itm.ID
			prevH = curH
			curH++
		}
		if cursor == "" {
			break
		}
	}

	return nil
}

func wrapDropBlock(ctx *cli.Context, p *pool.Pool, signer user.Signer, containerID cid.ID, objID oid.ID, retries uint, prevH uint64, curOID oid.ID, debug bool) func() error {
	return func() error {
		return dropBlock(ctx, p, signer, containerID, objID, retries, prevH, curOID, debug)
	}
}

func dropBlock(ctx *cli.Context, p *pool.Pool, signer user.Signer, containerID cid.ID, objID oid.ID, retries uint, prevH uint64, curOID oid.ID, debug bool) error {
	err := retry(func() error {
		_, e := p.ObjectDelete(ctx.Context, containerID, objID, signer, client.PrmObjectDelete{})
		return e
	}, retries, debug)
	if err != nil {
		err = fmt.Errorf("failed to remove block duplicate %s / %s (%d): %w", objID, curOID, prevH, err)
	} else if debug {
		fmt.Fprintf(ctx.App.Writer, "block duplicate %s for %d is removed (%s kept)\n", objID, prevH, curOID)
	}
	return err
}

func wrapRestoreMissingBlock(ctx *cli.Context, rpc *rpcclient.Client, p *pool.Pool, signer user.Signer, containerID cid.ID,
	blockAttr string, retries uint, index uint64, debug bool) func() error {
	return func() error {
		return restoreMissingBlock(ctx, rpc, p, signer, containerID, blockAttr, retries, index, debug)
	}
}
func restoreMissingBlock(ctx *cli.Context, rpc *rpcclient.Client, p *pool.Pool, signer user.Signer, containerID cid.ID,
	blockAttr string, retries uint, index uint64, debug bool) error {
	var (
		b   *block.Block
		err error
	)
	err = retry(func() error {
		b, err = rpc.GetBlockByIndex(uint32(index))
		return err
	}, retries, debug)
	if err != nil {
		return fmt.Errorf("failed to fetch block %d: %w", index, err)
	}

	bw := io.NewBufBinWriter()
	b.EncodeBinary(bw.BinWriter)
	if bw.Err != nil {
		return fmt.Errorf("failed to encode block %d: %w", index, bw.Err)
	}

	_, err = createBlockAndUpload(ctx, p, signer, containerID, b, bw, blockAttr, retries, index, debug)
	return err
}

func createBlockAndUpload(ctx *cli.Context, p *pool.Pool, signer user.Signer, containerID cid.ID, b *block.Block,
	bw *io.BufBinWriter, blockAttr string, retries uint, index uint64, debug bool) (oid.ID, error) {
	attrs := []object.Attribute{
		object.NewAttribute(blockAttr, strconv.FormatUint(uint64(b.Index), 10)),
		object.NewAttribute("Hash", b.Hash().StringLE()),
		object.NewAttribute("PrevHash", b.PrevHash.StringLE()),
		object.NewAttribute("BlockTime", strconv.FormatUint(b.Timestamp, 10)),
		object.NewAttribute("Timestamp", strconv.FormatInt(time.Now().Unix(), 10)),
	}

	var (
		objBytes = bw.Bytes()
		OID      oid.ID
	)
	err := retry(func() error {
		var e error
		OID, e = uploadObj(ctx.Context, p, signer, containerID, objBytes, attrs)
		return e
	}, retries, debug)
	if err != nil {
		return oid.ID{}, fmt.Errorf("failed to upload block %d: %w", index, err)
	}
	if debug {
		fmt.Fprintf(ctx.App.Writer, "block %d is uploaded: %s\n", index, OID)
	}
	return OID, nil
}
