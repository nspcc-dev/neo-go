package server

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"runtime"
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

var (
	// _winfileSinkRegistered denotes whether zap has registered
	// user-supplied factory for all sinks with `winfile`-prefixed scheme.
	_winfileSinkRegistered bool
	_winfileSinkCloser     func() error
)

// NewCommands returns 'node' command.
func NewCommands() []cli.Command {
	var cfgFlags = []cli.Flag{
		cli.StringFlag{Name: "config-path", Usage: "path to directory with configuration files"},
	}
	cfgFlags = append(cfgFlags, options.Network...)
	var cfgWithCountFlags = make([]cli.Flag, len(cfgFlags))
	copy(cfgWithCountFlags, cfgFlags)
	cfgFlags = append(cfgFlags, cli.BoolFlag{Name: "debug, d", Usage: "enable debug logging (LOTS of output)"})

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
	return []cli.Command{
		{
			Name:      "node",
			Usage:     "start a NEO node",
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
			},
		},
	}
}

func newGraceContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go func() {
		<-stop
		cancel()
	}()
	return ctx
}

// getConfigFromContext looks at the path and the mode flags in the given config and
// returns an appropriate config.
func getConfigFromContext(ctx *cli.Context) (config.Config, error) {
	configPath := "./config"
	if argCp := ctx.String("config-path"); argCp != "" {
		configPath = argCp
	}
	return config.Load(configPath, options.GetNetwork(ctx))
}

// handleLoggingParams reads logging parameters.
// If a user selected debug level -- function enables it.
// If logPath is configured -- function creates a dir and a file for logging.
// If logPath is configured on Windows -- function returns closer to be
// able to close sink for the opened log output file.
func handleLoggingParams(ctx *cli.Context, cfg config.ApplicationConfiguration) (*zap.Logger, func() error, error) {
	level := zapcore.InfoLevel
	if ctx.Bool("debug") {
		level = zapcore.DebugLevel
	}

	cc := zap.NewProductionConfig()
	cc.DisableCaller = true
	cc.DisableStacktrace = true
	cc.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	cc.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	cc.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cc.Encoding = "console"
	cc.Level = zap.NewAtomicLevelAt(level)
	cc.Sampling = nil

	if logPath := cfg.LogPath; logPath != "" {
		if err := io.MakeDirForFile(logPath, "logger"); err != nil {
			return nil, nil, err
		}

		if runtime.GOOS == "windows" {
			if !_winfileSinkRegistered {
				// See https://github.com/uber-go/zap/issues/621.
				err := zap.RegisterSink("winfile", func(u *url.URL) (zap.Sink, error) {
					if u.User != nil {
						return nil, fmt.Errorf("user and password not allowed with file URLs: got %v", u)
					}
					if u.Fragment != "" {
						return nil, fmt.Errorf("fragments not allowed with file URLs: got %v", u)
					}
					if u.RawQuery != "" {
						return nil, fmt.Errorf("query parameters not allowed with file URLs: got %v", u)
					}
					// Error messages are better if we check hostname and port separately.
					if u.Port() != "" {
						return nil, fmt.Errorf("ports not allowed with file URLs: got %v", u)
					}
					if hn := u.Hostname(); hn != "" && hn != "localhost" {
						return nil, fmt.Errorf("file URLs must leave host empty or use localhost: got %v", u)
					}
					switch u.Path {
					case "stdout":
						return os.Stdout, nil
					case "stderr":
						return os.Stderr, nil
					}
					f, err := os.OpenFile(u.Path[1:], // Remove leading slash left after url.Parse.
						os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
					_winfileSinkCloser = func() error {
						_winfileSinkCloser = nil
						return f.Close()
					}
					return f, err
				})
				if err != nil {
					return nil, nil, fmt.Errorf("failed to register windows-specific sinc: %w", err)
				}
				_winfileSinkRegistered = true
			}
			logPath = "winfile:///" + logPath
		}

		cc.OutputPaths = []string{logPath}
	}

	log, err := cc.Build()
	return log, _winfileSinkCloser, err
}

func initBCWithMetrics(cfg config.Config, log *zap.Logger) (*core.Blockchain, *metrics.Service, *metrics.Service, error) {
	chain, err := initBlockChain(cfg, log)
	if err != nil {
		return nil, nil, nil, cli.NewExitError(err, 1)
	}
	configureAddresses(&cfg.ApplicationConfiguration)
	prometheus := metrics.NewPrometheusService(cfg.ApplicationConfiguration.Prometheus, log)
	pprof := metrics.NewPprofService(cfg.ApplicationConfiguration.Pprof, log)

	go chain.Run()
	go prometheus.Start()
	go pprof.Start()

	return chain, prometheus, pprof, nil
}

func dumpDB(ctx *cli.Context) error {
	if err := cmdargs.EnsureNone(ctx); err != nil {
		return err
	}
	cfg, err := getConfigFromContext(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	log, logCloser, err := handleLoggingParams(ctx, cfg.ApplicationConfiguration)
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
	cfg, err := getConfigFromContext(ctx)
	if err != nil {
		return err
	}
	log, logCloser, err := handleLoggingParams(ctx, cfg.ApplicationConfiguration)
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
		cfg.ProtocolConfiguration.SaveStorageBatch = true
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

func mkConsensus(config config.Wallet, tpb time.Duration, chain *core.Blockchain, serv *network.Server, log *zap.Logger) (consensus.Service, error) {
	if len(config.Path) == 0 {
		return nil, nil
	}
	srv, err := consensus.NewService(consensus.Config{
		Logger:                log,
		Broadcast:             serv.BroadcastExtensible,
		Chain:                 chain,
		ProtocolConfiguration: chain.GetConfig(),
		RequestTx:             serv.RequestTx,
		Wallet:                &config,
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

	cfg, err := getConfigFromContext(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	log, logCloser, err := handleLoggingParams(ctx, cfg.ApplicationConfiguration)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if logCloser != nil {
		defer func() { _ = logCloser() }()
	}

	grace, cancel := context.WithCancel(newGraceContext())
	defer cancel()

	serverConfig := network.NewServerConfig(cfg)

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
	dbftSrv, err := mkConsensus(cfg.ApplicationConfiguration.UnlockWallet, serverConfig.TimePerBlock, chain, serv, log)
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
			log.Info("signal received", zap.Stringer("name", sig))
			cfgnew, err := getConfigFromContext(ctx)
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
			configureAddresses(&cfgnew.ApplicationConfiguration)
			switch sig {
			case sighup:
				serv.DelService(&rpcServer)
				rpcServer.Shutdown()
				rpcServer = rpcsrv.New(chain, cfgnew.ApplicationConfiguration.RPC, serv, oracleSrv, log, errChan)
				serv.AddService(&rpcServer)
				if !cfgnew.ApplicationConfiguration.RPC.StartWhenSynchronized || serv.IsInSync() {
					rpcServer.Start()
				}
				pprof.ShutDown()
				pprof = metrics.NewPprofService(cfgnew.ApplicationConfiguration.Pprof, log)
				go pprof.Start()
				prometheus.ShutDown()
				prometheus = metrics.NewPrometheusService(cfgnew.ApplicationConfiguration.Prometheus, log)
				go prometheus.Start()
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
				dbftSrv, err = mkConsensus(cfgnew.ApplicationConfiguration.UnlockWallet, serverConfig.TimePerBlock, chain, serv, log)
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
	if cfg.Address != "" {
		if cfg.RPC.Address == "" {
			cfg.RPC.Address = cfg.Address
		}
		if cfg.Prometheus.Address == "" {
			cfg.Prometheus.Address = cfg.Address
		}
		if cfg.Pprof.Address == "" {
			cfg.Pprof.Address = cfg.Address
		}
	}
}

// initBlockChain initializes BlockChain with preselected DB.
func initBlockChain(cfg config.Config, log *zap.Logger) (*core.Blockchain, error) {
	store, err := storage.NewStore(cfg.ApplicationConfiguration.DBConfiguration)
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("could not initialize storage: %w", err), 1)
	}

	chain, err := core.NewBlockchain(store, cfg.ProtocolConfiguration, log)
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("could not initialize blockchain: %w", err), 1)
	}
	return chain, nil
}

// Logo returns Neo-Go logo.
func Logo() string {
	return `
    _   ____________        __________
   / | / / ____/ __ \      / ____/ __ \
  /  |/ / __/ / / / /_____/ / __/ / / /
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /
/_/ |_/_____/\____/      \____/\____/
`
}
