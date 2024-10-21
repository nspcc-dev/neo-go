package util

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"sync"
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
	// Number of objects to search in a batch for finding max block in container.
	searchBatchSize = 10000
	// Control the number of concurrent searches.
	maxParallelSearches = 40
	// Size of object ID.
	oidSize = sha256.Size

	// Number of workers to fetch and upload blocks concurrently.
	numWorkers = 100
)

// Constants related to retry mechanism.
const (
	// Maximum number of retries.
	maxRetries = 5
	// Initial backoff duration.
	initialBackoff = 500 * time.Millisecond
	// Backoff multiplier.
	backoffFactor = 2
	// Maximum backoff duration.
	maxBackoff = 20 * time.Second
)

// Constants related to NeoFS pool request timeouts.
// Such big values are used to avoid NeoFS pool timeouts during block search and upload.
const (
	defaultDialTimeout        = 10 * time.Minute
	defaultStreamTimeout      = 10 * time.Minute
	defaultHealthcheckTimeout = 10 * time.Second
)

func uploadBin(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	rpcNeoFS := ctx.StringSlice("fs-rpc-endpoint")
	containerIDStr := ctx.String("container")
	attr := ctx.String("block-attribute")
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

	params := pool.DefaultOptions()
	params.SetHealthcheckTimeout(defaultHealthcheckTimeout)
	params.SetNodeDialTimeout(defaultDialTimeout)
	params.SetNodeStreamTimeout(defaultStreamTimeout)
	p, err := pool.New(pool.NewFlatNodeParams(rpcNeoFS), signer, params)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to create NeoFS pool: %v", err), 1)
	}

	if err = p.Dial(context.Background()); err != nil {
		return cli.Exit(fmt.Sprintf("failed to dial NeoFS pool: %v", err), 1)
	}
	defer p.Close()
	net, err := p.NetworkInfo(ctx.Context, client.PrmNetworkInfo{})
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to get network info: %w", err), 1)
	}
	homomorphicHashingDisabled := net.HomomorphicHashingDisabled()
	lastMissingBlockIndex, err := fetchLatestMissingBlockIndex(ctx.Context, p, containerID, acc.PrivateKey(), attr, int(currentBlockHeight))
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to fetch the latest missing block index from container: %w", err), 1)
	}

	fmt.Fprintln(ctx.App.Writer, "First block of latest incomplete batch uploaded to NeoFS container:", lastMissingBlockIndex)

	if lastMissingBlockIndex > int(currentBlockHeight) {
		fmt.Fprintf(ctx.App.Writer, "No new blocks to upload. Need to upload starting from %d, current height %d\n", lastMissingBlockIndex, currentBlockHeight)
		return nil
	}

	for batchStart := lastMissingBlockIndex; batchStart <= int(currentBlockHeight); batchStart += searchBatchSize {
		var (
			batchEnd = min(batchStart+searchBatchSize, int(currentBlockHeight)+1)
			errorCh  = make(chan error)
			doneCh   = make(chan struct{})
			wg       sync.WaitGroup
		)
		fmt.Fprintf(ctx.App.Writer, "Processing batch from %d to %d\n", batchStart, batchEnd-1)
		wg.Add(numWorkers)
		for i := range numWorkers {
			go func(i int) {
				defer wg.Done()
				for blockIndex := batchStart + i; blockIndex < batchEnd; blockIndex += numWorkers {
					var blk *block.Block
					err = retry(func() error {
						blk, err = rpc.GetBlockByIndex(uint32(blockIndex))
						if err != nil {
							return fmt.Errorf("failed to fetch block %d: %w", blockIndex, err)
						}
						return nil
					})
					if err != nil {
						select {
						case errorCh <- err:
						default:
						}
						return
					}

					bw := io.NewBufBinWriter()
					blk.EncodeBinary(bw.BinWriter)
					if bw.Err != nil {
						errorCh <- fmt.Errorf("failed to encode block %d: %w", blockIndex, bw.Err)
						return
					}
					attrs := []object.Attribute{
						*object.NewAttribute(attr, strconv.Itoa(int(blk.Index))),
						*object.NewAttribute("primary", strconv.Itoa(int(blk.PrimaryIndex))),
						*object.NewAttribute("hash", blk.Hash().StringLE()),
						*object.NewAttribute("prevHash", blk.PrevHash.StringLE()),
						*object.NewAttribute("timestamp", strconv.FormatUint(blk.Timestamp, 10)),
					}

					err = retry(func() error {
						return uploadObj(ctx.Context, p, signer, acc.PrivateKey().GetScriptHash(), containerID, bw.Bytes(), attrs, homomorphicHashingDisabled)
					})
					if err != nil {
						select {
						case errorCh <- err:
						default:
						}
						return
					}
				}
			}(i)
		}

		go func() {
			wg.Wait()
			close(doneCh)
		}()

		select {
		case err := <-errorCh:
			return cli.Exit(fmt.Errorf("upload error: %w", err), 1)
		case <-doneCh:
		}

		fmt.Fprintf(ctx.App.Writer, "Successfully uploaded batch of blocks: from %d to %d\n", batchStart, batchEnd-1)
	}

	err = updateIndexFiles(ctx, p, containerID, *acc, signer, uint(currentBlockHeight), attr, homomorphicHashingDisabled)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to update index files after upload: %w", err), 1)
	}
	return nil
}

// retry function with exponential backoff.
func retry(action func() error) error {
	var err error
	backoff := initialBackoff
	for range maxRetries {
		if err = action(); err == nil {
			return nil // Success, no retry needed.
		}
		time.Sleep(backoff) // Backoff before retrying.
		backoff *= time.Duration(backoffFactor)
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	return err // Return the last error after exhausting retries.
}

type searchResult struct {
	startIndex int
	endIndex   int
	numOIDs    int
	err        error
}

// fetchLatestMissingBlockIndex searches the container for the last full block batch,
// starting from the currentHeight and going backwards.
func fetchLatestMissingBlockIndex(ctx context.Context, p *pool.Pool, containerID cid.ID, priv *keys.PrivateKey, attributeKey string, currentHeight int) (int, error) {
	var (
		wg              sync.WaitGroup
		numBatches      = currentHeight/searchBatchSize + 1
		emptyBatchFound bool
	)

	for batch := numBatches; batch > -maxParallelSearches; batch -= maxParallelSearches {
		results := make([]searchResult, maxParallelSearches)

		for i := range maxParallelSearches {
			startIndex := (batch + i) * searchBatchSize
			endIndex := startIndex + searchBatchSize
			if endIndex <= 0 {
				continue
			}
			if startIndex < 0 {
				startIndex = 0
			}

			wg.Add(1)
			go func(i, startIndex, endIndex int) {
				defer wg.Done()

				prm := client.PrmObjectSearch{}
				filters := object.NewSearchFilters()
				filters.AddFilter(attributeKey, fmt.Sprintf("%d", startIndex), object.MatchNumGE)
				filters.AddFilter(attributeKey, fmt.Sprintf("%d", endIndex), object.MatchNumLT)
				prm.SetFilters(filters)
				var (
					objectIDs []oid.ID
					err       error
				)
				err = retry(func() error {
					objectIDs, err = neofs.ObjectSearch(ctx, p, priv, containerID.String(), prm)
					return err
				})
				results[i] = searchResult{startIndex: startIndex, endIndex: endIndex, numOIDs: len(objectIDs), err: err}
			}(i, startIndex, endIndex)
		}
		wg.Wait()

		for i := len(results) - 1; i >= 0; i-- {
			if results[i].err != nil {
				return 0, fmt.Errorf("search of index files failed for batch with indexes from %d to %d: %w", results[i].startIndex, results[i].endIndex-1, results[i].err)
			}
			if results[i].numOIDs < searchBatchSize {
				emptyBatchFound = true
				continue
			}
			if emptyBatchFound || (batch == numBatches && i == len(results)-1) {
				return results[i].endIndex, nil
			}
		}
	}
	return 0, nil
}

// updateIndexFiles updates the index files in the container.
func updateIndexFiles(ctx *cli.Context, p *pool.Pool, containerID cid.ID, account wallet.Account, signer user.Signer, currentHeight uint, blockAttributeKey string, homomorphicHashingDisabled bool) error {
	attributeKey := ctx.String("index-attribute")
	indexFileSize := ctx.Uint("index-file-size")
	fmt.Fprintln(ctx.App.Writer, "Updating index files...")

	prm := client.PrmObjectSearch{}
	filters := object.NewSearchFilters()
	filters.AddFilter(attributeKey, fmt.Sprintf("%d", 0), object.MatchNumGE)
	filters.AddFilter("size", fmt.Sprintf("%d", indexFileSize), object.MatchStringEqual)
	prm.SetFilters(filters)
	var (
		objectIDs []oid.ID
		err       error
	)
	err = retry(func() error {
		objectIDs, err = neofs.ObjectSearch(ctx.Context, p, account.PrivateKey(), containerID.String(), prm)
		return err
	})
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
		errCh                 = make(chan error)
		buffer                = make([]byte, indexFileSize*oidSize)
		oidCh                 = make(chan oid.ID, indexFileSize)
		oidFetcherToProcessor = make(chan struct{}, indexFileSize)
	)
	defer close(oidCh)
	for range maxParallelSearches {
		go func() {
			for id := range oidCh {
				var obj *object.Object
				err = retry(func() error {
					obj, err = p.ObjectHead(context.Background(), containerID, id, signer, client.PrmObjectHead{})
					return err
				})
				if err != nil {
					select {
					case errCh <- fmt.Errorf("failed to fetch object %s: %w", id.String(), err):
					default:
					}
				}
				blockIndex, err := getBlockIndex(obj, blockAttributeKey)
				if err != nil {
					select {
					case errCh <- fmt.Errorf("failed to get block index from object %s: %w", id.String(), err):
					default:
					}
				}
				offset := (uint(blockIndex) % indexFileSize) * oidSize
				id.Encode(buffer[offset:])
				oidFetcherToProcessor <- struct{}{}
			}
		}()
	}

	for i := existingIndexCount; i < expectedIndexCount; i++ {
		startIndex := i * indexFileSize
		endIndex := startIndex + indexFileSize
		go func() {
			for j := int(startIndex); j < int(endIndex); j += searchBatchSize {
				remaining := int(endIndex) - j
				end := j + min(searchBatchSize, remaining)

				prm = client.PrmObjectSearch{}
				filters = object.NewSearchFilters()
				filters.AddFilter(blockAttributeKey, fmt.Sprintf("%d", j), object.MatchNumGE)
				filters.AddFilter(blockAttributeKey, fmt.Sprintf("%d", end), object.MatchNumLT)
				prm.SetFilters(filters)
				var objIDs []oid.ID
				err = retry(func() error {
					objIDs, err = neofs.ObjectSearch(ctx.Context, p, account.PrivateKey(), containerID.String(), prm)
					return err
				})

				if err != nil {
					errCh <- fmt.Errorf("failed to search for objects from %d to %d for index file %d: %w", j, end, i, err)
					return
				}

				for _, id := range objIDs {
					oidCh <- id
				}
			}
		}()

		var completed int
	waitLoop:
		for {
			select {
			case err := <-errCh:
				return err
			case <-oidFetcherToProcessor:
				completed++
				if completed == int(indexFileSize) {
					break waitLoop
				}
			}
		}
		attrs := []object.Attribute{
			*object.NewAttribute(attributeKey, strconv.Itoa(int(i))),
			*object.NewAttribute("size", strconv.Itoa(int(indexFileSize))),
		}
		err = uploadObj(ctx.Context, p, signer, account.PrivateKey().GetScriptHash(), containerID, buffer, attrs, homomorphicHashingDisabled)
		if err != nil {
			return fmt.Errorf("failed to upload index file %d: %w", i, err)
		}
		fmt.Fprintf(ctx.App.Writer, "Uploaded index file %d\n", i)
	}
	return nil
}

// uploadObj uploads the block to the container using the pool.
func uploadObj(ctx context.Context, p *pool.Pool, signer user.Signer, owner util.Uint160, containerID cid.ID, objData []byte, attrs []object.Attribute, HomomorphicHashingDisabled bool) error {
	var (
		ownerID          user.ID
		hdr              object.Object
		chSHA256         checksum.Checksum
		chHomomorphic    checksum.Checksum
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
	if !HomomorphicHashingDisabled {
		checksum.Calculate(&chHomomorphic, checksum.TZ, objData)
		hdr.SetPayloadHomomorphicHash(chHomomorphic)
	}
	checksum.Calculate(&chSHA256, checksum.SHA256, objData)
	hdr.SetPayloadChecksum(chSHA256)

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

func getBlockIndex(header *object.Object, attribute string) (int, error) {
	for _, attr := range header.UserAttributes() {
		if attr.Key() == attribute {
			return strconv.Atoi(attr.Value())
		}
	}
	return -1, fmt.Errorf("attribute %s not found", attribute)
}
