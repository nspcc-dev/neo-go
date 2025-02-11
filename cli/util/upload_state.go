package util

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/server"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	"github.com/nspcc-dev/neofs-sdk-go/container"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/nspcc-dev/neofs-sdk-go/pool"
	"github.com/nspcc-dev/neofs-sdk-go/user"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

const (
	// defaultBatchUploadSize is the default size of the batch of state objects to upload.
	defaultBatchUploadSize = 1000
)

func traverseMPT(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	rpcNeoFS := ctx.StringSlice("fs-rpc-endpoint")
	containerIDStr := ctx.String("container")
	attr := ctx.String("state-attribute")
	maxRetries := ctx.Uint("retries")
	debug := ctx.Bool("debug")
	numWorkers := ctx.Int("workers")

	acc, _, err := options.GetAccFromContext(ctx)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to load account: %v", err), 1)
	}

	var containerID cid.ID
	if err = containerID.DecodeString(containerIDStr); err != nil {
		return cli.Exit(fmt.Sprintf("failed to decode container ID: %v", err), 1)
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

	logger := zap.NewExample()
	cfg, err := options.GetConfigFromContext(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}

	chain, store, err := server.InitBlockChain(cfg, logger)
	if err != nil {
		return cli.Exit(err, 1)
	}

	magic := strconv.Itoa(int(chain.GetConfig().Magic))
	if containerMagic != magic {
		return cli.Exit(fmt.Sprintf("container magic %s does not match the network magic %s", containerMagic, magic), 1)
	}

	lastUploadedStateBatch, err := searchStateLastBatch(ctx, pWrapper, containerID, acc.PrivateKey(), attr, maxRetries, debug)

	stateModule := chain.GetStateModule()
	currentHeight := stateModule.CurrentLocalHeight()
	if currentHeight <= lastUploadedStateBatch*defaultBatchUploadSize {
		fmt.Fprintf(ctx.App.Writer, "No new states to upload. Need to upload starting from %d, current height %d\n", lastUploadedStateBatch*defaultBatchUploadSize, currentHeight)
		return nil
	}

	fmt.Fprintf(ctx.App.Writer, "Latest state root found at height %d, current height %d\n", lastUploadedStateBatch*defaultBatchUploadSize, currentHeight)
	for batchStart := lastUploadedStateBatch * defaultBatchUploadSize; batchStart < currentHeight; batchStart += defaultBatchUploadSize {
		var (
			batchEnd = min(batchStart+defaultBatchUploadSize, currentHeight)
			errCh    = make(chan error)
			doneCh   = make(chan struct{})
			wg       sync.WaitGroup
		)
		fmt.Fprintf(ctx.App.Writer, "Processing batch from %d to %d\n", batchStart, batchEnd-1)
		wg.Add(numWorkers)
		for i := range numWorkers {
			go func(i uint32) {
				defer wg.Done()
				for blockIndex := batchStart + i; blockIndex < batchEnd; blockIndex += uint32(numWorkers) {
					stateRoot, err := stateModule.GetStateRoot(blockIndex)
					if err != nil {
						select {
						case errCh <- err:
						default:
						}
						return
					}

					nodes, err := traverseRawMPT(stateRoot.Root, store, mpt.ModeLatest)
					if err != nil {
						select {
						case errCh <- err:
						default:
						}
						return
					}
					objBytes, err := EncodeMPTNodes(nodes)
					if err != nil {
						select {
						case errCh <- err:
						default:
						}
						return
					}
					//h, err := chain.GetHeader(stateRoot.Hash())
					//if err != nil {
					//	select {
					//	case errCh <- err:
					//	default:
					//	}
					//	return
					//}
					attrs := []object.Attribute{
						*object.NewAttribute(attr, strconv.Itoa(int(blockIndex))),
						*object.NewAttribute("Timestamp", strconv.FormatInt(time.Now().Unix(), 10)),
						*object.NewAttribute("StateRoot", stateRoot.Root.StringLE()),
						//*object.NewAttribute("BlockTime", strconv.FormatUint(h.Timestamp, 10)),
					}
					_, err = uploadObj(ctx.Context, pWrapper, signer, containerID, objBytes, attrs)
					if err != nil {
						select {
						case errCh <- err:
						default:
						}
						return
					}
				}
			}(uint32(i))
		}
		go func() {
			wg.Wait()
			close(doneCh)
		}()
		select {
		case err := <-errCh:
			return cli.Exit(fmt.Sprintf("failed to process batch: %v", err), 1)
		case <-doneCh:
		}
	}
	err = store.Close()
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to close the DB: %w", err), 1)
	}
	return nil
}

func traverseRawMPT(root util.Uint256, store storage.Store, mode mpt.TrieMode) ([][]byte, error) {
	cache := storage.NewMemCachedStore(store)
	billet := mpt.NewBillet(root, mode, 0, cache)
	var nodes [][]byte

	err := billet.Traverse(func(pathToNode []byte, node mpt.Node, nodeBytes []byte) bool {
		nodes = append(nodes, nodeBytes)
		return false
	}, false)

	if err != nil {
		return nil, fmt.Errorf("failed to traverse MPT: %w", err)
	}
	return nodes, nil
}

// searchStateLastBatch searches for the last not empty batch (defaultBatchUploadSize) of state objects in the container.
func searchStateLastBatch(ctx *cli.Context, p poolWrapper, containerID cid.ID, privKeys *keys.PrivateKey, attributeKey string, maxRetries uint, debug bool) (uint32, error) {
	var (
		doneCh = make(chan struct{})
		errCh  = make(chan error)

		existingBatchStateCount = uint32(0)
	)
	go func() {
		defer close(doneCh)
		for i := 0; ; i++ {
			indexIDs := searchObjects(ctx.Context, p, containerID, privKeys, attributeKey, uint(i*defaultBatchUploadSize), uint(i+1)*defaultBatchUploadSize, 100, maxRetries, debug, errCh)
			resOIDs := make([]oid.ID, 0, 1)
			for id := range indexIDs {
				resOIDs = append(resOIDs, id)
				break
			}
			if len(resOIDs) == 0 {
				break
			}
			existingBatchStateCount++
		}
	}()
	select {
	case err := <-errCh:
		return existingBatchStateCount, err
	case <-doneCh:
		if existingBatchStateCount > 0 {
			return existingBatchStateCount - 1, nil
		}
		return 0, nil
	}
}

func EncodeMPTNodes(nodes [][]byte) ([]byte, error) {
	bw := io.NewBufBinWriter()
	bw.BinWriter.WriteVarUint(uint64(len(nodes)))
	if bw.Err != nil {
		return nil, fmt.Errorf("failed to encode node count: %w", bw.Err)
	}
	for _, n := range nodes {
		bw.BinWriter.WriteVarBytes(n) // Encodes length + data.
		if bw.Err != nil {
			return nil, fmt.Errorf("failed to encode MPT node: %w", bw.Err)
		}
	}
	return bw.Bytes(), nil
}
