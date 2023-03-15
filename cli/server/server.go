package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewCommands returns 'node' command.
func NewCommands() []cli.Command {
	cfgFlags := []cli.Flag{options.Config}
	cfgFlags = append(cfgFlags, options.Network...)
	var cfgWithCountFlags = make([]cli.Flag, len(cfgFlags))
	copy(cfgWithCountFlags, cfgFlags)
	cfgFlags = append(cfgFlags, options.Debug)

	cfgWithCountFlags = append(cfgWithCountFlags,
		cli.UintFlag{
			Name:  "count, c",
			Usage: "number of blocks to be processed (default or 0: all chain)",
		},
	)
	var cfgCountOutFlags = make([]cli.Flag, len(cfgWithCountFlags))
	copy(cfgCountOutFlags, cfgWithCountFlags)
	cfgCountOutFlags = append(cfgCountOutFlags,
		cli.UintFlag{
			Name:  "start, s",
			Usage: "block number to start from (default: 0)",
		},
		cli.StringFlag{
			Name:  "out, o",
			Usage: "Output file (stdout if not given)",
		},
	)
	var cfgCountInFlags = make([]cli.Flag, len(cfgWithCountFlags))
	copy(cfgCountInFlags, cfgWithCountFlags)
	cfgCountInFlags = append(cfgCountInFlags,
		cli.StringFlag{
			Name:  "in, i",
			Usage: "Input file (stdin if not given)",
		},
		cli.StringFlag{
			Name:  "dump",
			Usage: "directory for storing JSON dumps",
		},
		cli.BoolFlag{
			Name:  "incremental, n",
			Usage: "use if dump is incremental",
		},
	)
	var cfgHeightFlags = make([]cli.Flag, len(cfgFlags)+1)
	copy(cfgHeightFlags, cfgFlags)
	cfgHeightFlags[len(cfgHeightFlags)-1] = cli.UintFlag{
		Name:     "height",
		Usage:    "Height of the state to reset DB to",
		Required: true,
	}
	return []cli.Command{
		{
			Name:      "node",
			Usage:     "start a NeoGo node",
			UsageText: "neo-go node [--config-path path] [-d] [-p/-m/-t]",
			Action:    startServer,
			Flags:     cfgFlags,
		},
		{
			Name:  "db",
			Usage: "database manipulations",
			Subcommands: []cli.Command{
				{
					Name:      "dump",
					Usage:     "dump blocks (starting with block #1) to the file",
					UsageText: "neo-go db dump -o file [-s start] [-c count] [--config-path path] [-p/-m/-t]",
					Action:    dumpDB,
					Flags:     cfgCountOutFlags,
				},
				{
					Name:      "restore",
					Usage:     "restore blocks from the file",
					UsageText: "neo-go db restore -i file [--dump] [-n] [-c count] [--config-path path] [-p/-m/-t]",
					Action:    restoreDB,
					Flags:     cfgCountInFlags,
				},
				{
					Name:      "reset",
					Usage:     "reset database to the previous state",
					UsageText: "neo-go db reset --height height [--config-path path] [-p/-m/-t]",
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

func initBCWithMetrics(cfg config.Config, log *zap.Logger) (*core.Blockchain, *metrics.Service, *metrics.Service, error) {
	chain, _, err := initBlockChain(cfg, log)
	if err != nil {
		return nil, nil, nil, cli.NewExitError(err, 1)
	}
	configureAddresses(&cfg.ApplicationConfiguration)
	prometheus := metrics.NewPrometheusService(cfg.ApplicationConfiguration.Prometheus, log)
	pprof := metrics.NewPprofService(cfg.ApplicationConfiguration.Pprof, log)

	go chain.Run()
	err = prometheus.Start()
	if err != nil {
		return nil, nil, nil, cli.NewExitError(fmt.Errorf("failed to start Prometheus service: %w", err), 1)
	}
	err = pprof.Start()
	if err != nil {
		return nil, nil, nil, cli.NewExitError(fmt.Errorf("failed to start Pprof service: %w", err), 1)
	}

	return chain, prometheus, pprof, nil
}

func dumpDB(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	cfg, err := options.GetConfigFromContext(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	log, _, logCloser, err := options.HandleLoggingParams(ctx.Bool("debug"), cfg.ApplicationConfiguration)
	if err != nil {
		return cli.NewExitError(err, 1)
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
			return cli.NewExitError(err, 1)
		}
	}
	defer outStream.Close()
	writer := io.NewBinWriterFromIO(outStream)

	chain, prometheus, pprof, err := initBCWithMetrics(cfg, log)
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
		return cli.NewExitError(fmt.Errorf("chain is not that high (%d) to dump %d blocks starting from %d", chainCount-1, count, start), 1)
	}
	if count == 0 {
		count = chainCount - start
	}
	writer.WriteU32LE(count)
	err = chaindump.Dump(chain, writer, start, count)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
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
	log, _, logCloser, err := options.HandleLoggingParams(ctx.Bool("debug"), cfg.ApplicationConfiguration)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if logCloser != nil {
		defer func() { _ = logCloser() }()
	}
	count := uint32(ctx.Uint("count"))

	var inStream = os.Stdin
	if in := ctx.String("in"); in != "" {
		inStream, err = os.Open(in)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	defer inStream.Close()
	reader := io.NewBinReaderFromIO(inStream)

	dumpDir := ctx.String("dump")
	if dumpDir != "" {
		cfg.ApplicationConfiguration.SaveStorageBatch = true
	}

	chain, prometheus, pprof, err := initBCWithMetrics(cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		pprof.ShutDown()
		prometheus.ShutDown()
		chain.Close()
	}()

	var start uint32
	if ctx.Bool("incremental") {
		start = reader.ReadU32LE()
		if chain.BlockHeight()+1 < start {
			return cli.NewExitError(fmt.Errorf("expected height: %d, dump starts at %d",
				chain.BlockHeight()+1, start), 1)
		}
	}

	var skip uint32
	if chain.BlockHeight() != 0 {
		skip = chain.BlockHeight() + 1 - start
	}

	var allBlocks = reader.ReadU32LE()
	if reader.Err != nil {
		return cli.NewExitError(err, 1)
	}
	if skip+count > allBlocks {
		return cli.NewExitError(fmt.Errorf("input file has only %d blocks, can't read %d starting from %d", allBlocks, count, skip), 1)
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
		return cli.NewExitError(err, 1)
	}
	return nil
}

func resetDB(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	cfg, err := options.GetConfigFromContext(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	h := uint32(ctx.Uint("height"))

	log, _, logCloser, err := options.HandleLoggingParams(ctx.Bool("debug"), cfg.ApplicationConfiguration)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if logCloser != nil {
		defer func() { _ = logCloser() }()
	}
	chain, store, err := initBlockChain(cfg, log)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to create Blockchain instance: %w", err), 1)
	}

	err = chain.Reset(h)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to reset chain state to height %d: %w", h, err), 1)
	}
	err = store.Close()
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to close the DB: %w", err), 1)
	}
	return nil
}

func mkOracle(config config.OracleConfiguration, magic netmode.Magic, chain *core.Blockchain, serv *network.Server, log *zap.Logger) (*oracle.Oracle, error) {
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

func mkConsensus(config config.Consensus, tpb time.Duration, chain *core.Blockchain, serv *network.Server, log *zap.Logger) (consensus.Service, error) {
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
		TimePerBlock:          tpb,
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
		if err != nil && !errors.Is(err, core.ErrAlreadyExists) {
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
		return cli.NewExitError(err, 1)
	}
	var logDebug = ctx.Bool("debug")
	log, logLevel, logCloser, err := options.HandleLoggingParams(logDebug, cfg.ApplicationConfiguration)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if logCloser != nil {
		defer func() { _ = logCloser() }()
	}

	grace, cancel := context.WithCancel(newGraceContext())
	defer cancel()

	serverConfig, err := network.NewServerConfig(cfg)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	chain, prometheus, pprof, err := initBCWithMetrics(cfg, log)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer func() {
		pprof.ShutDown()
		prometheus.ShutDown()
		chain.Close()
	}()

	serv, err := network.NewServer(serverConfig, chain, chain.GetStateSyncModule(), log)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("failed to create network server: %w", err), 1)
	}
	srMod := chain.GetStateModule().(*corestate.Module) // Take full responsibility here.
	sr, err := stateroot.New(serverConfig.StateRootCfg, srMod, log, chain, serv.BroadcastExtensible)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("can't initialize StateRoot service: %w", err), 1)
	}
	serv.AddExtensibleService(sr, stateroot.Category, sr.OnPayload)

	oracleSrv, err := mkOracle(cfg.ApplicationConfiguration.Oracle, cfg.ProtocolConfiguration.Magic, chain, serv, log)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	dbftSrv, err := mkConsensus(cfg.ApplicationConfiguration.Consensus, serverConfig.TimePerBlock, chain, serv, log)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	p2pNotary, err := mkP2PNotary(cfg.ApplicationConfiguration.P2PNotary, chain, serv, log)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	errChan := make(chan error)
	rpcServer := rpcsrv.New(chain, cfg.ApplicationConfiguration.RPC, serv, oracleSrv, log, errChan)
	serv.AddService(&rpcServer)

	go serv.Start(errChan)
	if !cfg.ApplicationConfiguration.RPC.StartWhenSynchronized {
		rpcServer.Start()
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
			configureAddresses(&cfgnew.ApplicationConfiguration)
			switch sig {
			case sighup:
				if newLogLevel != zapcore.InvalidLevel {
					logLevel.SetLevel(newLogLevel)
					log.Warn("using new logging level", zap.Stringer("level", newLogLevel))
				}
				serv.DelService(&rpcServer)
				rpcServer.Shutdown()
				rpcServer = rpcsrv.New(chain, cfgnew.ApplicationConfiguration.RPC, serv, oracleSrv, log, errChan)
				serv.AddService(&rpcServer)
				if !cfgnew.ApplicationConfiguration.RPC.StartWhenSynchronized || serv.IsInSync() {
					rpcServer.Start()
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
				dbftSrv, err = mkConsensus(cfgnew.ApplicationConfiguration.Consensus, serverConfig.TimePerBlock, chain, serv, log)
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
		return cli.NewExitError(shutdownErr, 1)
	}

	return nil
}

// configureAddresses sets up addresses for RPC, Prometheus and Pprof depending from the provided config.
// In case RPC or Prometheus or Pprof Address provided each of them will use it.
// In case global Address (of the node) provided and RPC/Prometheus/Pprof don't have configured addresses they will
// use global one. So Node and RPC and Prometheus and Pprof will run on one address.
func configureAddresses(cfg *config.ApplicationConfiguration) {
	if cfg.Address != nil && *cfg.Address != "" { //nolint:staticcheck // SA1019: cfg.Address is deprecated
		if cfg.RPC.Address == nil || *cfg.RPC.Address == "" { //nolint:staticcheck // SA1019: cfg.RPC.Address is deprecated
			cfg.RPC.Address = cfg.Address //nolint:staticcheck // SA1019: cfg.RPC.Address is deprecated
		}
		if cfg.Prometheus.Address == nil || *cfg.Prometheus.Address == "" { //nolint:staticcheck // SA1019: cfg.Prometheus.Address is deprecated
			cfg.Prometheus.Address = cfg.Address //nolint:staticcheck // SA1019: cfg.Prometheus.Address is deprecated
		}
		if cfg.Pprof.Address == nil || *cfg.Pprof.Address == "" { //nolint:staticcheck // SA1019: cfg.Pprof.Address is deprecated
			cfg.Pprof.Address = cfg.Address //nolint:staticcheck // SA1019: cfg.Pprof.Address is deprecated
		}
	}
}

// initBlockChain initializes BlockChain with preselected DB.
func initBlockChain(cfg config.Config, log *zap.Logger) (*core.Blockchain, storage.Store, error) {
	store, err := storage.NewStore(cfg.ApplicationConfiguration.DBConfiguration)
	if err != nil {
		return nil, nil, cli.NewExitError(fmt.Errorf("could not initialize storage: %w", err), 1)
	}

	chain, err := core.NewBlockchain(store, cfg.Blockchain(), log)
	if err != nil {
		errText := "could not initialize blockchain: %w"
		errArgs := []interface{}{err}
		closeErr := store.Close()
		if closeErr != nil {
			errText += "; failed to close the DB: %w"
			errArgs = append(errArgs, closeErr)
		}

		return nil, nil, cli.NewExitError(fmt.Errorf(errText, errArgs...), 1)
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
