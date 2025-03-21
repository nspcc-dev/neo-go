package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/urfave/cli/v2"
)

func dumpBin(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	cfg, err := options.GetConfigFromContext(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}
	log, _, logCloser, err := options.HandleLoggingParams(ctx, cfg.ApplicationConfiguration)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if logCloser != nil {
		defer func() { _ = logCloser() }()
	}
	count := uint32(ctx.Uint("count"))
	start := uint32(ctx.Uint("start"))

	chain, _, prometheus, pprof, err := InitBCWithMetrics(cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		pprof.ShutDown()
		prometheus.ShutDown()
		chain.Close()
	}()

	blocksCount := chain.BlockHeight() + 1
	if start+count > blocksCount {
		return cli.Exit(fmt.Errorf("chain is not that high (%d) to dump %d blocks starting from %d", blocksCount-1, count, start), 1)
	}
	if count == 0 {
		count = blocksCount - start
	}

	out := ctx.String("out")
	if out == "" {
		return cli.Exit("output directory is not specified", 1)
	}
	if _, err = os.Stat(out); os.IsNotExist(err) {
		if err = os.MkdirAll(out, os.ModePerm); err != nil {
			return cli.Exit(fmt.Sprintf("failed to create directory %s: %s", out, err), 1)
		}
	}
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to check directory %s: %s", out, err), 1)
	}

	for i := start; i < start+count; i++ {
		blk, err := chain.GetBlock(chain.GetHeaderHash(i))
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to get block %d: %s", i, err), 1)
		}
		filePath := filepath.Join(out, fmt.Sprintf("block-%d.bin", i))
		if err = saveBlockToFile(blk, filePath); err != nil {
			return cli.Exit(fmt.Sprintf("failed to save block %d to file %s: %s", i, filePath, err), 1)
		}
	}
	return nil
}

func saveBlockToFile(blk *block.Block, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := io.NewBinWriterFromIO(file)
	blk.EncodeBinary(writer)
	return writer.Err
}
