package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	stdio "io"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/nspcc-dev/neo-go/cli/cmdargs"
	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/consensus"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	corestate "github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/services/metrics"
	"github.com/nspcc-dev/neo-go/pkg/services/notary"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv"
	"github.com/nspcc-dev/neo-go/pkg/services/stateroot"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewCommands returns 'node' command.
func NewCommands() []*cli.Command {
	cfgFlags := []cli.Flag{options.Config, options.ConfigFile, options.RelativePath}
	cfgFlags = append(cfgFlags, options.Network...)

	var cfgWithCountFlags = slices.Clone(cfgFlags)
	cfgFlags = append(cfgFlags, options.Debug, options.ForceTimestampLogs)
	cfgWithCountFlags = append(cfgWithCountFlags,
		&cli.UintFlag{
			Name:    "count",
			Aliases: []string{"c"},
			Usage:   "Number of blocks to be processed (default or 0: all chain)",
		},
	)
	var cfgCountOutFlags = slices.Clone(cfgWithCountFlags)
	cfgCountOutFlags = append(cfgCountOutFlags,
		&cli.UintFlag{
			Name:    "start",
			Aliases: []string{"s"},
			Usage:   "Block number to start from",
		},
		&cli.StringFlag{
			Name:    "out",
			Aliases: []string{"o"},
			Usage:   "Output file (stdout if not given)",
		},
	)
	var cfgCountInFlags = slices.Clone(cfgWithCountFlags)
	cfgCountInFlags = append(cfgCountInFlags,
		&cli.StringFlag{
			Name:    "in",
			Aliases: []string{"i"},
			Usage:   "Input file (stdin if not given)",
		},
		&cli.StringFlag{
			Name:  "dump",
			Usage: "Directory for storing JSON dumps",
		},
	)
	var cfgHeightFlags = slices.Clone(cfgFlags)
	cfgHeightFlags = append(cfgHeightFlags, &cli.UintFlag{
		Name:     "height",
		Usage:    "Height of the state to reset DB to",
		Required: true,
	})
	return []*cli.Command{
		{
			Name:      "node",
			Usage:     "Start a NeoGo node",
			UsageText: "neo-go node [--config-path path] [-d] [-p/-m/-t] [--config-file file] [--force-timestamp-logs]",
			Action:    startServer,
			Flags:     cfgFlags,
		},
		{
			Name:  "db",
			Usage: "Database manipulations",
			Subcommands: []*cli.Command{
				{
					Name:      "dump",
					Usage:     "Dump blocks (starting with the genesis or specified block) to the file",
					UsageText: "neo-go db dump [-o file] [-s start] [-c count] [--config-path path] [-p/-m/-t] [--config-file file] [--force-timestamp-logs]",
					Action:    dumpDB,
					Flags: append(cfgCountOutFlags,
						&cli.BoolFlag{
							Name:  "non-incremental",
							Usage: "Force legacy (non-incremental) dump output",
						},
					),
				},
				{
					Name:      "dump-bin",
					Usage:     "Dump blocks (starting with the genesis or specified block) to the directory in binary format",
					UsageText: "neo-go db dump-bin -o directory [-s start] [-c count] [--config-path path] [-p/-m/-t] [--config-file file] [--force-timestamp-logs]",
					Action:    dumpBin,
					Flags:     cfgCountOutFlags,
				},
				{
					Name:      "restore",
					Usage:     "Restore blocks from the file",
					UsageText: "neo-go db restore [-i file] [--dump] [-c count] [--config-path path] [-p/-m/-t] [--config-file file] [--force-timestamp-logs]",
					Action:    restoreDB,
					Flags:     cfgCountInFlags,
				},
				{
					Name:      "reset",
					Usage:     "Reset database to the previous state",
					UsageText: "neo-go db reset --height height [--config-path path] [-p/-m/-t] [--config-file file] [--force-timestamp-logs]",
					Action:    resetDB,
					Flags:     cfgHeightFlags,
				},
			},
		},
	}
}

func newGraceContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	signal.Notify(stop, syscall.SIGTERM)
	go func() {
		<-stop
		cancel()
	}()
	return ctx
}

// InitBCWithMetrics initializes the blockchain with metrics with the given configuration.
func InitBCWithMetrics(cfg config.Config, log *zap.Logger) (*core.Blockchain, *metrics.Service, *metrics.Service, error) {
	chain, _, err := initBlockChain(cfg, log)
	if err != nil {
		return nil, nil, nil, cli.Exit(err, 1)
	}
	prometheus := metrics.NewPrometheusService(cfg.ApplicationConfiguration.Prometheus, log)
	pprof := metrics.NewPprofService(cfg.ApplicationConfiguration.Pprof, log)

	go chain.Run()
	err = prometheus.Start()
	if err != nil {
		return nil, nil, nil, cli.Exit(fmt.Errorf("failed to start Prometheus service: %w", err), 1)
	}
	err = pprof.Start()
	if err != nil {
		return nil, nil, nil, cli.Exit(fmt.Errorf("failed to start Pprof service: %w", err), 1)
	}

	return chain, prometheus, pprof, nil
}

func dumpDB(ctx *cli.Context) error {
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

	var outStream = os.Stdout
	if out := ctx.String("out"); out != "" {
		outStream, err = os.Create(out)
		if err != nil {
			return cli.Exit(err, 1)
		}
	}
	defer outStream.Close()
	writer := io.NewBinWriterFromIO(outStream)

	chain, prometheus, pprof, err := InitBCWithMetrics(cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		pprof.ShutDown()
		prometheus.ShutDown()
		chain.Close()
	}()

	chainCount := chain.BlockHeight() + 1
	if start+count > chainCount {
		return cli.Exit(fmt.Errorf("chain is not that high (%d) to dump %d blocks starting from %d", chainCount-1, count, start), 1)
	}
	if count == 0 {
		count = chainCount - start
	}
	if start != 0 || !ctx.Bool("non-incremental") {
		writer.WriteU32LE(start)
	}
	writer.WriteU32LE(count)
	err = chaindump.Dump(chain, writer, start, count)
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}
	return nil
}

func restoreDB(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	cfg, err := options.GetConfigFromContext(ctx)
	if err != nil {
		return err
	}
	log, _, logCloser, err := options.HandleLoggingParams(ctx, cfg.ApplicationConfiguration)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if logCloser != nil {
		defer func() { _ = logCloser() }()
	}
	count := uint32(ctx.Uint("count"))

	var inStream = os.Stdin
	if in := ctx.String("in"); in != "" {
		inStream, err = os.Open(in)
		if err != nil {
			return cli.Exit(err, 1)
		}
	}
	defer inStream.Close()
	reader := io.NewBinReaderFromIO(inStream)

	dumpDir := ctx.String("dump")
	if dumpDir != "" {
		cfg.ApplicationConfiguration.SaveStorageBatch = true
	}

	chain, prometheus, pprof, err := InitBCWithMetrics(cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		pprof.ShutDown()
		prometheus.ShutDown()
		chain.Close()
	}()

	var start = reader.ReadU32LE()
	if reader.Err != nil {
		return cli.Exit(err, 1)
	}
	var allBlocks = reader.ReadU32LE()
	if reader.Err != nil {
		return cli.Exit(err, 1)
	}
	var versionOrLen = reader.ReadU32LE()
	if reader.Err != nil {
		return cli.Exit(err, 1)
	}

	var buf []byte
	if versionOrLen == block.VersionInitial || versionOrLen == block.VersionFaun {
		// Block length > 0 => we read block version, so we have ordinary chain dump.
		buf = binary.LittleEndian.AppendUint32(buf, allBlocks)
		allBlocks = start
		start = 0
	}
	buf = binary.LittleEndian.AppendUint32(buf, versionOrLen)
	reader = io.NewBinReaderFromIO(stdio.MultiReader(
		bytes.NewReader(buf),
		inStream,
	))

	if chain.BlockHeight()+1 < start {
		return cli.Exit(fmt.Errorf("expected height: %d, dump starts at %d",
			chain.BlockHeight()+1, start), 1)
	}

	var skip uint32
	if chain.BlockHeight() != 0 {
		skip = chain.BlockHeight() + 1 - start
	}

	if skip+count > allBlocks {
		return cli.Exit(fmt.Errorf("input file has only %d blocks, can't read %d starting from %d", allBlocks, count, skip), 1)
	}
	if count == 0 {
		count = allBlocks - skip
	}
	log.Info("initialize restore",
		zap.Uint32("start", start),
		zap.Uint32("height", chain.BlockHeight()),
		zap.Uint32("skip", skip),
		zap.Uint32("count", count))

	gctx := newGraceContext()
	var lastIndex uint32
	dump := newDump()
	defer func() {
		_ = dump.tryPersist(dumpDir, lastIndex)
	}()

	var f = func(b *block.Block) error {
		select {
		case <-gctx.Done():
			return gctx.Err()
		default:
			return nil
		}
	}
	if dumpDir != "" {
		f = func(b *block.Block) error {
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
			}
			batch := chain.LastBatch()
			// The genesis block may already be persisted, so LastBatch() will return nil.
			if batch == nil && b.Index == 0 {
				return nil
			}
			dump.add(b.Index, batch)
			lastIndex = b.Index
			if b.Index%1000 == 0 {
				if err := dump.tryPersist(dumpDir, b.Index); err != nil {
					return fmt.Errorf("can't dump storage to file: %w", err)
				}
			}
			return nil
		}
	}

	err = chaindump.Restore(chain, reader, skip, count, f)
	if err != nil {
		return cli.Exit(fmt.Errorf("wrong dump file or settings mismatch: %w", err), 1)
	}
	return nil
}

func resetDB(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	cfg, err := options.GetConfigFromContext(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}
	h := uint32(ctx.Uint("height"))

	log, _, logCloser, err := options.HandleLoggingParams(ctx, cfg.ApplicationConfiguration)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if logCloser != nil {
		defer func() { _ = logCloser() }()
	}
	chain, store, err := initBlockChain(cfg, log)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to create Blockchain instance: %w", err), 1)
	}

	err = chain.Reset(h)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to reset chain state to height %d: %w", h, err), 1)
	}
	err = store.Close()
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to close the DB: %w", err), 1)
	}
	return nil
}

// oracleService is an interface representing Oracle service with network.Service
// capabilities and ability to submit oracle responses.
type oracleService interface {
	rpcsrv.OracleHandler
	network.Service
}

func mkOracle(config config.OracleConfiguration, magic netmode.Magic, chain *core.Blockchain, serv *network.Server, log *zap.Logger) (oracleService, error) {
	if !config.Enabled {
		return nil, nil
	}
	orcCfg := oracle.Config{
		Log:           log,
		Network:       magic,
		MainCfg:       config,
		Chain:         chain,
		OnTransaction: serv.RelayTxn,
	}
	orc, err := oracle.NewOracle(orcCfg)
	if err != nil {
		return nil, fmt.Errorf("can't initialize Oracle module: %w", err)
	}
	chain.SetOracle(orc)
	serv.AddService(orc)
	return orc, nil
}

func mkConsensus(config config.Consensus, chain *core.Blockchain, serv *network.Server, log *zap.Logger) (consensus.Service, error) {
	if !config.Enabled {
		return nil, nil
	}
	srv, err := consensus.NewService(consensus.Config{
		Logger:                log,
		Broadcast:             serv.BroadcastExtensible,
		Chain:                 chain,
		BlockQueue:            serv.GetBlockQueue(),
		ProtocolConfiguration: chain.GetConfig().ProtocolConfiguration,
		RequestTx:             serv.RequestTx,
		StopTxFlow:            serv.StopTxFlow,
		Wallet:                config.UnlockWallet,
	})
	if err != nil {
		return nil, fmt.Errorf("can't initialize Consensus module: %w", err)
	}

	serv.AddConsensusService(srv, srv.OnPayload, srv.OnTransaction)
	return srv, nil
}

func mkP2PNotary(config config.P2PNotary, chain *core.Blockchain, serv *network.Server, log *zap.Logger) (*notary.Notary, error) {
	if !config.Enabled {
		return nil, nil
	}
	if !chain.P2PSigExtensionsEnabled() {
		return nil, errors.New("P2PSigExtensions are disabled, but Notary service is enabled")
	}
	cfg := notary.Config{
		MainCfg: config,
		Chain:   chain,
		Log:     log,
	}
	n, err := notary.NewNotary(cfg, serv.Net, serv.GetNotaryPool(), func(tx *transaction.Transaction) error {
		err := serv.RelayTxn(tx)
		if err != nil && !errors.Is(err, core.ErrAlreadyExists) && !errors.Is(err, core.ErrAlreadyInPool) {
			return fmt.Errorf("can't relay completed notary transaction: hash %s, error: %w", tx.Hash().StringLE(), err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Notary module: %w", err)
	}
	serv.AddService(n)
	chain.SetNotary(n)
	return n, nil
}

func startServer(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}

	cfg, err := options.GetConfigFromContext(ctx)
	if err != nil {
		return cli.Exit(err, 1)
	}
	var logDebug = ctx.Bool("debug")
	log, logLevel, logCloser, err := options.HandleLoggingParams(ctx, cfg.ApplicationConfiguration)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if logCloser != nil {
		defer func() { _ = logCloser() }()
	}

	grace, cancel := context.WithCancel(newGraceContext())
	defer cancel()

	serverConfig, err := network.NewServerConfig(cfg)
	if err != nil {
		return cli.Exit(err, 1)
	}

	chain, prometheus, pprof, err := InitBCWithMetrics(cfg, log)
	if err != nil {
		return cli.Exit(err, 1)
	}
	defer func() {
		pprof.ShutDown()
		prometheus.ShutDown()
		chain.Close()
	}()

	serv, err := network.NewServer(serverConfig, chain, chain.GetStateSyncModule(), log)
	if err != nil {
		return cli.Exit(fmt.Errorf("failed to create network server: %w", err), 1)
	}
	srMod := chain.GetStateModule().(*corestate.Module) // Take full responsibility here.
	sr, err := stateroot.New(serverConfig.StateRootCfg, srMod, log, chain, serv.BroadcastExtensible)
	if err != nil {
		return cli.Exit(fmt.Errorf("can't initialize StateRoot service: %w", err), 1)
	}
	serv.AddExtensibleService(sr, stateroot.Category, sr.OnPayload)

	oracleSrv, err := mkOracle(cfg.ApplicationConfiguration.Oracle, cfg.ProtocolConfiguration.Magic, chain, serv, log)
	if err != nil {
		return cli.Exit(err, 1)
	}
	dbftSrv, err := mkConsensus(cfg.ApplicationConfiguration.Consensus, chain, serv, log)
	if err != nil {
		return cli.Exit(err, 1)
	}
	p2pNotary, err := mkP2PNotary(cfg.ApplicationConfiguration.P2PNotary, chain, serv, log)
	if err != nil {
		return cli.Exit(err, 1)
	}
	errChan := make(chan error)
	rpcServer := rpcsrv.New(chain, cfg.ApplicationConfiguration.RPC, serv, oracleSrv, log, errChan)
	serv.AddService(rpcServer)
	setNeoGoVersion(config.Version)
	serv.Start()
	if !cfg.ApplicationConfiguration.RPC.StartWhenSynchronized {
		// Run RPC server in a separate routine. This is necessary to avoid a potential
		// deadlock: Start() can write errors to errChan which is not yet read in the
		// current execution context (see for-loop below).
		go rpcServer.Start()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, sighup)
	signal.Notify(sigCh, sigusr1)
	signal.Notify(sigCh, sigusr2)

	fmt.Fprintln(ctx.App.Writer, Logo())
	fmt.Fprintln(ctx.App.Writer, serv.UserAgent)
	fmt.Fprintln(ctx.App.Writer)

	var shutdownErr error
Main:
	for {
		select {
		case err := <-errChan:
			shutdownErr = fmt.Errorf("server error: %w", err)
			cancel()
		case sig := <-sigCh:
			var newLogLevel = zapcore.InvalidLevel

			log.Info("signal received", zap.Stringer("name", sig))
			cfgnew, err := options.GetConfigFromContext(ctx)
			if err != nil {
				log.Warn("can't reread the config file, signal ignored", zap.Error(err))
				break // Continue working.
			}
			if !cfg.ProtocolConfiguration.Equals(&cfgnew.ProtocolConfiguration) {
				log.Warn("ProtocolConfiguration changed, signal ignored")
				break // Continue working.
			}
			if !cfg.ApplicationConfiguration.EqualsButServices(&cfgnew.ApplicationConfiguration) {
				log.Warn("ApplicationConfiguration changed in incompatible way, signal ignored")
				break // Continue working.
			}
			if !logDebug && cfgnew.ApplicationConfiguration.LogLevel != cfg.ApplicationConfiguration.LogLevel {
				newLogLevel, err = zapcore.ParseLevel(cfgnew.ApplicationConfiguration.LogLevel)
				if err != nil {
					log.Warn("wrong LogLevel in ApplicationConfiguration, signal ignored", zap.Error(err))
					break // Continue working.
				}
			}
			switch sig {
			case sighup:
				if newLogLevel != zapcore.InvalidLevel {
					logLevel.SetLevel(newLogLevel)
					log.Warn("using new logging level", zap.Stringer("level", newLogLevel))
				}
				serv.DelService(rpcServer)
				rpcServer.Shutdown()
				rpcServer = rpcsrv.New(chain, cfgnew.ApplicationConfiguration.RPC, serv, oracleSrv, log, errChan)
				serv.AddService(rpcServer)
				if !cfgnew.ApplicationConfiguration.RPC.StartWhenSynchronized || serv.IsInSync() {
					// Here similar to the initial run (see above for-loop), so async.
					go rpcServer.Start()
				}
				pprof.ShutDown()
				pprof = metrics.NewPprofService(cfgnew.ApplicationConfiguration.Pprof, log)
				err = pprof.Start()
				if err != nil {
					shutdownErr = fmt.Errorf("failed to start Pprof service: %w", err)
					cancel() // Fatal error, like for RPC server.
				}
				prometheus.ShutDown()
				prometheus = metrics.NewPrometheusService(cfgnew.ApplicationConfiguration.Prometheus, log)
				err = prometheus.Start()
				if err != nil {
					shutdownErr = fmt.Errorf("failed to start Prometheus service: %w", err)
					cancel() // Fatal error, like for RPC server.
				}
			case sigusr1:
				if oracleSrv != nil {
					serv.DelService(oracleSrv)
					chain.SetOracle(nil)
					rpcServer.SetOracleHandler(nil)
					oracleSrv.Shutdown()
				}
				oracleSrv, err = mkOracle(cfgnew.ApplicationConfiguration.Oracle, cfgnew.ProtocolConfiguration.Magic, chain, serv, log)
				if err != nil {
					log.Error("failed to create oracle service", zap.Error(err))
					break // Keep going.
				}
				if oracleSrv != nil {
					rpcServer.SetOracleHandler(oracleSrv)
					if serv.IsInSync() {
						oracleSrv.Start()
					}
				}
				if p2pNotary != nil {
					serv.DelService(p2pNotary)
					chain.SetNotary(nil)
					p2pNotary.Shutdown()
				}
				p2pNotary, err = mkP2PNotary(cfgnew.ApplicationConfiguration.P2PNotary, chain, serv, log)
				if err != nil {
					log.Error("failed to create notary service", zap.Error(err))
					break // Keep going.
				}
				if p2pNotary != nil && serv.IsInSync() {
					p2pNotary.Start()
				}
				serv.DelExtensibleService(sr, stateroot.Category)
				srMod.SetUpdateValidatorsCallback(nil)
				sr.Shutdown()
				sr, err = stateroot.New(cfgnew.ApplicationConfiguration.StateRoot, srMod, log, chain, serv.BroadcastExtensible)
				if err != nil {
					log.Error("failed to create state validation service", zap.Error(err))
					break // The show must go on.
				}
				serv.AddExtensibleService(sr, stateroot.Category, sr.OnPayload)
				if serv.IsInSync() {
					sr.Start()
				}
			case sigusr2:
				if dbftSrv != nil {
					serv.DelConsensusService(dbftSrv)
					dbftSrv.Shutdown()
				}
				dbftSrv, err = mkConsensus(cfgnew.ApplicationConfiguration.Consensus, chain, serv, log)
				if err != nil {
					log.Error("failed to create consensus service", zap.Error(err))
					break // Whatever happens, I'll leave it all to chance.
				}
				if dbftSrv != nil && serv.IsInSync() {
					dbftSrv.Start()
				}
			}
			cfg = cfgnew
		case <-grace.Done():
			signal.Stop(sigCh)
			serv.Shutdown()
			break Main
		}
	}

	if shutdownErr != nil {
		return cli.Exit(shutdownErr, 1)
	}

	return nil
}

// initBlockChain initializes BlockChain with preselected DB.
func initBlockChain(cfg config.Config, log *zap.Logger) (*core.Blockchain, storage.Store, error) {
	store, err := storage.NewStore(cfg.ApplicationConfiguration.DBConfiguration)
	if err != nil {
		return nil, nil, cli.Exit(fmt.Errorf("could not initialize storage: %w", err), 1)
	}

	chain, err := core.NewBlockchain(store, cfg.Blockchain(), log)
	if err != nil {
		errText := "could not initialize blockchain: %w"
		errArgs := []any{err}
		closeErr := store.Close()
		if closeErr != nil {
			errText += "; failed to close the DB: %w"
			errArgs = append(errArgs, closeErr)
		}

		return nil, nil, cli.Exit(fmt.Errorf(errText, errArgs...), 1)
	}
	return chain, store, nil
}

// Logo returns NeoGo logo.
func Logo() string {
	return `
    _   ____________        __________
   / | / / ____/ __ \      / ____/ __ \
  /  |/ / __/ / / / /_____/ / __/ / / /
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /
/_/ |_/_____/\____/      \____/\____/
`
}
