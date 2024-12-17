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
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
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

// poolWrapper wraps a NeoFS pool to adapt its Close method to return an error.
type poolWrapper struct {
	*pool.Pool
}

// Close closes the pool and returns nil.
func (p poolWrapper) Close() error {
	p.Pool.Close()
	return nil
}

func uploadBin(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	rpcNeoFS := ctx.StringSlice("fs-rpc-endpoint")
	containerIDStr := ctx.String("container")
	attr := ctx.String("block-attribute")
	numWorkers := ctx.Uint("workers")
	maxParallelSearches := ctx.Uint("searchers")
	maxRetries := ctx.Uint("retries")
	debug := ctx.Bool("debug")
	indexFileSize := ctx.Uint("index-file-size")
	indexAttrKey := ctx.String("index-attribute")
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
	params.SetHealthcheckTimeout(neofs.DefaultHealthcheckTimeout)
	params.SetNodeDialTimeout(neofs.DefaultDialTimeout)
	params.SetNodeStreamTimeout(neofs.DefaultStreamTimeout)
	p, err := pool.New(pool.NewFlatNodeParams(rpcNeoFS), signer, params)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to create NeoFS pool: %v", err), 1)
	}
	pWrapper := poolWrapper{p}
	if err = pWrapper.Dial(context.Background()); err != nil {
		return cli.Exit(fmt.Sprintf("failed to dial NeoFS pool: %v", err), 1)
	}
	defer p.Close()

	var containerObj container.Container
	err = retry(func() error {
		containerObj, err = p.ContainerGet(ctx.Context, containerID, client.PrmContainerGet{})
		return err
	}, maxRetries, debug)
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
	i, buf, err := searchIndexFile(ctx, pWrapper, containerID, acc.PrivateKey(), signer, indexFileSize, attr, indexAttrKey, maxParallelSearches, maxRetries, debug)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to find objects: %w", err), 1)
	}

	err = uploadBlocksAndIndexFiles(ctx, pWrapper, rpc, signer, containerID, attr, indexAttrKey, buf, i, indexFileSize, uint(currentBlockHeight), numWorkers, maxRetries, debug)
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

// uploadBlocksAndIndexFiles uploads the blocks and index files to the container using the pool.
func uploadBlocksAndIndexFiles(ctx *cli.Context, p poolWrapper, rpc *rpcclient.Client, signer user.Signer, containerID cid.ID, attr, indexAttributeKey string, buf []byte, currentIndexFileID, indexFileSize, currentBlockHeight uint, numWorkers, maxRetries uint, debug bool) error {
	if currentIndexFileID*indexFileSize >= currentBlockHeight {
		fmt.Fprintf(ctx.App.Writer, "No new blocks to upload. Need to upload starting from %d, current height %d\n", currentIndexFileID*indexFileSize, currentBlockHeight)
		return nil
	}
	fmt.Fprintln(ctx.App.Writer, "Uploading blocks and index files...")
	for indexFileStart := currentIndexFileID * indexFileSize; indexFileStart < currentBlockHeight; indexFileStart += indexFileSize {
		var (
			indexFileEnd = min(indexFileStart+indexFileSize, currentBlockHeight)
			errCh        = make(chan error)
			doneCh       = make(chan struct{})
			wg           sync.WaitGroup
		)
		fmt.Fprintf(ctx.App.Writer, "Processing batch from %d to %d\n", indexFileStart, indexFileEnd-1)
		wg.Add(int(numWorkers))
		for i := range numWorkers {
			go func(i uint) {
				defer wg.Done()
				for blockIndex := indexFileStart + i; blockIndex < indexFileEnd; blockIndex += numWorkers {
					if !oid.ID(buf[blockIndex%indexFileSize*oid.Size : blockIndex%indexFileSize*oid.Size+oid.Size]).IsZero() {
						if debug {
							fmt.Fprintf(ctx.App.Writer, "Block %d is already uploaded\n", blockIndex)
						}
						continue
					}
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
						*object.NewAttribute(attr, strconv.Itoa(int(blk.Index))),
						*object.NewAttribute("Primary", strconv.Itoa(int(blk.PrimaryIndex))),
						*object.NewAttribute("Hash", blk.Hash().StringLE()),
						*object.NewAttribute("PrevHash", blk.PrevHash.StringLE()),
						*object.NewAttribute("BlockTime", strconv.FormatUint(blk.Timestamp, 10)),
						*object.NewAttribute("Timestamp", strconv.FormatInt(time.Now().Unix(), 10)),
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
					copy(buf[blockIndex%indexFileSize*oid.Size:], resOid[:])
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
		fmt.Fprintf(ctx.App.Writer, "Successfully processed batch of blocks: from %d to %d\n", indexFileStart, indexFileEnd-1)

		// Additional check for empty OIDs in the buffer.
		for k := uint(0); k < (indexFileEnd-indexFileStart)*oid.Size; k += oid.Size {
			if oid.ID(buf[k : k+oid.Size]).IsZero() {
				return fmt.Errorf("empty OID found in index file %d at position %d (block index %d)", indexFileStart/indexFileSize, k/oid.Size, indexFileStart/indexFileSize*indexFileSize+k/oid.Size)
			}
		}
		if indexFileEnd-indexFileStart == indexFileSize {
			attrs := []object.Attribute{
				*object.NewAttribute(indexAttributeKey, strconv.Itoa(int(indexFileStart/indexFileSize))),
				*object.NewAttribute("IndexSize", strconv.Itoa(int(indexFileSize))),
				*object.NewAttribute("Timestamp", strconv.FormatInt(time.Now().Unix(), 10)),
			}
			err := retry(func() error {
				var errUpload error
				_, errUpload = uploadObj(ctx.Context, p, signer, containerID, buf, attrs)
				return errUpload
			}, maxRetries, debug)
			if err != nil {
				return fmt.Errorf("failed to upload index file: %w", err)
			}
			fmt.Println("Successfully uploaded index file ", indexFileStart/indexFileSize)
		}
		clear(buf)
	}
	return nil
}

// searchIndexFile returns the ID and buffer for the next index file to be uploaded.
func searchIndexFile(ctx *cli.Context, p poolWrapper, containerID cid.ID, privKeys *keys.PrivateKey, signer user.Signer, indexFileSize uint, blockAttributeKey, attributeKey string, maxParallelSearches, maxRetries uint, debug bool) (uint, []byte, error) {
	var (
		// buf is used to store OIDs of the uploaded blocks.
		buf    = make([]byte, indexFileSize*oid.Size)
		doneCh = make(chan struct{})
		errCh  = make(chan error)

		existingIndexCount = uint(0)
		filters            = object.NewSearchFilters()
	)
	go func() {
		defer close(doneCh)
		// Search for existing index files.
		filters.AddFilter("IndexSize", fmt.Sprintf("%d", indexFileSize), object.MatchStringEqual)
		for i := 0; ; i++ {
			indexIDs := searchObjects(ctx.Context, p, containerID, privKeys, attributeKey, uint(i), uint(i+1), 1, maxRetries, debug, errCh, filters)
			resOIDs := make([]oid.ID, 0, 1)
			for id := range indexIDs {
				resOIDs = append(resOIDs, id)
			}
			if len(resOIDs) == 0 {
				break
			}
			if len(resOIDs) > 1 {
				fmt.Fprintf(ctx.App.Writer, "WARN: %d duplicated index files with index %d found: %s\n", len(resOIDs), i, resOIDs)
			}
			existingIndexCount++
		}
		fmt.Fprintf(ctx.App.Writer, "Current index files count: %d\n", existingIndexCount)

		// Start block parsing goroutines.
		var (
			// processedIndices is a mapping from position in buffer to the block index.
			// It prevents duplicates.
			processedIndices sync.Map
			wg               sync.WaitGroup
			oidCh            = make(chan oid.ID, 2*maxParallelSearches)
		)
		wg.Add(int(maxParallelSearches))
		for range maxParallelSearches {
			go func() {
				defer wg.Done()
				for id := range oidCh {
					var obj object.Object
					errRetr := retry(func() error {
						var errGet error
						obj, _, errGet = p.ObjectGetInit(ctx.Context, containerID, id, signer, client.PrmObjectGet{})
						return errGet
					}, maxRetries, debug)
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
						copy(buf[pos*oid.Size:], id[:])
					}
				}
			}()
		}

		// Search for blocks within the index file range.
		objIDs := searchObjects(ctx.Context, p, containerID, privKeys, blockAttributeKey, existingIndexCount*indexFileSize, (existingIndexCount+1)*indexFileSize, maxParallelSearches, maxRetries, debug, errCh)
		for id := range objIDs {
			oidCh <- id
		}
		close(oidCh)
		wg.Wait()
	}()

	select {
	case err := <-errCh:
		return existingIndexCount, nil, err
	case <-doneCh:
		return existingIndexCount, buf, nil
	}
}

// searchObjects searches in parallel for objects with attribute GE startIndex and LT
// endIndex. It returns a buffered channel of resulting object IDs and closes it once
// OID search is finished. Errors are sent to errCh in a non-blocking way.
func searchObjects(ctx context.Context, p poolWrapper, containerID cid.ID, privKeys *keys.PrivateKey, blockAttributeKey string, startIndex, endIndex, maxParallelSearches, maxRetries uint, debug bool, errCh chan error, additionalFilters ...object.SearchFilters) chan oid.ID {
	var res = make(chan oid.ID, 2*neofs.DefaultSearchBatchSize)
	go func() {
		var wg sync.WaitGroup
		defer close(res)

		for i := startIndex; i < endIndex; i += neofs.DefaultSearchBatchSize * maxParallelSearches {
			for j := range maxParallelSearches {
				start := i + j*neofs.DefaultSearchBatchSize
				end := start + neofs.DefaultSearchBatchSize

				if start >= endIndex {
					break
				}
				if end > endIndex {
					end = endIndex
				}

				wg.Add(1)
				go func(start, end uint) {
					defer wg.Done()

					prm := client.PrmObjectSearch{}
					filters := object.NewSearchFilters()
					if len(additionalFilters) != 0 {
						filters = additionalFilters[0]
					}
					if end == start+1 {
						filters.AddFilter(blockAttributeKey, fmt.Sprintf("%d", start), object.MatchStringEqual)
					} else {
						filters.AddFilter(blockAttributeKey, fmt.Sprintf("%d", start), object.MatchNumGE)
						filters.AddFilter(blockAttributeKey, fmt.Sprintf("%d", end), object.MatchNumLT)
					}
					prm.SetFilters(filters)

					var objIDs []oid.ID
					err := retry(func() error {
						var errBlockSearch error
						objIDs, errBlockSearch = neofs.ObjectSearch(ctx, p, privKeys, containerID.String(), prm)
						return errBlockSearch
					}, maxRetries, debug)
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
func uploadObj(ctx context.Context, p poolWrapper, signer user.Signer, containerID cid.ID, objData []byte, attrs []object.Attribute) (oid.ID, error) {
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
