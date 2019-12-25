package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/encoding/address"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/network/metrics"
	"github.com/CityOfZion/neo-go/pkg/rpc"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// NewCommands returns 'node' command.
func NewCommands() []cli.Command {
	var cfgFlags = []cli.Flag{
		cli.StringFlag{Name: "config-path"},
		cli.BoolFlag{Name: "privnet, p"},
		cli.BoolFlag{Name: "mainnet, m"},
		cli.BoolFlag{Name: "testnet, t"},
		cli.BoolFlag{Name: "debug, d"},
	}
	var cfgWithCountFlags = make([]cli.Flag, len(cfgFlags))
	copy(cfgWithCountFlags, cfgFlags)
	cfgWithCountFlags = append(cfgWithCountFlags,
		cli.UintFlag{
			Name:  "count, c",
			Usage: "number of blocks to be processed (default or 0: all chain)",
		},
		cli.UintFlag{
			Name:  "skip, s",
			Usage: "number of blocks to skip (default: 0)",
		},
	)
	var cfgCountOutFlags = make([]cli.Flag, len(cfgWithCountFlags))
	copy(cfgCountOutFlags, cfgWithCountFlags)
	cfgCountOutFlags = append(cfgCountOutFlags, cli.StringFlag{
		Name:  "out, o",
		Usage: "Output file (stdout if not given)",
	})
	var cfgCountInFlags = make([]cli.Flag, len(cfgWithCountFlags))
	copy(cfgCountInFlags, cfgWithCountFlags)
	cfgCountInFlags = append(cfgCountInFlags, cli.StringFlag{
		Name:  "in, i",
		Usage: "Input file (stdin if not given)",
	})
	return []cli.Command{
		{
			Name:   "node",
			Usage:  "start a NEO node",
			Action: startServer,
			Flags:  cfgFlags,
		},
		{
			Name:  "db",
			Usage: "database manipulations",
			Subcommands: []cli.Command{
				{
					Name:   "dump",
					Usage:  "dump blocks (starting with block #1) to the file",
					Action: dumpDB,
					Flags:  cfgCountOutFlags,
				},
				{
					Name:   "restore",
					Usage:  "restore blocks from the file",
					Action: restoreDB,
					Flags:  cfgCountInFlags,
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

// getConfigFromContext looks at path and mode flags in the given config and
// returns appropriate config.
func getConfigFromContext(ctx *cli.Context) (config.Config, error) {
	var net = config.ModePrivNet
	if ctx.Bool("testnet") {
		net = config.ModeTestNet
	}
	if ctx.Bool("mainnet") {
		net = config.ModeMainNet
	}
	configPath := "./config"
	if argCp := ctx.String("config-path"); argCp != "" {
		configPath = argCp
	}
	return config.Load(configPath, net)
}

// handleLoggingParams reads logging parameters.
// If user selected debug level -- function enables it.
// If logPath is configured -- function creates dir and file for logging.
func handleLoggingParams(ctx *cli.Context, cfg config.ApplicationConfiguration) error {
	if ctx.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	if logPath := cfg.LogPath; logPath != "" {
		if err := io.MakeDirForFile(logPath, "logger"); err != nil {
			return err
		}
		f, err := os.Create(logPath)
		if err != nil {
			return err
		}
		log.SetOutput(f)
	}
	return nil
}

func getCountAndSkipFromContext(ctx *cli.Context) (uint32, uint32) {
	count := uint32(ctx.Uint("count"))
	skip := uint32(ctx.Uint("skip"))
	return count, skip
}

func initBCWithMetrics(cfg config.Config) (*core.Blockchain, *metrics.Service, *metrics.Service, error) {
	chain, err := initBlockChain(cfg)
	if err != nil {
		return nil, nil, nil, cli.NewExitError(err, 1)
	}
	configureAddresses(cfg.ApplicationConfiguration)
	prometheus := metrics.NewPrometheusService(cfg.ApplicationConfiguration.Prometheus)
	pprof := metrics.NewPprofService(cfg.ApplicationConfiguration.Pprof)

	go chain.Run()
	go prometheus.Start()
	go pprof.Start()

	return chain, prometheus, pprof, nil
}

func dumpDB(ctx *cli.Context) error {
	cfg, err := getConfigFromContext(ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if err := handleLoggingParams(ctx, cfg.ApplicationConfiguration); err != nil {
		return cli.NewExitError(err, 1)
	}
	count, skip := getCountAndSkipFromContext(ctx)

	var outStream = os.Stdout
	if out := ctx.String("out"); out != "" {
		outStream, err = os.Create(out)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	defer outStream.Close()
	writer := io.NewBinWriterFromIO(outStream)

	chain, prometheus, pprof, err := initBCWithMetrics(cfg)
	if err != nil {
		return err
	}

	chainHeight := chain.BlockHeight()
	if skip+count > chainHeight {
		return cli.NewExitError(fmt.Errorf("chain is not that high (%d) to dump %d blocks starting from %d", chainHeight, count, skip), 1)
	}
	if count == 0 {
		count = chainHeight - skip
	}
	writer.WriteU32LE(count)
	for i := skip + 1; i <= skip+count; i++ {
		bh := chain.GetHeaderHash(int(i))
		b, err := chain.GetBlock(bh)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to get block %d: %s", i, err), 1)
		}
		buf := io.NewBufBinWriter()
		b.EncodeBinary(buf.BinWriter)
		bytes := buf.Bytes()
		writer.WriteVarBytes(bytes)
		if writer.Err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	pprof.ShutDown()
	prometheus.ShutDown()
	chain.Close()
	return nil
}

func restoreDB(ctx *cli.Context) error {
	cfg, err := getConfigFromContext(ctx)
	if err != nil {
		return err
	}
	if err := handleLoggingParams(ctx, cfg.ApplicationConfiguration); err != nil {
		return cli.NewExitError(err, 1)
	}
	count, skip := getCountAndSkipFromContext(ctx)

	var inStream = os.Stdin
	if in := ctx.String("in"); in != "" {
		inStream, err = os.Open(in)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	defer inStream.Close()
	reader := io.NewBinReaderFromIO(inStream)

	chain, prometheus, pprof, err := initBCWithMetrics(cfg)
	if err != nil {
		return err
	}

	var allBlocks = reader.ReadU32LE()
	if reader.Err != nil {
		return cli.NewExitError(err, 1)
	}
	if skip+count > allBlocks {
		return cli.NewExitError(fmt.Errorf("input file has only %d blocks, can't read %d starting from %d", allBlocks, count, skip), 1)
	}
	if count == 0 {
		count = allBlocks
	}
	i := uint32(0)
	for ; i < skip; i++ {
		_, err := readBlock(reader)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	for ; i < skip+count; i++ {
		bytes, err := readBlock(reader)
		block := &core.Block{}
		newReader := io.NewBinReaderFromBuf(bytes)
		block.DecodeBinary(newReader)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		err = chain.AddBlock(block)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("failed to add block %d: %s", i, err), 1)
		}
	}
	pprof.ShutDown()
	prometheus.ShutDown()
	chain.Close()
	return nil
}

// readBlock performs reading of block size and then bytes with the length equal to that size.
func readBlock(reader *io.BinReader) ([]byte, error) {
	var size = reader.ReadU32LE()
	bytes := make([]byte, size)
	reader.ReadBytes(bytes)
	if reader.Err != nil {
		return nil, reader.Err
	}
	return bytes, nil
}

func startServer(ctx *cli.Context) error {
	cfg, err := getConfigFromContext(ctx)
	if err != nil {
		return err
	}
	if err := handleLoggingParams(ctx, cfg.ApplicationConfiguration); err != nil {
		return err
	}

	grace, cancel := context.WithCancel(newGraceContext())
	defer cancel()

	serverConfig := network.NewServerConfig(cfg)

	chain, prometheus, pprof, err := initBCWithMetrics(cfg)
	if err != nil {
		return err
	}

	server := network.NewServer(serverConfig, chain)
	rpcServer := rpc.NewServer(chain, cfg.ApplicationConfiguration.RPC, server)
	errChan := make(chan error)

	go server.Start(errChan)
	go rpcServer.Start(errChan)

	fmt.Println(logo())
	fmt.Println(server.UserAgent)
	fmt.Println()

	var shutdownErr error
Main:
	for {
		select {
		case err := <-errChan:
			shutdownErr = errors.Wrap(err, "Error encountered by server")
			cancel()

		case <-grace.Done():
			server.Shutdown()
			if serverErr := rpcServer.Shutdown(); serverErr != nil {
				shutdownErr = errors.Wrap(serverErr, "Error encountered whilst shutting down server")
			}
			prometheus.ShutDown()
			pprof.ShutDown()
			chain.Close()
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
func configureAddresses(cfg config.ApplicationConfiguration) {
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
func initBlockChain(cfg config.Config) (*core.Blockchain, error) {
	store, err := storage.NewStore(cfg.ApplicationConfiguration.DBConfiguration)
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("could not initialize storage: %s", err), 1)
	}

	chain, err := core.NewBlockchain(store, cfg.ProtocolConfiguration)
	if err != nil {
		return nil, cli.NewExitError(fmt.Errorf("could not initialize blockchain: %s", err), 1)
	}
	if cfg.ProtocolConfiguration.AddressVersion != 0 {
		address.Prefix = cfg.ProtocolConfiguration.AddressVersion
	}
	return chain, nil
}

func logo() string {
	return `
    _   ____________        __________
   / | / / ____/ __ \      / ____/ __ \
  /  |/ / __/ / / / /_____/ / __/ / / /
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /
/_/ |_/_____/\____/      \____/\____/
`
}
