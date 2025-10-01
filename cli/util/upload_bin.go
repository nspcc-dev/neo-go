package util

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	"github.com/nspcc-dev/neofs-sdk-go/container"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/nspcc-dev/neofs-sdk-go/pool"
	"github.com/nspcc-dev/neofs-sdk-go/user"
	"github.com/urfave/cli/v2"
)

func uploadBin(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	attr := ctx.String("block-attribute")
	numWorkers := ctx.Uint("workers")
	maxRetries := ctx.Uint("retries")
	debug := ctx.Bool("debug")
	batchSize := ctx.Uint("batch-size")
	acc, _, err := options.GetAccFromContext(ctx)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to load account: %v", err), 1)
	}
	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()
	rpc, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to create RPC client: %v", err), 1)
	}

	signer, p, err := options.GetNeoFSClientPool(ctx, acc)
	if err != nil {
		return cli.Exit(err, 1)
	}
	defer p.Close()

	v, err := rpc.GetVersion()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to get version from RPC: %v", err), 1)
	}
	magic := strconv.Itoa(int(v.Protocol.Network))
	containerID, err := getContainer(ctx, p, magic, maxRetries, debug)
	if err != nil {
		return cli.Exit(err, 1)
	}

	currentBlockHeight, err := rpc.GetBlockCount()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to get current block height from RPC: %v", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, "Chain block height:", currentBlockHeight)
	batchID, existing, err := searchLastBatch(ctx, p, containerID, signer, batchSize, attr, uint(currentBlockHeight), debug)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to find objects: %w", err), 1)
	}

	err = uploadBlocks(ctx, p, rpc, signer, containerID, attr, existing, batchID, batchSize, uint(currentBlockHeight), numWorkers, maxRetries, debug)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to upload objects: %w", err), 1)
	}
	return nil
}

// retry function with exponential backoff.
func retry(action func() error, maxRetries uint, debug bool) error {
	var err error
	backoff := neofs.InitialBackoff
	for i := range maxRetries {
		if err = action(); err == nil {
			return nil // Success, no retry needed.
		}
		if debug {
			fmt.Printf("Retry %d: %v\n", i, err)
		}
		time.Sleep(backoff) // Backoff before retrying.
		backoff *= time.Duration(neofs.BackoffFactor)
		if backoff > neofs.MaxBackoff {
			backoff = neofs.MaxBackoff
		}
	}
	return err // Return the last error after exhausting retries.
}

// uploadBlocks uploads missing blocks in batches.
func uploadBlocks(ctx *cli.Context, p *pool.Pool, rpc *rpcclient.Client, signer user.Signer, containerID cid.ID, attr string, existing map[uint]oid.ID, currentBatchID, batchSize, currentBlockHeight uint, numWorkers, maxRetries uint, debug bool) error {
	if currentBatchID*batchSize+uint(len(existing)) >= currentBlockHeight {
		fmt.Fprintf(ctx.App.Writer, "No new blocks to upload. Need to upload starting from %d, current height %d\n", currentBatchID*batchSize+uint(len(existing)), currentBlockHeight)
		return nil
	}
	fmt.Fprintln(ctx.App.Writer, "Uploading blocks...")

	var mu sync.RWMutex
	for batchStart := currentBatchID * batchSize; batchStart < currentBlockHeight; batchStart += batchSize {
		var (
			batchEnd = min(batchStart+batchSize, currentBlockHeight)
			errCh    = make(chan error)
			doneCh   = make(chan struct{})
			wg       sync.WaitGroup
		)
		fmt.Fprintf(ctx.App.Writer, "Processing batch from %d to %d\n", batchStart, batchEnd-1)
		wg.Add(int(numWorkers))
		for i := range numWorkers {
			go func(i uint) {
				defer wg.Done()
				for blockIndex := batchStart + i; blockIndex < batchEnd; blockIndex += numWorkers {
					mu.RLock()
					if _, uploaded := existing[blockIndex]; uploaded {
						mu.RUnlock()
						if debug {
							fmt.Fprintf(ctx.App.Writer, "Block %d already uploaded\n", blockIndex)
						}
						continue
					}
					mu.RUnlock()

					var blk *block.Block
					errGet := retry(func() error {
						var errGetBlock error
						blk, errGetBlock = rpc.GetBlockByIndex(uint32(blockIndex))
						if errGetBlock != nil {
							return fmt.Errorf("failed to fetch block %d: %w", blockIndex, errGetBlock)
						}
						return nil
					}, maxRetries, debug)
					if errGet != nil {
						select {
						case errCh <- errGet:
						default:
						}
						return
					}

					bw := io.NewBufBinWriter()
					blk.EncodeBinary(bw.BinWriter)
					if bw.Err != nil {
						select {
						case errCh <- fmt.Errorf("failed to encode block %d: %w", blockIndex, bw.Err):
						default:
						}
						return
					}
					attrs := []object.Attribute{
						object.NewAttribute(attr, strconv.Itoa(int(blk.Index))),
						object.NewAttribute("Primary", strconv.Itoa(int(blk.PrimaryIndex))),
						object.NewAttribute("Hash", blk.Hash().StringLE()),
						object.NewAttribute("PrevHash", blk.PrevHash.StringLE()),
						object.NewAttribute("BlockTime", strconv.FormatUint(blk.Timestamp, 10)),
						object.NewAttribute("Timestamp", strconv.FormatInt(time.Now().Unix(), 10)),
					}

					var (
						objBytes = bw.Bytes()
						resOid   oid.ID
					)
					errRetr := retry(func() error {
						var errUpload error
						resOid, errUpload = uploadObj(ctx.Context, p, signer, containerID, objBytes, attrs)
						if errUpload != nil {
							return errUpload
						}
						if debug {
							fmt.Fprintf(ctx.App.Writer, "Uploaded block %d with object ID: %s\n", blockIndex, resOid.String())
						}
						return errUpload
					}, maxRetries, debug)
					if errRetr != nil {
						select {
						case errCh <- errRetr:
						default:
						}
						return
					}

					mu.Lock()
					existing[blockIndex] = resOid
					mu.Unlock()
				}
			}(i)
		}

		go func() {
			wg.Wait()
			close(doneCh)
		}()

		select {
		case err := <-errCh:
			return fmt.Errorf("upload error: %w", err)
		case <-doneCh:
		}
		fmt.Fprintf(ctx.App.Writer, "Successfully processed batch of blocks: from %d to %d\n", batchStart, batchEnd-1)
	}
	return nil
}

// searchLastBatch scans batches backwards to find the last fully-uploaded batch;
// Returns next batch ID and a map of already uploaded object IDs in that batch.
func searchLastBatch(ctx *cli.Context, p *pool.Pool, containerID cid.ID, signer user.Signer, batchSize uint, blockAttributeKey string, currentBlockHeight uint, debug bool) (uint, map[uint]oid.ID, error) {
	var (
		totalBatches = (currentBlockHeight + batchSize - 1) / batchSize
		nextExisting = make(map[uint]oid.ID, batchSize)
		nextBatch    uint
	)

	for b := int(totalBatches) - 1; b >= 0; b-- {
		start := uint(b) * batchSize
		end := min(start+batchSize, currentBlockHeight)

		// Collect blocks in [start, end).
		existing := make(map[uint]oid.ID, batchSize)
		blkErr := make(chan error)
		items := searchObjects(ctx.Context, p, containerID, signer,
			blockAttributeKey, start, end, blkErr)

	loop:
		for {
			select {
			case itm, ok := <-items:
				if !ok {
					break loop
				}
				idx, err := strconv.ParseUint(itm.Attributes[0], 10, 32)
				if err != nil {
					return 0, nil, fmt.Errorf("failed to parse block index: %w", err)
				}
				if duplicate, exists := existing[uint(idx)]; exists {
					if debug {
						fmt.Fprintf(ctx.App.Writer,
							"Duplicate object found for block %d: %s, already exists as %s\n",
							idx, itm.ID, duplicate)
					}
					continue
				}
				existing[uint(idx)] = itm.ID
			case err := <-blkErr:
				return 0, nil, err
			}
		}

		if len(existing) == 0 {
			// completely empty → keep scanning earlier batches
			continue
		}
		if uint(len(existing)) < batchSize {
			// non-empty and incomplete  →  resume here
			nextExisting = existing
			nextBatch = uint(b)
			break
		}
		// otherwise: full batch  →  resume after it
		nextBatch = uint(b) + 1
		break
	}
	fmt.Fprintf(ctx.App.Writer, "Last fully uploaded batch: %d, next to upload: %d\n", int(nextBatch)-1, nextBatch)

	return nextBatch, nextExisting, nil
}

// searchObjects searches objects with attribute GE startIndex and LT endIndex.
// It returns a buffered channel of resulting object IDs and closes it once OID
// search is finished. Errors are sent to errCh in a non-blocking way.
func searchObjects(ctx context.Context, p *pool.Pool, containerID cid.ID, signer user.Signer, blockAttributeKey string, startIndex, endIndex uint, errCh chan error) chan client.SearchResultItem {
	var res = make(chan client.SearchResultItem, 2*neofs.DefaultSearchBatchSize)

	go func() {
		defer close(res)

		f := object.NewSearchFilters()
		f.AddFilter(blockAttributeKey, strconv.FormatUint(uint64(startIndex), 10), object.MatchNumGE)
		f.AddFilter(blockAttributeKey, strconv.FormatUint(uint64(endIndex), 10), object.MatchNumLT)

		var (
			cursor string
			items  []client.SearchResultItem
			err    error
		)

		for {
			items, cursor, err = p.SearchObjects(ctx, containerID, f, []string{blockAttributeKey}, cursor, signer, client.SearchObjectsOptions{})
			if err != nil {
				select {
				case errCh <- fmt.Errorf("failed to search objects from %d to %d (cursor=%q): %w", startIndex, endIndex, cursor, err):
				default:
				}
				return
			}

			for _, it := range items {
				select {
				case <-ctx.Done():
					return
				case res <- it:
				}
			}

			if cursor == "" {
				break
			}
		}
	}()

	return res
}

// uploadObj uploads object to the container using provided settings.
func uploadObj(ctx context.Context, p *pool.Pool, signer user.Signer, containerID cid.ID, objData []byte, attrs []object.Attribute) (oid.ID, error) {
	var (
		hdr              object.Object
		prmObjectPutInit client.PrmObjectPutInit
		resOID           = oid.ID{}
	)

	hdr.SetContainerID(containerID)
	hdr.SetOwner(signer.UserID())
	hdr.SetAttributes(attrs...)

	writer, err := p.ObjectPutInit(ctx, hdr, signer, prmObjectPutInit)
	if err != nil {
		return resOID, fmt.Errorf("failed to initiate object upload: %w", err)
	}
	_, err = writer.Write(objData)
	if err != nil {
		_ = writer.Close()
		return resOID, fmt.Errorf("failed to write object data: %w", err)
	}
	err = writer.Close()
	if err != nil {
		return resOID, fmt.Errorf("failed to close object writer: %w", err)
	}
	res := writer.GetResult()
	resOID = res.StoredObjectID()
	return resOID, nil
}

// getContainer gets container by ID and checks its magic.
func getContainer(ctx *cli.Context, p *pool.Pool, expectedMagic string, maxRetries uint, debug bool) (cid.ID, error) {
	var (
		containerObj   container.Container
		err            error
		containerIDStr = ctx.String("container")
	)
	var containerID cid.ID
	if err = containerID.DecodeString(containerIDStr); err != nil {
		return containerID, fmt.Errorf("failed to decode container ID: %w", err)
	}
	err = retry(func() error {
		containerObj, err = p.ContainerGet(ctx.Context, containerID, client.PrmContainerGet{})
		return err
	}, maxRetries, debug)
	if err != nil {
		return containerID, fmt.Errorf("failed to get container: %w", err)
	}
	containerMagic := containerObj.Attribute("Magic")
	if containerMagic != expectedMagic {
		return containerID, fmt.Errorf("container magic mismatch: expected %s, got %s", expectedMagic, containerMagic)
	}
	return containerID, nil
}
