/*
Package options contains a set of common CLI options and helper functions to use them.
*/
package options

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// DefaultTimeout is the default timeout used for RPC requests.
const DefaultTimeout = 10 * time.Second

// RPCEndpointFlag is a long flag name for an RPC endpoint. It can be used to
// check for flag presence in the context.
const RPCEndpointFlag = "rpc-endpoint"

// Network is a set of flags for choosing the network to operate on
// (privnet/mainnet/testnet).
var Network = []cli.Flag{
	cli.BoolFlag{Name: "privnet, p", Usage: "use private network configuration"},
	cli.BoolFlag{Name: "mainnet, m", Usage: "use mainnet network configuration"},
	cli.BoolFlag{Name: "testnet, t", Usage: "use testnet network configuration"},
	cli.BoolFlag{Name: "unittest", Hidden: true},
}

// RPC is a set of flags used for RPC connections (endpoint and timeout).
var RPC = []cli.Flag{
	cli.StringFlag{
		Name:  RPCEndpointFlag + ", r",
		Usage: "RPC node address",
	},
	cli.DurationFlag{
		Name:  "timeout, s",
		Value: DefaultTimeout,
		Usage: "Timeout for the operation",
	},
}

// Historic is a flag for commands that can perform historic invocations.
var Historic = cli.StringFlag{
	Name:  "historic",
	Usage: "Use historic state (height, block hash or state root hash)",
}

// Config is a flag for commands that use node configuration.
var Config = cli.StringFlag{
	Name:  "config-path",
	Usage: "path to directory with configuration files",
}

// Debug is a flag for commands that allow node in debug mode usage.
var Debug = cli.BoolFlag{
	Name:  "debug, d",
	Usage: "enable debug logging (LOTS of output, overrides configuration)",
}

var errNoEndpoint = errors.New("no RPC endpoint specified, use option '--" + RPCEndpointFlag + "' or '-r'")
var errInvalidHistoric = errors.New("invalid 'historic' parameter, neither a block number, nor a block/state hash")

// GetNetwork examines Context's flags and returns the appropriate network. It
// defaults to PrivNet if no flags are given.
func GetNetwork(ctx *cli.Context) netmode.Magic {
	var net = netmode.PrivNet
	if ctx.Bool("testnet") {
		net = netmode.TestNet
	}
	if ctx.Bool("mainnet") {
		net = netmode.MainNet
	}
	if ctx.Bool("unittest") {
		net = netmode.UnitTestNet
	}
	return net
}

// GetTimeoutContext returns a context.Context with the default or a user-set timeout.
func GetTimeoutContext(ctx *cli.Context) (context.Context, func()) {
	dur := ctx.Duration("timeout")
	if dur == 0 {
		dur = DefaultTimeout
	}
	return context.WithTimeout(context.Background(), dur)
}

// GetRPCClient returns an RPC client instance for the given Context.
func GetRPCClient(gctx context.Context, ctx *cli.Context) (*rpcclient.Client, cli.ExitCoder) {
	endpoint := ctx.String(RPCEndpointFlag)
	if len(endpoint) == 0 {
		return nil, cli.NewExitError(errNoEndpoint, 1)
	}
	c, err := rpcclient.New(gctx, endpoint, rpcclient.Options{})
	if err != nil {
		return nil, cli.NewExitError(err, 1)
	}
	err = c.Init()
	if err != nil {
		return nil, cli.NewExitError(err, 1)
	}
	return c, nil
}

// GetInvoker returns an invoker using the given RPC client, context and signers.
// It parses "--historic" parameter to adjust it.
func GetInvoker(c *rpcclient.Client, ctx *cli.Context, signers []transaction.Signer) (*invoker.Invoker, cli.ExitCoder) {
	historic := ctx.String("historic")
	if historic == "" {
		return invoker.New(c, signers), nil
	}
	if index, err := strconv.ParseUint(historic, 10, 32); err == nil {
		return invoker.NewHistoricAtHeight(uint32(index), c, signers), nil
	}
	if u256, err := util.Uint256DecodeStringLE(historic); err == nil {
		// Might as well be a block hash, but it makes no practical difference.
		return invoker.NewHistoricWithState(u256, c, signers), nil
	}
	return nil, cli.NewExitError(errInvalidHistoric, 1)
}

// GetRPCWithInvoker combines GetRPCClient with GetInvoker for cases where it's
// appropriate to do so.
func GetRPCWithInvoker(gctx context.Context, ctx *cli.Context, signers []transaction.Signer) (*rpcclient.Client, *invoker.Invoker, cli.ExitCoder) {
	c, err := GetRPCClient(gctx, ctx)
	if err != nil {
		return nil, nil, err
	}
	inv, err := GetInvoker(c, ctx, signers)
	if err != nil {
		c.Close()
		return nil, nil, err
	}
	return c, inv, err
}

// GetConfigFromContext looks at the path and the mode flags in the given config and
// returns an appropriate config.
func GetConfigFromContext(ctx *cli.Context) (config.Config, error) {
	configPath := "./config"
	if argCp := ctx.String("config-path"); argCp != "" {
		configPath = argCp
	}
	return config.Load(configPath, GetNetwork(ctx))
}

var (
	// _winfileSinkRegistered denotes whether zap has registered
	// user-supplied factory for all sinks with `winfile`-prefixed scheme.
	_winfileSinkRegistered bool
	_winfileSinkCloser     func() error
)

// HandleLoggingParams reads logging parameters.
// If a user selected debug level -- function enables it.
// If logPath is configured -- function creates a dir and a file for logging.
// If logPath is configured on Windows -- function returns closer to be
// able to close sink for the opened log output file.
func HandleLoggingParams(debug bool, cfg config.ApplicationConfiguration) (*zap.Logger, *zap.AtomicLevel, func() error, error) {
	var (
		level = zapcore.InfoLevel
		err   error
	)
	if len(cfg.LogLevel) > 0 {
		level, err = zapcore.ParseLevel(cfg.LogLevel)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("log setting: %w", err)
		}
	}
	if debug {
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
			return nil, nil, nil, err
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
					return nil, nil, nil, fmt.Errorf("failed to register windows-specific sinc: %w", err)
				}
				_winfileSinkRegistered = true
			}
			logPath = "winfile:///" + logPath
		}

		cc.OutputPaths = []string{logPath}
	}

	log, err := cc.Build()
	return log, &cc.Level, _winfileSinkCloser, err
}
