package util

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
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
	indexAttrKey := ctx.String("index-attribute")
	indexFileSize := ctx.Uint("index-file-size")
	retries := ctx.Uint("retries")
	cnrID := ctx.String("container")
	debug := ctx.Bool("debug")
	dryRun := ctx.Bool("dry-run")
	blockAttr := ctx.String("block-attribute")
	searchers := ctx.Uint("searchers")

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

	filters := object.NewSearchFilters()
	filters.AddFilter(indexAttrKey, fmt.Sprintf("%d", 0), object.MatchNumGE)
	results, errs := neofs.ObjectSearch(ctx.Context, neoFSPool, acc.PrivateKey(), containerID, filters, []string{indexAttrKey})

	var (
		originalID  uint64
		originalOID oid.ID
	)
loop:
	for {
		select {
		case <-ctx.Done():
			return cli.Exit("context cancelled", 1)
		case err, ok := <-errs:
			if !ok {
				break loop
			}
			if err != nil {
				return cli.Exit(fmt.Sprintf("search index files: %v", err), 1)
			}
		case itm, ok := <-results:
			if !ok {
				break loop
			}
			duplicateID, err := strconv.ParseUint(itm.Attributes[0], 10, 32)
			if err != nil {
				return cli.Exit(fmt.Errorf("failed to parse index file ID (%s): %w", itm.ID, err), 1)
			}

			if !originalOID.IsZero() && duplicateID == originalID {
				if dryRun {
					fmt.Fprintf(ctx.App.Writer, "[dry-run] index file duplicate %s / %s (%d)\n", itm.ID, originalOID, originalID)
				} else {
					_, err := neoFSPool.ObjectDelete(ctx.Context, containerID, itm.ID, signer, client.PrmObjectDelete{})
					if err != nil {
						return cli.Exit(fmt.Errorf("failed to remove index file duplicate %s / %s (%d): %w", itm.ID, originalOID, originalID, err), 1)
					}
					fmt.Fprintf(ctx.App.Writer, "Index file duplicate %s / %s (%d) is removed\n", itm.ID, originalOID, originalID)
				}
				continue
			}
			originalID = duplicateID
			originalOID = itm.ID
			fmt.Fprintf(ctx.App.Writer, "Processing index file %d (%s)\n", originalID, originalOID)

			originalOIDs, err := getBlockIDs(ctx, neoFSPool, containerID, originalOID, indexFileSize, acc.PrivateKey(), retries, debug)
			if err != nil {
				return cli.Exit(fmt.Errorf("failed to retrieve block OIDs for index file %d (%s): %w", originalID, originalOID, err), 1)
			}

			startHeight := uint32(duplicateID) * uint32(indexFileSize)
			endHeight := startHeight + uint32(indexFileSize)
			err = deleteOrphans(ctx, neoFSPool, acc.PrivateKey(), containerID, blockAttr, originalOIDs,
				int(startHeight), int(endHeight), int(searchers), int(retries), debug, dryRun)
			if err != nil {
				return cli.Exit(fmt.Errorf("failed to remove block duplicates: %w", err), 1)
			}
		}
	}
	fmt.Fprintln(ctx.App.Writer, "Audit is completed.")
	return nil
}

func getBlockIDs(ctx *cli.Context, p *pool.Pool, containerID cid.ID, indexFileID oid.ID, indexFileSize uint, priv *keys.PrivateKey, maxRetries uint, debug bool) ([]oid.ID, error) {
	u, err := url.Parse(fmt.Sprintf("%s:%s/%s", neofs.URIScheme, containerID, indexFileID))
	if err != nil {
		return nil, err
	}
	var rc io.ReadCloser

	err = retry(func() error {
		var e error
		rc, e = neofs.GetWithClient(ctx.Context, p, priv, u, false)
		return e
	}, maxRetries, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to get index file %s: %w", u, err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	if len(raw) != int(indexFileSize)*oid.Size {
		return nil, fmt.Errorf("index file %s: size mismatch: expected %d bytes, got %d", indexFileID, int(indexFileSize)*oid.Size, len(raw))
	}

	out := make([]oid.ID, 0, indexFileSize)
	for i := range indexFileSize {
		out = append(out, oid.ID(raw[i*oid.Size:(i+1)*oid.Size]))
	}
	return out, nil
}

// deleteOrphans removes every block object those OID differs from the one
// specified in the index file for the given height. It prints a WARN if the
// expected object is missing. If dryRun is enabled, it prints duplicate OIDs
// instead of removing them.
func deleteOrphans(ctx *cli.Context, p *pool.Pool, priv *keys.PrivateKey, containerID cid.ID, blockAttr string, originalOIDs []oid.ID, start, end, workersCnt, maxRetries int, debug, dryRun bool) error {
	var (
		wg      sync.WaitGroup
		printMu sync.Mutex
		s       = user.NewAutoIDSignerRFC6979(priv.PrivateKey)
	)

	wg.Add(workersCnt)
	for w := range workersCnt {
		go func(offset int) {
			defer wg.Done()

			for height := start + offset; height < end; height += workersCnt {
				var (
					orphans    []oid.ID
					original   = originalOIDs[height-start]
					originalOK bool
				)

				// Search for block objects with height matching the expected one.
				f := object.NewSearchFilters()
				f.AddFilter(blockAttr, strconv.Itoa(height), object.MatchStringEqual)
				results, errs := neofs.ObjectSearch(ctx.Context, p, priv,
					containerID, f, []string{blockAttr})

			loop:
				for {
					select {
					case itm, ok := <-results:
						if !ok {
							break loop
						}
						if itm.ID == original {
							originalOK = true
						} else {
							orphans = append(orphans, itm.ID)
						}
					case err, ok := <-errs:
						if !ok {
							break loop
						}
						if err != nil {
							printMu.Lock()
							fmt.Fprintf(ctx.App.Writer, "WARN: failed to search for block duplicates at %d: %s\n", height, err)
							printMu.Unlock()
						}
					}
				}

				// Warn if the index entry is missing.
				if !originalOK {
					printMu.Lock()
					fmt.Fprintf(ctx.App.Writer, "WARN: block %d (%s) is listed in the index file but missing from the storage\n", height, original)
					printMu.Unlock()
				}

				// Delete orphans.
				for _, orphan := range orphans {
					if dryRun {
						printMu.Lock()
						fmt.Fprintf(ctx.App.Writer, "[dry-run] block duplicate %s / %s (%d)\n", orphan, original, height)
						printMu.Unlock()
						continue
					}

					err := retry(func() error {
						_, errDelete := p.ObjectDelete(ctx.Context, containerID, orphan, s, client.PrmObjectDelete{})
						return errDelete
					}, uint(maxRetries), debug)

					printMu.Lock()
					if err != nil {
						fmt.Fprintf(ctx.App.Writer, "WARN: failed to remove block %s / %s (%d): %s\n", orphan, original, height, err)
					} else if debug {
						fmt.Fprintf(ctx.App.Writer, "Block duplicate %s / %s (%d) is removed\n", orphan, original, height)
					}
					printMu.Unlock()
				}
			}
		}(w)
	}

	wg.Wait()
	return nil
}
