package main

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/rpc/server"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/term"
)

const (
	validatorWIF  = "KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY"
	validatorAddr = "NfgHwwTi3wHAS8aFAN243C5vGbkYDpqLHP"
	multisigAddr  = "NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq"

	validatorWallet = "testdata/wallet1_solo.json"
)

var (
	validatorHash, _ = address.StringToUint160(validatorAddr)
	validatorPriv, _ = keys.NewPrivateKeyFromWIF(validatorWIF)
)

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
	// In contains command input.
	In *bytes.Buffer
}

func newTestChain(t *testing.T, f func(*config.Config), run bool) (*core.Blockchain, *server.Server, *network.Server) {
	configPath := "../config/protocol.unit_testnet.single.yml"
	cfg, err := config.LoadFile(configPath)
	require.NoError(t, err, "could not load config")
	if f != nil {
		f(&cfg)
	}

	memoryStore := storage.NewMemoryStore()
	logger := zaptest.NewLogger(t)
	chain, err := core.NewBlockchain(memoryStore, cfg.ProtocolConfiguration, logger)
	require.NoError(t, err, "could not create chain")

	if run {
		go chain.Run()
	}

	serverConfig := network.NewServerConfig(cfg)
	netSrv, err := network.NewServer(serverConfig, chain, zap.NewNop())
	require.NoError(t, err)
	go netSrv.Start(make(chan error, 1))
	rpcServer := server.New(chain, cfg.ApplicationConfiguration.RPC, netSrv, nil, logger)
	errCh := make(chan error, 2)
	rpcServer.Start(errCh)

	return chain, &rpcServer, netSrv
}

func newExecutor(t *testing.T, needChain bool) *executor {
	return newExecutorWithConfig(t, needChain, true, nil)
}

func newExecutorSuspended(t *testing.T) *executor {
	return newExecutorWithConfig(t, true, false, nil)
}

func newExecutorWithConfig(t *testing.T, needChain, runChain bool, f func(*config.Config)) *executor {
	e := &executor{
		CLI: newApp(),
		Out: bytes.NewBuffer(nil),
		Err: bytes.NewBuffer(nil),
		In:  bytes.NewBuffer(nil),
	}
	e.CLI.Writer = e.Out
	e.CLI.ErrWriter = e.Err
	if needChain {
		e.Chain, e.RPC, e.NetSrv = newTestChain(t, f, runChain)
	}
	t.Cleanup(func() {
		e.Close(t)
	})
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

// GetTransaction returns tx with hash h after it has persisted.
// If it is in mempool, we can just wait for the next block, otherwise
// it must be already in chain. 1 second is time per block in a unittest chain.
func (e *executor) GetTransaction(t *testing.T, h util.Uint256) (*transaction.Transaction, uint32) {
	var tx *transaction.Transaction
	var height uint32
	require.Eventually(t, func() bool {
		var err error
		tx, height, err = e.Chain.GetTransaction(h)
		return err == nil && height != math.MaxUint32
	}, time.Second*2, time.Millisecond*100, "too long time waiting for block")
	return tx, height
}

func (e *executor) getNextLine(t *testing.T) string {
	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	return strings.TrimSuffix(line, "\n")
}

func (e *executor) checkNextLine(t *testing.T, expected string) {
	line := e.getNextLine(t)
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
	input.Terminal = term.NewTerminal(input.ReadWriter{
		Reader: e.In,
		Writer: ioutil.Discard,
	}, "")
	err := e.CLI.Run(args)
	input.Terminal = nil
	e.In.Reset()
	return err
}

func (e *executor) checkTxPersisted(t *testing.T, prefix ...string) (*transaction.Transaction, uint32) {
	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)

	line = strings.TrimSpace(line)
	if len(prefix) > 0 {
		line = strings.TrimPrefix(line, prefix[0])
	}
	h, err := util.Uint256DecodeStringLE(line)
	require.NoError(t, err, "can't decode tx hash: %s", line)

	tx, height := e.GetTransaction(t, h)
	aer, err := e.Chain.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 1, len(aer))
	require.Equal(t, vm.HaltState, aer[0].VMState)
	return tx, height
}

func generateKeys(t *testing.T, n int) ([]*keys.PrivateKey, keys.PublicKeys) {
	privs := make([]*keys.PrivateKey, n)
	pubs := make(keys.PublicKeys, n)
	for i := range privs {
		var err error
		privs[i], err = keys.NewPrivateKey()
		require.NoError(t, err)
		pubs[i] = privs[i].PublicKey()
	}
	return privs, pubs
}
