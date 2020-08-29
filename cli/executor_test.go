package main

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/rpc/server"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

const (
	validatorAddr = "NVNvVRW5Q5naSx2k2iZm7xRgtRNGuZppAK"

	validatorWallet = "testdata/wallet1_solo.json"
)

var validatorHash, _ = address.StringToUint160(validatorAddr)

// executor represents context for a test instance.
// It can be safely used in multiple tests, but not in parallel.
type executor struct {
	// CLI is a cli application to test.
	CLI *cli.App
	// Chain is a blockchain instance (can be empty).
	Chain *core.Blockchain
	// RPC is an RPC server to query (can be empty).
	RPC *server.Server
	// NetSrv is a network server (can be empty).
	NetSrv *network.Server
	// Out contains command output.
	Out *bytes.Buffer
	// Err contains command errors.
	Err *bytes.Buffer
}

func newTestChain(t *testing.T) (*core.Blockchain, *server.Server, *network.Server) {
	configPath := "../config/protocol.unit_testnet.single.yml"
	cfg, err := config.LoadFile(configPath)
	require.NoError(t, err, "could not load config")

	memoryStore := storage.NewMemoryStore()
	logger := zaptest.NewLogger(t)
	chain, err := core.NewBlockchain(memoryStore, cfg.ProtocolConfiguration, logger)
	require.NoError(t, err, "could not create chain")

	go chain.Run()

	serverConfig := network.NewServerConfig(cfg)
	netSrv, err := network.NewServer(serverConfig, chain, zap.NewNop())
	require.NoError(t, err)
	go netSrv.Start(make(chan error, 1))
	rpcServer := server.New(chain, cfg.ApplicationConfiguration.RPC, netSrv, logger)
	errCh := make(chan error, 2)
	rpcServer.Start(errCh)

	return chain, &rpcServer, netSrv
}

func newExecutor(t *testing.T, needChain bool) *executor {
	e := &executor{
		CLI: newApp(),
		Out: bytes.NewBuffer(nil),
		Err: bytes.NewBuffer(nil),
	}
	e.CLI.Writer = e.Out
	e.CLI.ErrWriter = e.Err
	if needChain {
		e.Chain, e.RPC, e.NetSrv = newTestChain(t)
	}
	return e
}

func (e *executor) Close(t *testing.T) {
	input.Terminal = nil
	if e.RPC != nil {
		require.NoError(t, e.RPC.Shutdown())
	}
	if e.NetSrv != nil {
		e.NetSrv.Shutdown()
	}
	if e.Chain != nil {
		e.Chain.Close()
	}
}

func (e *executor) checkNextLine(t *testing.T, expected string) {
	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	e.checkLine(t, line, expected)
}

func (e *executor) checkLine(t *testing.T, line, expected string) {
	require.Regexp(t, expected, line)
}

func (e *executor) checkEOF(t *testing.T) {
	_, err := e.Out.ReadString('\n')
	require.True(t, errors.Is(err, io.EOF))
}

func setExitFunc() <-chan int {
	ch := make(chan int, 1)
	cli.OsExiter = func(code int) {
		ch <- code
	}
	return ch
}

func checkExit(t *testing.T, ch <-chan int, code int) {
	select {
	case c := <-ch:
		require.Equal(t, code, c)
	default:
		if code != 0 {
			require.Fail(t, "no exit was called")
		}
	}
}

// RunWithError runs command and checks that is exits with error.
func (e *executor) RunWithError(t *testing.T, args ...string) {
	ch := setExitFunc()
	require.Error(t, e.run(args...))
	checkExit(t, ch, 1)
}

// Run runs command and checks that there were no errors.
func (e *executor) Run(t *testing.T, args ...string) {
	ch := setExitFunc()
	require.NoError(t, e.run(args...))
	checkExit(t, ch, 0)
}
func (e *executor) run(args ...string) error {
	e.Out.Reset()
	e.Err.Reset()
	return e.CLI.Run(args)
}
