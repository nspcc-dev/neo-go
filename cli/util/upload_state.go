package util

import (
	"fmt"
	"strconv"
	"time"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/server"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	gio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	"github.com/nspcc-dev/neofs-sdk-go/object"
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

	chain, prometheus, pprof, err := server.InitBCWithMetrics(cfg, log)
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
	if syncInterval <= 0 {
		syncInterval = config.DefaultStateSyncInterval
	}

	containerID, err := getContainer(ctx, p, strconv.Itoa(int(chain.GetConfig().Magic)), maxRetries, debug)
	if err != nil {
		return cli.Exit(err, 1)
	}

	filters := object.NewSearchFilters()
	filters.AddFilter(attr, "0", object.MatchNumGE)
	results, errs := neofs.ObjectSearch(ctx.Context, p, acc.PrivateKey(), containerID, filters, []string{attr})

	var lastItem *client.SearchResultItem

loop:
	for {
		select {
		case item, ok := <-results:
			if !ok {
				break loop
			}
			lastItem = &item

		case err = <-errs:
			if err != nil {
				return cli.Exit(fmt.Sprintf("failed searching existing states: %v", err), 1)
			}
			break loop
		}
	}

	stateObjIndex := 0
	if lastItem != nil {
		height, err := strconv.ParseUint(lastItem.Attributes[0], 10, 32)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to parse state object height: %v", err), 1)
		}
		stateObjIndex = int(height) / syncInterval
	}

	stateModule := chain.GetStateModule()
	currentHeight := int(stateModule.CurrentLocalHeight())
	currentStateIndex := currentHeight / syncInterval
	if currentStateIndex <= stateObjIndex {
		log.Info("no new states to upload",
			zap.Int("number of uploaded state objects", stateObjIndex),
			zap.Int("latest state is uploaded for block", (stateObjIndex-1)*syncInterval),
			zap.Int("current height", currentHeight),
			zap.Int("StateSyncInterval", syncInterval))
		return nil
	}
	log.Info("starting uploading",
		zap.Int("number of uploaded state objects", stateObjIndex),
		zap.Int("next state to upload for block", stateObjIndex*syncInterval),
		zap.Int("current height", currentHeight),
		zap.Int("StateSyncInterval", syncInterval),
		zap.Int("number of states to upload", currentStateIndex-stateObjIndex))
	for state := stateObjIndex + 1; state <= currentStateIndex; state++ {
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
				object.NewAttribute(attr, strconv.Itoa(int(height))),
				object.NewAttribute("Timestamp", strconv.FormatInt(time.Now().Unix(), 10)),
				object.NewAttribute("StateRoot", stateRoot.Root.StringLE()),
				object.NewAttribute("StateSyncInterval", strconv.Itoa(syncInterval)),
				object.NewAttribute("BlockTime", strconv.FormatUint(h.Timestamp, 10)),
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
			err = traverseMPT(stateRoot.Root, stateModule, wrt)
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

func traverseMPT(root util.Uint256, stateModule core.StateRoot, writer *gio.BinWriter) error {
	stateModule.SeekStates(root, []byte{}, func(k, v []byte) bool {
		writer.WriteVarBytes(k)
		writer.WriteVarBytes(v)
		return writer.Err == nil
	})
	return writer.Err
}
