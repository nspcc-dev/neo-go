package util

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle/neofs"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/nspcc-dev/neofs-sdk-go/checksum"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/nspcc-dev/neofs-sdk-go/pool"
	"github.com/nspcc-dev/neofs-sdk-go/user"
	"github.com/nspcc-dev/neofs-sdk-go/version"
	"github.com/urfave/cli/v2"
)

const (
	searchBatchSize     = 10000       // Number of objects to search in a batch for finding max block in container.
	maxParallelSearches = 40          // Control the number of concurrent searches.
	oidSize             = sha256.Size // Size of object ID.
)

func uploadBin(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	rpcNeoFS := ctx.StringSlice("fs-rpc-endpoint")
	containerIDStr := ctx.String("container")
	attr := ctx.String("block-attribute")
	indexFileAttribute := ctx.String("index-attribute")
	acc, _, err := options.GetAccFromContext(ctx)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to load account: %v", err), 1)
	}

	var containerID cid.ID
	if err = containerID.DecodeString(containerIDStr); err != nil {
		return cli.Exit(fmt.Sprintf("failed to decode container ID: %v", err), 1)
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
	fmt.Fprintln(ctx.App.Writer, "Chain block height:", currentBlockHeight)

	signer := user.NewAutoIDSignerRFC6979(acc.PrivateKey().PrivateKey)

	p, err := pool.New(pool.NewFlatNodeParams(rpcNeoFS), signer, pool.DefaultOptions())
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to create NeoFS pool: %v", err), 1)
	}

	if err = p.Dial(context.Background()); err != nil {
		return cli.Exit(fmt.Sprintf("failed to dial NeoFS pool: %v", err), 1)
	}
	defer p.Close()

	maxBlockIndex, err := fetchMaxBlockIndex(ctx.Context, p, containerID, acc.PrivateKey(), attr)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to fetch max block index from container: %v", err), 1)
	}

	fmt.Fprintln(ctx.App.Writer, "Blocks uploaded in NeoFS container:", maxBlockIndex)

	if maxBlockIndex >= int(currentBlockHeight) {
		fmt.Fprintln(ctx.App.Writer, "No new blocks to upload. Max index in NeoFS: ", maxBlockIndex, "current height:", currentBlockHeight)
		return nil
	}

	const (
		maxRetries     = 3                      // Maximum number of retries
		initialBackoff = 500 * time.Millisecond // Initial backoff duration
		backoffFactor  = 2                      // Backoff multiplier
		maxBackoff     = 10 * time.Second       // Maximum backoff duration

		numWorkers   = 10             // Number of concurrent block uploaders
		numProducers = numWorkers * 2 // Number of concurrent block fetchers
	)

	blockChan := make(chan *block.Block, 20000)
	errorChan := make(chan error, 1)
	doneChan := make(chan struct{})

	producerWg := sync.WaitGroup{}
	producerWg.Add(numProducers)

	// Retry function with exponential backoff
	retry := func(action func() error) error {
		var err error
		backoff := initialBackoff
		for range maxRetries {
			if err = action(); err == nil {
				return nil // Success, no retry needed
			}
			time.Sleep(backoff) // Backoff before retrying
			backoff *= backoffFactor
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
		return err // Return the last error after exhausting retries
	}

	//Goroutines to fetch blocks concurrently
	for i := range numProducers {
		go func(i int) {
			defer producerWg.Done()
			for blockIndex := maxBlockIndex + 1 + i; blockIndex <= int(currentBlockHeight); blockIndex += numProducers {
				err = retry(func() error {
					blk, err := rpc.GetBlockByIndex(uint32(blockIndex))
					if err != nil {
						return fmt.Errorf("failed to fetch block %d: %w", blockIndex, err)
					}
					select {
					case blockChan <- blk:
					case <-ctx.Context.Done():
						return context.Canceled
					}
					return nil
				})
				if err != nil {
					errorChan <- err
					return
				}
			}
		}(i)
	}

	go func() {
		producerWg.Wait()
		close(blockChan)
	}()

	// upload workers
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for range numWorkers {
		go func() {
			defer wg.Done()
			for blk := range blockChan {
				err := retry(func() error {
					bw := io.NewBufBinWriter()
					blk.EncodeBinary(bw.BinWriter)
					if bw.Err != nil {
						return fmt.Errorf("failed to encode block: %w", bw.Err)
					}

					attrs := []object.Attribute{
						*object.NewAttribute(attr, strconv.Itoa(int(blk.Index))),
						*object.NewAttribute("primary", strconv.Itoa(int(blk.PrimaryIndex))),
						*object.NewAttribute("hash", blk.Hash().Reverse().String()),
						*object.NewAttribute("prevHash", blk.PrevHash.StringLE()),
						*object.NewAttribute("timestamp", strconv.FormatUint(blk.Timestamp, 10)),
					}
					return uploadObj(ctx.Context, *p, signer, acc.PrivateKey().GetScriptHash(), containerID, bw.Bytes(), attrs)
				})
				if err != nil {
					errorChan <- err
					return
				}

				if blk.Index%1000 == 0 {
					fmt.Fprintf(ctx.App.Writer, "[%s] Successfully uploaded block: %d\n", time.Now().Format(time.RFC3339), blk.Index)
				}
			}
		}()
	}

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	// Wait for completion or error
	select {
	case err := <-errorChan:
		return cli.Exit(fmt.Sprintf("Upload error: %v", err), 1)
	case <-doneChan:
		err = updateIndexFiles(ctx, *p, containerID, *acc, signer, uint(currentBlockHeight), indexFileAttribute, attr)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to update index files after upload: %v", err), 1)
		}
		fmt.Fprintln(ctx.App.Writer, "Upload completed successfully.")
		return nil
	}
}

type searchResult struct {
	startIndex int
	numOIDs    int
	err        error
}

// fetchMaxBlockIndex searches the container for the maximum block index.
func fetchMaxBlockIndex(ctx context.Context, p *pool.Pool, containerID cid.ID, priv *keys.PrivateKey, attributeKey string) (int, error) {
	var wg sync.WaitGroup
	height := 0
	for {
		results := make([]searchResult, maxParallelSearches)
		for i := range maxParallelSearches {
			startIndex := height + i*searchBatchSize
			endIndex := startIndex + searchBatchSize - 1

			wg.Add(1)
			go func(i, startIndex, endIndex int) {
				defer wg.Done()

				prm := client.PrmObjectSearch{}
				filters := object.NewSearchFilters()
				filters.AddFilter(attributeKey, fmt.Sprintf("%d", startIndex), object.MatchNumGE)
				filters.AddFilter(attributeKey, fmt.Sprintf("%d", endIndex), object.MatchNumLE)
				prm.SetFilters(filters)

				objectIDs, err := neofs.ObjectSearch(ctx, p, priv, containerID.String(), prm)
				results[i] = searchResult{startIndex: startIndex, numOIDs: len(objectIDs), err: err}
			}(i, startIndex, endIndex)
		}

		wg.Wait()

		for _, res := range results {
			if res.err != nil {
				return 0, res.err
			}
			if res.numOIDs < searchBatchSize {
				if res.startIndex == 0 && res.numOIDs == 0 {
					return -1, nil
				}
				// Return the start index of the first incomplete batch
				return res.startIndex, nil
			}
		}

		height += maxParallelSearches * searchBatchSize
	}
}

// updateIndexFiles updates the index files in the container.
func updateIndexFiles(ctx *cli.Context, p pool.Pool, containerID cid.ID, account wallet.Account, signer user.Signer, currentHeight uint, attributeKey string, blockAttributeKey string) error {
	indexFileSize := ctx.Uint("index-file-size")
	fmt.Fprintln(ctx.App.Writer, "Updating index files...")

	prm := client.PrmObjectSearch{}
	filters := object.NewSearchFilters()
	filters.AddFilter(attributeKey, fmt.Sprintf("%d", 0), object.MatchNumGE)
	filters.AddFilter("size", fmt.Sprintf("%d", indexFileSize), object.MatchStringEqual)
	prm.SetFilters(filters)

	objectIDs, err := neofs.ObjectSearch(ctx.Context, &p, account.PrivateKey(), containerID.String(), prm)
	if err != nil {
		return fmt.Errorf("search of index files failed: %w", err)
	}

	existingIndexCount := uint(len(objectIDs))
	expectedIndexCount := currentHeight / indexFileSize

	if existingIndexCount >= expectedIndexCount {
		fmt.Fprintf(ctx.App.Writer, "Index files are up to date. Existing: %d, expected: %d\n", existingIndexCount, expectedIndexCount)
		return nil
	}

	var (
		processedCounter atomic.Int32
		errCh            = make(chan error, 1)
		buffer           = make([]byte, indexFileSize*oidSize)
		oidCh            = make(chan oid.ID, indexFileSize)
	)

	for range maxParallelSearches {
		go func() {
			for currOid := range oidCh {
				obj, err := p.ObjectHead(context.Background(), containerID, currOid, signer, client.PrmObjectHead{})
				if err != nil {
					select {
					case errCh <- fmt.Errorf("failed to get header for OID %s: %w", currOid.String(), err):
					default:
					}
					return
				}

				blockIndex, err := getBlockIndex(*obj, blockAttributeKey)
				if err != nil {
					select {
					case errCh <- fmt.Errorf("failed to process header for OID %s: %w", currOid.String(), err):
					default:
					}
					return
				}

				index := uint(blockIndex) / indexFileSize
				offset := (uint(blockIndex) - index*indexFileSize) * oidSize
				res := make([]byte, oidSize)
				currOid.Encode(res)
				copy(buffer[offset:], res)
				processedCounter.Add(1)
			}
		}()
	}

	for i := existingIndexCount; i < expectedIndexCount; i++ {
		startIndex := i * indexFileSize
		endIndex := startIndex + indexFileSize

		for j := int(startIndex); j < int(endIndex); j += searchBatchSize {
			remaining := int(endIndex) - j
			end := j + min(searchBatchSize, remaining)
			prm := client.PrmObjectSearch{}
			filters := object.NewSearchFilters()
			filters.AddFilter(blockAttributeKey, fmt.Sprintf("%d", j), object.MatchNumGE)
			filters.AddFilter(blockAttributeKey, fmt.Sprintf("%d", end), object.MatchNumLE)
			prm.SetFilters(filters)

			objectIDs, err = neofs.ObjectSearch(ctx.Context, &p, account.PrivateKey(), containerID.String(), prm)
			if err != nil {
				return fmt.Errorf("no OIDs found for index files: %w", err)
			}

			for _, currOid := range objectIDs {
				oidCh <- currOid
			}
		}

		for processedCounter.Load() < int32(indexFileSize) {
			select {
			case err := <-errCh:
				return err
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}

		processedCounter.Store(0)

		select {
		case err := <-errCh:
			return err
		default:
		}
		attrs := []object.Attribute{
			*object.NewAttribute(attributeKey, strconv.Itoa(int(i))),
			*object.NewAttribute("size", strconv.Itoa(int(indexFileSize))),
			*object.NewAttribute("timestamp", strconv.FormatInt(time.Now().Unix(), 10)),
			*object.NewAttribute("block-attribute", blockAttributeKey),
		}
		err = uploadObj(ctx.Context, p, user.NewAutoIDSignerRFC6979(account.PrivateKey().PrivateKey), account.PrivateKey().GetScriptHash(), containerID, buffer, attrs)
		if err != nil {
			return fmt.Errorf("failed to upload index file %d: %w", i, err)
		}
		fmt.Fprintf(ctx.App.Writer, "Uploaded index file %d\n", i)
	}
	close(oidCh)
	return nil
}

// uploadObj uploads the block to the container using the pool.
func uploadObj(ctx context.Context, p pool.Pool, signer user.Signer, owner util.Uint160, containerID cid.ID, objData []byte, attrs []object.Attribute) error {
	var (
		ownerID          user.ID
		hdr              object.Object
		ch               checksum.Checksum
		v                = new(version.Version)
		prmObjectPutInit client.PrmObjectPutInit
	)

	ownerID.SetScriptHash(owner)
	hdr.SetPayload(objData)
	hdr.SetPayloadSize(uint64(len(objData)))
	hdr.SetContainerID(containerID)
	hdr.SetOwnerID(&ownerID)
	hdr.SetAttributes(attrs...)
	hdr.SetCreationEpoch(1)
	v.SetMajor(1)
	hdr.SetVersion(v)

	checksum.Calculate(&ch, checksum.TZ, objData)
	hdr.SetPayloadHomomorphicHash(ch)
	checksum.Calculate(&ch, checksum.SHA256, objData)
	hdr.SetPayloadChecksum(ch)

	err := hdr.SetIDWithSignature(signer)
	if err != nil {
		return err
	}
	err = hdr.CheckHeaderVerificationFields()
	if err != nil {
		return err
	}

	writer, err := p.ObjectPutInit(ctx, hdr, signer, prmObjectPutInit)
	if err != nil {
		return fmt.Errorf("failed to initiate object upload: %w", err)
	}
	defer writer.Close()
	_, err = writer.Write(objData)
	if err != nil {
		return fmt.Errorf("failed to write object data: %w", err)
	}
	return nil
}

func getBlockIndex(header object.Object, attribute string) (int, error) {
	for _, attr := range header.UserAttributes() {
		if attr.Key() == attribute {
			return strconv.Atoi(attr.Value())
		}
	}
	return -1, fmt.Errorf("attribute %s not found", attribute)
}
