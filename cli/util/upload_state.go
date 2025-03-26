package util

import (
	"fmt"
	"strconv"
	"time"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/server"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	gio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func uploadState(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	cfg, err := options.GetConfigFromContext(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}
	attr := ctx.String("state-attribute")
	maxRetries := ctx.Uint("retries")
	debug := ctx.Bool("debug")

	acc, _, err := options.GetAccFromContext(ctx)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to load account: %v", err), 1)
	}

	signer, p, err := options.GetNeoFSClientPool(ctx, acc)
	if err != nil {
		return cli.Exit(err, 1)
	}
	defer p.Close()
	log, _, logCloser, err := options.HandleLoggingParams(ctx, cfg.ApplicationConfiguration)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if logCloser != nil {
		defer func() { _ = logCloser() }()
	}

	chain, store, prometheus, pprof, err := server.InitBCWithMetrics(cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		pprof.ShutDown()
		prometheus.ShutDown()
		chain.Close()
	}()

	if chain.GetConfig().Ledger.KeepOnlyLatestState || chain.GetConfig().Ledger.RemoveUntraceableBlocks {
		return cli.Exit("only full-state node is supported: disable KeepOnlyLatestState and RemoveUntraceableBlocks", 1)
	}
	syncInterval := cfg.ProtocolConfiguration.StateSyncInterval
	if syncInterval == 0 {
		syncInterval = core.DefaultStateSyncInterval
	}

	containerID, err := getContainer(ctx, p, strconv.Itoa(int(chain.GetConfig().Magic)), maxRetries, debug)
	if err != nil {
		return cli.Exit(err, 1)
	}

	stateObjCount, err := searchStateIndex(ctx, p, containerID, acc.PrivateKey(), attr, syncInterval, maxRetries, debug)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed searching existing states: %v", err), 1)
	}
	stateModule := chain.GetStateModule()
	currentHeight := int(stateModule.CurrentLocalHeight())
	currentStateIndex := currentHeight / syncInterval
	if currentStateIndex < stateObjCount {
		log.Info("no new states to upload",
			zap.Int("number of uploaded state objects", stateObjCount),
			zap.Int("latest state is uploaded for block", (stateObjCount-1)*syncInterval),
			zap.Int("current height", currentHeight),
			zap.Int("StateSyncInterval", syncInterval))
		return nil
	}
	log.Info("starting uploading",
		zap.Int("number of uploaded state objects", stateObjCount),
		zap.Int("next state to upload for block", stateObjCount*syncInterval),
		zap.Int("current height", currentHeight),
		zap.Int("StateSyncInterval", syncInterval),
		zap.Int("number of states to upload", currentStateIndex-stateObjCount))
	for state := stateObjCount; state <= currentStateIndex; state++ {
		height := uint32(state * syncInterval)
		stateRoot, err := stateModule.GetStateRoot(height)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to get state root for height %d: %v", height, err), 1)
		}
		h, err := chain.GetHeader(chain.GetHeaderHash(height))
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to get header %d: %v", height, err), 1)
		}

		var (
			hdr              object.Object
			prmObjectPutInit client.PrmObjectPutInit
			attrs            = []object.Attribute{
				*object.NewAttribute(attr, strconv.Itoa(int(height))),
				*object.NewAttribute("Timestamp", strconv.FormatInt(time.Now().Unix(), 10)),
				*object.NewAttribute("StateRoot", stateRoot.Root.StringLE()),
				*object.NewAttribute("StateSyncInterval", strconv.Itoa(syncInterval)),
				*object.NewAttribute("BlockTime", strconv.FormatUint(h.Timestamp, 10)),
			}
		)
		hdr.SetContainerID(containerID)
		hdr.SetOwner(signer.UserID())
		hdr.SetAttributes(attrs...)
		err = retry(func() error {
			writer, err := p.ObjectPutInit(ctx.Context, hdr, signer, prmObjectPutInit)
			if err != nil {
				return err
			}
			start := time.Now()
			wrt := gio.NewBinWriterFromIO(writer)
			wrt.WriteB(byte(0))
			wrt.WriteU32LE(uint32(chain.GetConfig().Magic))
			wrt.WriteU32LE(height)
			wrt.WriteBytes(stateRoot.Root[:])
			err = traverseMPT(stateRoot.Root, store, wrt)
			if err != nil {
				_ = writer.Close()
				return err
			}
			err = writer.Close()
			if err != nil {
				return err
			}
			duration := time.Since(start)
			res := writer.GetResult()
			log.Info("uploaded state object",
				zap.String("object ID", res.StoredObjectID().String()),
				zap.Uint32("height", height),
				zap.Duration("time spent", duration))
			return nil
		}, maxRetries, debug)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to upload object at height %d: %v", height, err), 1)
		}
	}
	return nil
}

func searchStateIndex(ctx *cli.Context, p neofs.PoolWrapper, containerID cid.ID, privKeys *keys.PrivateKey,
	attributeKey string, syncInterval int, maxRetries uint, debug bool,
) (int, error) {
	var (
		doneCh   = make(chan struct{})
		errCh    = make(chan error)
		objCount = 0
	)

	go func() {
		defer close(doneCh)
		for i := 0; ; i++ {
			indexIDs := searchObjects(ctx.Context, p, containerID, privKeys,
				attributeKey, uint(i*syncInterval), uint(i*syncInterval)+1, 1, maxRetries, debug, errCh)
			resOIDs := make([]oid.ID, 0, 1)
			for id := range indexIDs {
				resOIDs = append(resOIDs, id)
			}
			if len(resOIDs) == 0 {
				break
			}
			if len(resOIDs) > 1 {
				fmt.Fprintf(ctx.App.Writer, "WARN: %d duplicated state objects with %s: %d found: %s\n", len(resOIDs), attributeKey, i, resOIDs)
			}
			objCount++
		}
	}()
	select {
	case err := <-errCh:
		return objCount, err
	case <-doneCh:
		return objCount, nil
	}
}

func traverseMPT(root util.Uint256, store storage.Store, writer *gio.BinWriter) error {
	cache := storage.NewMemCachedStore(store)
	billet := mpt.NewBillet(root, mpt.ModeAll, mpt.DummySTTempStoragePrefix, cache)
	err := billet.Traverse(func(pathToNode []byte, node mpt.Node, nodeBytes []byte) bool {
		writer.WriteVarBytes(nodeBytes)
		return writer.Err != nil
	}, false)
	if err != nil {
		return fmt.Errorf("billet traversal error: %w", err)
	}
	return nil
}
