/*
Package testcli contains auxiliary code to test CLI commands.

All testdata assets for it are contained in the cli directory and paths here
use `../` prefix to reference them because the package itself is used from
cli/* subpackages.
*/
package testcli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/cli/app"
	"github.com/nspcc-dev/neo-go/cli/input"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/consensus"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/term"
)

const (
	ValidatorWIF  = "KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY"
	ValidatorAddr = "NfgHwwTi3wHAS8aFAN243C5vGbkYDpqLHP"
	MultisigAddr  = "NVTiAjNgagDkTr5HTzDmQP9kPwPHN5BgVq"

	TestWalletPath    = "../testdata/testwallet.json"
	TestWalletAccount = "Nfyz4KcsgYepRJw1W5C2uKCi6QWKf7v6gG"

	ValidatorWallet = "../testdata/wallet1_solo.json"
	ValidatorPass   = "one"
)

var (
	ValidatorHash, _ = address.StringToUint160(ValidatorAddr)
	ValidatorPriv, _ = keys.NewPrivateKeyFromWIF(ValidatorWIF)
)

// Executor represents context for a test instance.
// It can be safely used in multiple tests, but not in parallel.
type Executor struct {
	// CLI is a cli application to test.
	CLI *cli.App
	// Chain is a blockchain instance (can be empty).
	Chain *core.Blockchain
	// RPC is an RPC server to query (can be empty).
	RPC *rpcsrv.Server
	// NetSrv is a network server (can be empty).
	NetSrv *network.Server
	// Out contains command output.
	Out *ConcurrentBuffer
	// Err contains command errors.
	Err *bytes.Buffer
	// In contains command input.
	In *bytes.Buffer
}

// ConcurrentBuffer is a wrapper over Buffer with mutex.
type ConcurrentBuffer struct {
	lock sync.RWMutex
	buf  *bytes.Buffer
}

// NewConcurrentBuffer returns new ConcurrentBuffer with underlying buffer initialized.
func NewConcurrentBuffer() *ConcurrentBuffer {
	return &ConcurrentBuffer{
		buf: bytes.NewBuffer(nil),
	}
}

// Write is a concurrent wrapper over the corresponding method of bytes.Buffer.
func (w *ConcurrentBuffer) Write(p []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.buf.Write(p)
}

// ReadString is a concurrent wrapper over the corresponding method of bytes.Buffer.
func (w *ConcurrentBuffer) ReadString(delim byte) (string, error) {
	w.lock.RLock()
	defer w.lock.RUnlock()

	return w.buf.ReadString(delim)
}

// Bytes is a concurrent wrapper over the corresponding method of bytes.Buffer.
func (w *ConcurrentBuffer) Bytes() []byte {
	w.lock.RLock()
	defer w.lock.RUnlock()

	return w.buf.Bytes()
}

// String is a concurrent wrapper over the corresponding method of bytes.Buffer.
func (w *ConcurrentBuffer) String() string {
	w.lock.RLock()
	defer w.lock.RUnlock()

	return w.buf.String()
}

// Reset is a concurrent wrapper over the corresponding method of bytes.Buffer.
func (w *ConcurrentBuffer) Reset() {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.buf.Reset()
}

func NewTestChain(t *testing.T, f func(*config.Config), run bool) (*core.Blockchain, *rpcsrv.Server, *network.Server) {
	configPath := "../../config/protocol.unit_testnet.single.yml"
	cfg, err := config.LoadFile(configPath)
	require.NoError(t, err, "could not load config")
	if f != nil {
		f(&cfg)
	}

	memoryStore := storage.NewMemoryStore()
	logger := zaptest.NewLogger(t)
	chain, err := core.NewBlockchain(memoryStore, cfg.Blockchain(), logger)
	require.NoError(t, err, "could not create chain")

	if run {
		go chain.Run()
	}

	serverConfig, err := network.NewServerConfig(cfg)
	require.NoError(t, err)
	serverConfig.UserAgent = fmt.Sprintf(config.UserAgentFormat, "0.98.3-test")
	netSrv, err := network.NewServer(serverConfig, chain, chain.GetStateSyncModule(), zap.NewNop())
	require.NoError(t, err)
	cons, err := consensus.NewService(consensus.Config{
		Logger:                zap.NewNop(),
		Broadcast:             netSrv.BroadcastExtensible,
		Chain:                 chain,
		BlockQueue:            netSrv.GetBlockQueue(),
		ProtocolConfiguration: cfg.ProtocolConfiguration,
		RequestTx:             netSrv.RequestTx,
		StopTxFlow:            netSrv.StopTxFlow,
		Wallet:                cfg.ApplicationConfiguration.Consensus.UnlockWallet,
		TimePerBlock:          serverConfig.TimePerBlock,
	})
	require.NoError(t, err)
	netSrv.AddConsensusService(cons, cons.OnPayload, cons.OnTransaction)
	go netSrv.Start(make(chan error, 1))
	errCh := make(chan error, 2)
	rpcServer := rpcsrv.New(chain, cfg.ApplicationConfiguration.RPC, netSrv, nil, logger, errCh)
	rpcServer.Start()

	return chain, &rpcServer, netSrv
}

func NewExecutor(t *testing.T, needChain bool) *Executor {
	return NewExecutorWithConfig(t, needChain, true, nil)
}

func NewExecutorSuspended(t *testing.T) *Executor {
	return NewExecutorWithConfig(t, true, false, nil)
}

func NewExecutorWithConfig(t *testing.T, needChain, runChain bool, f func(*config.Config)) *Executor {
	e := &Executor{
		CLI: app.New(),
		Out: NewConcurrentBuffer(),
		Err: bytes.NewBuffer(nil),
		In:  bytes.NewBuffer(nil),
	}
	e.CLI.Writer = e.Out
	e.CLI.ErrWriter = e.Err
	if needChain {
		e.Chain, e.RPC, e.NetSrv = NewTestChain(t, f, runChain)
	}
	t.Cleanup(func() {
		e.Close(t)
	})
	return e
}

func (e *Executor) Close(t *testing.T) {
	input.Terminal = nil
	if e.RPC != nil {
		e.RPC.Shutdown()
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
func (e *Executor) GetTransaction(t *testing.T, h util.Uint256) (*transaction.Transaction, uint32) {
	var tx *transaction.Transaction
	var height uint32
	require.Eventually(t, func() bool {
		var err error
		tx, height, err = e.Chain.GetTransaction(h)
		return err == nil && height != math.MaxUint32
	}, time.Second*2, time.Millisecond*100, "too long time waiting for block")
	return tx, height
}

func (e *Executor) GetNextLine(t *testing.T) string {
	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	return strings.TrimSuffix(line, "\n")
}

func (e *Executor) CheckNextLine(t *testing.T, expected string) {
	line := e.GetNextLine(t)
	e.CheckLine(t, line, expected)
}

func (e *Executor) CheckLine(t *testing.T, line, expected string) {
	require.Regexp(t, expected, line)
}

func (e *Executor) CheckEOF(t *testing.T) {
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
func (e *Executor) RunWithError(t *testing.T, args ...string) {
	ch := setExitFunc()
	require.Error(t, e.run(args...))
	checkExit(t, ch, 1)
}

// Run runs command and checks that there were no errors.
func (e *Executor) Run(t *testing.T, args ...string) {
	ch := setExitFunc()
	require.NoError(t, e.run(args...))
	checkExit(t, ch, 0)
}
func (e *Executor) run(args ...string) error {
	e.Out.Reset()
	e.Err.Reset()
	input.Terminal = term.NewTerminal(input.ReadWriter{
		Reader: e.In,
		Writer: io.Discard,
	}, "")
	err := e.CLI.Run(args)
	input.Terminal = nil
	e.In.Reset()
	return err
}

func (e *Executor) CheckTxPersisted(t *testing.T, prefix ...string) (*transaction.Transaction, uint32) {
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
	require.Equal(t, vmstate.Halt, aer[0].VMState)
	return tx, height
}

func GenerateKeys(t *testing.T, n int) ([]*keys.PrivateKey, keys.PublicKeys) {
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

func (e *Executor) CheckTxTestInvokeOutput(t *testing.T, scriptSize int) {
	e.CheckNextLine(t, `Hash:\s+`)
	e.CheckNextLine(t, `OnChain:\s+false`)
	e.CheckNextLine(t, `ValidUntil:\s+\d+`)
	e.CheckNextLine(t, `Signer:\s+\w+`)
	e.CheckNextLine(t, `SystemFee:\s+(\d|\.)+`)
	e.CheckNextLine(t, `NetworkFee:\s+(\d|\.)+`)
	e.CheckNextLine(t, `Script:\s+\w+`)
	e.CheckScriptDump(t, scriptSize)
}

func (e *Executor) CheckScriptDump(t *testing.T, scriptSize int) {
	e.CheckNextLine(t, `INDEX\s+`)
	for i := 0; i < scriptSize; i++ {
		e.CheckNextLine(t, `\d+\s+\w+`)
	}
}

func DeployContract(t *testing.T, e *Executor, inPath, configPath, wallet, address, pass string) util.Uint160 {
	config.Version = "0.90.0-test" // Contracts are compiled and we want NEFs to not change from run to run.
	tmpDir := t.TempDir()
	nefName := filepath.Join(tmpDir, "contract.nef")
	manifestName := filepath.Join(tmpDir, "contract.manifest.json")
	e.Run(t, "neo-go", "contract", "compile",
		"--in", inPath,
		"--config", configPath,
		"--out", nefName, "--manifest", manifestName)
	e.In.WriteString(pass + "\r")
	e.Run(t, "neo-go", "contract", "deploy",
		"--rpc-endpoint", "http://"+e.RPC.Addresses()[0],
		"--wallet", wallet, "--address", address,
		"--force",
		"--in", nefName, "--manifest", manifestName)
	e.CheckTxPersisted(t, "Sent invocation transaction ")
	line, err := e.Out.ReadString('\n')
	require.NoError(t, err)
	line = strings.TrimSpace(strings.TrimPrefix(line, "Contract: "))
	h, err := util.Uint160DecodeStringLE(line)
	require.NoError(t, err)
	return h
}
