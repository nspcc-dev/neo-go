package util

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
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

			originalOIDs, err := getBlockIDs(ctx, neoFSPool, containerID, originalOID, indexFileSize, signer, retries, debug)
			if err != nil {
				return cli.Exit(fmt.Errorf("failed to retrieve block OIDs for index file %d (%s): %w", originalID, originalOID, err), 1)
			}

			startHeight := uint32(duplicateID) * uint32(indexFileSize)
			endHeight := startHeight + uint32(indexFileSize)
			err = deleteOrphans(ctx, neoFSPool, signer, containerID, blockAttr, originalOIDs,
				int(startHeight), int(endHeight), int(retries), debug, dryRun)
			if err != nil {
				return cli.Exit(fmt.Errorf("failed to remove block duplicates: %w", err), 1)
			}
		}
	}
	fmt.Fprintln(ctx.App.Writer, "Audit is completed.")
	return nil
}

func getBlockIDs(ctx *cli.Context, p *pool.Pool, containerID cid.ID, indexFileID oid.ID, indexFileSize uint, signer user.Signer, maxRetries uint, debug bool) ([]oid.ID, error) {
	var rc io.ReadCloser

	err := retry(func() error {
		var e error
		_, rc, e = p.ObjectGetInit(ctx.Context, containerID, indexFileID, signer, client.PrmObjectGet{})
		return e
	}, maxRetries, debug)
	if err != nil {
		return nil, fmt.Errorf("failed to get index file %s: %w", indexFileID, err)
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
func deleteOrphans(ctx *cli.Context, p *pool.Pool, signer user.Signer, containerID cid.ID, blockAttr string, originalOIDs []oid.ID, start, end, maxRetries int, debug, dryRun bool) error {
	var (
		cursor   string
		oidIndex int
	)

	// Search for block objects with height matching the expected one.
	f := object.NewSearchFilters()
	f.AddFilter(blockAttr, strconv.Itoa(start), object.MatchNumGE)
	f.AddFilter(blockAttr, strconv.Itoa(end), object.MatchNumLT)

	for {
		var (
			err        error
			nextCursor string
			page       []client.SearchResultItem
		)

		err = retry(func() error {
			page, nextCursor, err = p.SearchObjects(ctx.Context, containerID, f, []string{blockAttr}, cursor, signer, client.SearchObjectsOptions{})
			if err != nil {
				return fmt.Errorf("failed to search objects: %w", err)
			}
			return nil
		}, uint(maxRetries), debug)
		if err != nil {
			return err
		}
		for _, itm := range page {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			foundID := itm.ID
			foundIndex, err := strconv.Atoi(itm.Attributes[0])
			if err != nil {
				return fmt.Errorf("incorrect index in result: %q", itm.Attributes[0])
			}
			if debug {
				fmt.Fprintf(ctx.App.Writer, "found block %d (%s), expected %d (%s)\n", foundIndex, foundID, oidIndex, originalOIDs[oidIndex])
			}
			if foundIndex == start+oidIndex && foundID == originalOIDs[oidIndex] {
				oidIndex++
				continue
			}

			if foundIndex > start+oidIndex {
				for start+oidIndex < foundIndex {
					fmt.Fprintf(ctx.App.Writer, "WARN: block %d (%s) is listed in the index file but missing from the storage\n", start+oidIndex, originalOIDs[oidIndex])
					oidIndex++
				}
				if foundID == originalOIDs[oidIndex] {
					continue
				}
			}
			if dryRun {
				fmt.Fprintf(ctx.App.Writer, "[dry-run] block duplicate %s / %s (%d)\n", foundID, originalOIDs[oidIndex], foundIndex)
			} else {
				err := retry(func() error {
					_, errDelete := p.ObjectDelete(ctx.Context, containerID, foundID, signer, client.PrmObjectDelete{})
					return errDelete
				}, uint(maxRetries), debug)

				if err != nil {
					fmt.Fprintf(ctx.App.Writer, "WARN: failed to remove block %s / %s (%d): %s\n", foundID, originalOIDs[foundIndex-start], foundIndex, err)
				} else if debug {
					fmt.Fprintf(ctx.App.Writer, "Block duplicate %s / %s (%d) is removed\n", foundID, originalOIDs[foundIndex-start], foundIndex)
				}
			}
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return nil
}
