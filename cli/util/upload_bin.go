package util

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle/neofs"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/nspcc-dev/neofs-sdk-go/checksum"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	"github.com/nspcc-dev/neofs-sdk-go/container"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/netmap"
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
	// Size of object ID.
	oidSize = sha256.Size
)

// Constants related to retry mechanism.
const (
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
	numWorkers := ctx.Int("workers")
	maxParallelSearches := ctx.Int("searchers")
	maxRetries := int(ctx.Uint("retries"))
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

	var net netmap.NetworkInfo
	err = retry(func() error {
		var errNet error
		net, errNet = p.NetworkInfo(ctx.Context, client.PrmNetworkInfo{})
		return errNet
	}, maxRetries)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to get network info: %w", err), 1)
	}
	homomorphicHashingDisabled := net.HomomorphicHashingDisabled()

	var containerObj container.Container
	err = retry(func() error {
		containerObj, err = p.ContainerGet(ctx.Context, containerID, client.PrmContainerGet{})
		return err
	}, maxRetries)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to get container with ID %s: %w", containerID, err), 1)
	}
	containerMagic := containerObj.Attribute("Magic")

	v, err := rpc.GetVersion()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to get version from RPC: %v", err), 1)
	}
	magic := strconv.Itoa(int(v.Protocol.Network))
	if containerMagic != magic {
		return cli.Exit(fmt.Sprintf("container magic %s does not match the network magic %s", containerMagic, magic), 1)
	}

	currentBlockHeight, err := rpc.GetBlockCount()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to get current block height from RPC: %v", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, "Chain block height:", currentBlockHeight)

	oldestMissingBlockIndex, errBlock := fetchLatestMissingBlockIndex(ctx.Context, p, containerID, acc.PrivateKey(), attr, int(currentBlockHeight), maxParallelSearches, maxRetries)
	if errBlock != nil {
		return cli.Exit(fmt.Errorf("failed to fetch the oldest missing block index from container: %w", err), 1)
	}
	fmt.Fprintln(ctx.App.Writer, "First block of latest incomplete batch uploaded to NeoFS container:", oldestMissingBlockIndex)

	if !ctx.Bool("skip-blocks-uploading") {
		err = uploadBlocks(ctx, p, rpc, signer, containerID, acc, attr, oldestMissingBlockIndex, uint(currentBlockHeight), homomorphicHashingDisabled, numWorkers, maxRetries)
		if err != nil {
			return cli.Exit(fmt.Errorf("failed to upload blocks: %w", err), 1)
		}
	}

	err = uploadIndexFiles(ctx, p, containerID, acc, signer, uint(currentBlockHeight), attr, homomorphicHashingDisabled, maxParallelSearches, maxRetries)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to upload index files: %w", err), 1)
	}
	return nil
}

// retry function with exponential backoff.
func retry(action func() error, maxRetries int) error {
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

// fetchLatestMissingBlockIndex searches the container for the latest full batch of blocks
// starting from the currentHeight and going backwards. It returns the index of first block
// in the next batch.
func fetchLatestMissingBlockIndex(ctx context.Context, p *pool.Pool, containerID cid.ID, priv *keys.PrivateKey, attributeKey string, currentHeight int, maxParallelSearches, maxRetries int) (int, error) {
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
				}, maxRetries)
				results[i] = searchResult{startIndex: startIndex, endIndex: endIndex, numOIDs: len(objectIDs), err: err}
			}(i, startIndex, endIndex)
		}
		wg.Wait()

		for i := len(results) - 1; i >= 0; i-- {
			if results[i].err != nil {
				return 0, fmt.Errorf("blocks search failed for batch with indexes from %d to %d: %w", results[i].startIndex, results[i].endIndex-1, results[i].err)
			}
			if results[i].numOIDs < searchBatchSize {
				emptyBatchFound = true
				continue
			}
			if emptyBatchFound || (batch == numBatches && i == len(results)-1) {
				return results[i].startIndex + searchBatchSize, nil
			}
		}
	}
	return 0, nil
}

// uploadBlocks uploads the blocks to the container using the pool.
func uploadBlocks(ctx *cli.Context, p *pool.Pool, rpc *rpcclient.Client, signer user.Signer, containerID cid.ID, acc *wallet.Account, attr string, oldestMissingBlockIndex int, currentBlockHeight uint, homomorphicHashingDisabled bool, numWorkers, maxRetries int) error {
	if oldestMissingBlockIndex > int(currentBlockHeight) {
		fmt.Fprintf(ctx.App.Writer, "No new blocks to upload. Need to upload starting from %d, current height %d\n", oldestMissingBlockIndex, currentBlockHeight)
		return nil
	}
	for batchStart := oldestMissingBlockIndex; batchStart <= int(currentBlockHeight); batchStart += searchBatchSize {
		var (
			batchEnd = min(batchStart+searchBatchSize, int(currentBlockHeight)+1)
			errCh    = make(chan error)
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
					errGet := retry(func() error {
						var errGetBlock error
						blk, errGetBlock = rpc.GetBlockByIndex(uint32(blockIndex))
						if errGetBlock != nil {
							return fmt.Errorf("failed to fetch block %d: %w", blockIndex, errGetBlock)
						}
						return nil
					}, maxRetries)
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
						*object.NewAttribute(attr, strconv.Itoa(int(blk.Index))),
						*object.NewAttribute("Primary", strconv.Itoa(int(blk.PrimaryIndex))),
						*object.NewAttribute("Hash", blk.Hash().StringLE()),
						*object.NewAttribute("PrevHash", blk.PrevHash.StringLE()),
						*object.NewAttribute("Timestamp", strconv.FormatUint(blk.Timestamp, 10)),
					}

					objBytes := bw.Bytes()
					errRetr := retry(func() error {
						return uploadObj(ctx.Context, p, signer, acc.PrivateKey().GetScriptHash(), containerID, objBytes, attrs, homomorphicHashingDisabled)
					}, maxRetries)
					if errRetr != nil {
						select {
						case errCh <- errRetr:
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
		case err := <-errCh:
			return fmt.Errorf("upload error: %w", err)
		case <-doneCh:
		}

		fmt.Fprintf(ctx.App.Writer, "Successfully uploaded batch of blocks: from %d to %d\n", batchStart, batchEnd-1)
	}
	return nil
}

// uploadIndexFiles uploads missing index files to the container.
func uploadIndexFiles(ctx *cli.Context, p *pool.Pool, containerID cid.ID, account *wallet.Account, signer user.Signer, currentHeight uint, blockAttributeKey string, homomorphicHashingDisabled bool, maxParallelSearches, maxRetries int) error {
	attributeKey := ctx.String("index-attribute")
	indexFileSize := ctx.Uint("index-file-size")
	fmt.Fprintln(ctx.App.Writer, "Uploading index files...")

	prm := client.PrmObjectSearch{}
	filters := object.NewSearchFilters()
	filters.AddFilter(attributeKey, fmt.Sprintf("%d", 0), object.MatchNumGE)
	filters.AddFilter("IndexSize", fmt.Sprintf("%d", indexFileSize), object.MatchStringEqual)
	prm.SetFilters(filters)
	var objectIDs []oid.ID
	errSearch := retry(func() error {
		var errSearchIndex error
		objectIDs, errSearchIndex = neofs.ObjectSearch(ctx.Context, p, account.PrivateKey(), containerID.String(), prm)
		return errSearchIndex
	}, maxRetries)
	if errSearch != nil {
		return fmt.Errorf("index files search failed: %w", errSearch)
	}

	existingIndexCount := uint(len(objectIDs))
	expectedIndexCount := currentHeight / indexFileSize
	if existingIndexCount >= expectedIndexCount {
		fmt.Fprintf(ctx.App.Writer, "Index files are up to date. Existing: %d, expected: %d\n", existingIndexCount, expectedIndexCount)
		return nil
	}

	var (
		buffer   = make([]byte, indexFileSize*oidSize)
		doneCh   = make(chan struct{})
		errCh    = make(chan error)
		emptyOid = make([]byte, oidSize)
	)

	go func() {
		defer close(doneCh)

		// Main processing loop for each index file.
		for i := existingIndexCount; i < expectedIndexCount; i++ {
			// Start block parsing goroutines.
			var (
				// processedIndices is a mapping from position in buffer to the block index.
				processedIndices sync.Map
				wg               sync.WaitGroup
				oidCh            = make(chan oid.ID, 2*maxParallelSearches)
			)
			wg.Add(maxParallelSearches)
			for range maxParallelSearches {
				go func() {
					defer wg.Done()
					for id := range oidCh {
						var obj object.Object
						errRetr := retry(func() error {
							var errGet error
							obj, _, errGet = p.ObjectGetInit(ctx.Context, containerID, id, signer, client.PrmObjectGet{})
							return errGet
						}, maxRetries)
						if errRetr != nil {
							select {
							case errCh <- fmt.Errorf("failed to fetch object %s: %w", id.String(), errRetr):
							default:
							}
							return
						}
						blockIndex, err := getBlockIndex(obj, blockAttributeKey)
						if err != nil {
							select {
							case errCh <- fmt.Errorf("failed to get block index from object %s: %w", id.String(), err):
							default:
							}
							return
						}
						pos := uint(blockIndex) % indexFileSize
						if _, ok := processedIndices.LoadOrStore(pos, blockIndex); !ok {
							id.Encode(buffer[pos*oidSize:])
						}
					}
				}()
			}

			// Search for blocks within the index file range.
			startIndex := i * indexFileSize
			endIndex := startIndex + indexFileSize
			objIDs := searchObjects(ctx.Context, p, containerID, account, blockAttributeKey, startIndex, endIndex, maxParallelSearches, maxRetries, errCh)
			for id := range objIDs {
				oidCh <- id
			}
			close(oidCh)
			wg.Wait()
			fmt.Fprintf(ctx.App.Writer, "Index file %d generated, checking for the missing blocks...\n", i)

			// Check if there are empty OIDs in the generated index file. This may happen
			// if searchObjects has returned not all blocks within the requested range, ref.
			// #3645. In this case, retry the search for every missing object.
			var count int
			for idx := range indexFileSize {
				if _, ok := processedIndices.Load(idx); !ok {
					count++
					fmt.Fprintf(ctx.App.Writer, "Index file %d: fetching missing block %d\n", i, i*indexFileSize+idx)
					objIDs = searchObjects(ctx.Context, p, containerID, account, blockAttributeKey, i*indexFileSize+idx, i*indexFileSize+idx+1, 1, maxRetries, errCh)
					// Block object duplicates are allowed, we're OK with the first found result.
					id, ok := <-objIDs
					for range objIDs {
					}
					if !ok {
						select {
						case errCh <- fmt.Errorf("index file %d: block %d is missing from the storage", i, i*indexFileSize+idx):
						default:
						}
						return
					}
					processedIndices.Store(idx, id)
					id.Encode(buffer[idx*oidSize:])
				}
			}
			fmt.Fprintf(ctx.App.Writer, "%d missing block(s) processed for index file %d, uploading index file...\n", count, i)

			// Check if there are empty OIDs in the generated index file. If it happens at
			// this stage, then there's a bug in the code.
			for k := 0; k < len(buffer); k += oidSize {
				if slices.Compare(buffer[k:k+oidSize], emptyOid) == 0 {
					select {
					case errCh <- fmt.Errorf("empty OID found in index file %d at position %d (block index %d)", i, k/oidSize, i*indexFileSize+uint(k/oidSize)):
					default:
					}
					return
				}
			}

			// Upload index file.
			attrs := []object.Attribute{
				*object.NewAttribute(attributeKey, strconv.Itoa(int(i))),
				*object.NewAttribute("IndexSize", strconv.Itoa(int(indexFileSize))),
			}
			err := retry(func() error {
				return uploadObj(ctx.Context, p, signer, account.PrivateKey().GetScriptHash(), containerID, buffer, attrs, homomorphicHashingDisabled)
			}, maxRetries)
			if err != nil {
				select {
				case errCh <- fmt.Errorf("failed to upload index file %d: %w", i, err):
				default:
				}
				return
			}
			fmt.Fprintf(ctx.App.Writer, "Uploaded index file %d\n", i)
			clear(buffer)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-doneCh:
	}

	return nil
}

// searchObjects searches in parallel for objects with attribute GE startIndex and LT
// endIndex. It returns a buffered channel of resulting object IDs and closes it once
// OID search is finished. Errors are sent to errCh in a non-blocking way.
func searchObjects(ctx context.Context, p *pool.Pool, containerID cid.ID, account *wallet.Account, blockAttributeKey string, startIndex, endIndex uint, maxParallelSearches, maxRetries int, errCh chan error) chan oid.ID {
	var res = make(chan oid.ID, 2*searchBatchSize)
	go func() {
		var wg sync.WaitGroup
		defer close(res)

		for i := int(startIndex); i < int(endIndex); i += searchBatchSize * maxParallelSearches {
			for j := range maxParallelSearches {
				start := i + j*searchBatchSize
				end := start + searchBatchSize

				if start >= int(endIndex) {
					break
				}
				if end > int(endIndex) {
					end = int(endIndex)
				}

				wg.Add(1)
				go func(start, end int) {
					defer wg.Done()

					prm := client.PrmObjectSearch{}
					filters := object.NewSearchFilters()
					filters.AddFilter(blockAttributeKey, fmt.Sprintf("%d", start), object.MatchNumGE)
					filters.AddFilter(blockAttributeKey, fmt.Sprintf("%d", end), object.MatchNumLT)
					prm.SetFilters(filters)

					var objIDs []oid.ID
					err := retry(func() error {
						var errBlockSearch error
						objIDs, errBlockSearch = neofs.ObjectSearch(ctx, p, account.PrivateKey(), containerID.String(), prm)
						return errBlockSearch
					}, maxRetries)
					if err != nil {
						select {
						case errCh <- fmt.Errorf("failed to search for block(s) from %d to %d: %w", start, end, err):
						default:
						}
						return
					}

					for _, id := range objIDs {
						res <- id
					}
				}(start, end)
			}
			wg.Wait()
		}
	}()

	return res
}

// uploadObj uploads object to the container using provided settings.
func uploadObj(ctx context.Context, p *pool.Pool, signer user.Signer, owner util.Uint160, containerID cid.ID, objData []byte, attrs []object.Attribute, homomorphicHashingDisabled bool) error {
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
	if !homomorphicHashingDisabled {
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
	_, err = writer.Write(objData)
	if err != nil {
		_ = writer.Close()
		return fmt.Errorf("failed to write object data: %w", err)
	}
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close object writer: %w", err)
	}
	res := writer.GetResult()
	if res.StoredObjectID().Equals(oid.ID{}) {
		return fmt.Errorf("object ID is empty")
	}
	return nil
}

func getBlockIndex(header object.Object, attribute string) (int, error) {
	for _, attr := range header.UserAttributes() {
		if attr.Key() == attribute {
			value := attr.Value()
			blockIndex, err := strconv.Atoi(value)
			if err != nil {
				return -1, fmt.Errorf("attribute %s has invalid value: %s, error: %w", attribute, value, err)
			}
			return blockIndex, nil
		}
	}
	return -1, fmt.Errorf("attribute %s not found", attribute)
}
