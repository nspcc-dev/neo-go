package waiter_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/waiter"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type RPCClient struct {
	err     error
	invRes  *result.Invoke
	netFee  int64
	bCount  atomic.Uint32
	version *result.Version
	hash    util.Uint256
	appLog  *result.ApplicationLog
	context context.Context
}

func (r *RPCClient) InvokeContractVerify(contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	return r.invRes, r.err
}
func (r *RPCClient) InvokeFunction(contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	return r.invRes, r.err
}
func (r *RPCClient) InvokeScript(script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	return r.invRes, r.err
}
func (r *RPCClient) CalculateNetworkFee(tx *transaction.Transaction) (int64, error) {
	return r.netFee, r.err
}
func (r *RPCClient) GetBlockCount() (uint32, error) {
	return r.bCount.Load(), r.err
}
func (r *RPCClient) GetVersion() (*result.Version, error) {
	verCopy := *r.version
	return &verCopy, r.err
}
func (r *RPCClient) SendRawTransaction(tx *transaction.Transaction) (util.Uint256, error) {
	return r.hash, r.err
}
func (r *RPCClient) TerminateSession(sessionID uuid.UUID) (bool, error) {
	return false, nil // Just a stub, unused by actor.
}
func (r *RPCClient) TraverseIterator(sessionID, iteratorID uuid.UUID, maxItemsCount int) ([]stackitem.Item, error) {
	return nil, nil // Just a stub, unused by actor.
}
func (r *RPCClient) Context() context.Context {
	if r.context == nil {
		return context.Background()
	}
	return r.context
}

func (r *RPCClient) GetApplicationLog(hash util.Uint256, trig *trigger.Type) (*result.ApplicationLog, error) {
	if r.appLog != nil {
		return r.appLog, nil
	}
	return nil, errors.New("not found")
}

type AwaitableRPCClient struct {
	RPCClient

	chLock     sync.RWMutex
	subBlockCh chan<- *block.Block
	subTxCh    chan<- *state.AppExecResult
}

func (c *AwaitableRPCClient) ReceiveBlocks(flt *neorpc.BlockFilter, rcvr chan<- *block.Block) (string, error) {
	c.chLock.Lock()
	defer c.chLock.Unlock()
	c.subBlockCh = rcvr
	return "1", nil
}
func (c *AwaitableRPCClient) ReceiveExecutions(flt *neorpc.ExecutionFilter, rcvr chan<- *state.AppExecResult) (string, error) {
	c.chLock.Lock()
	defer c.chLock.Unlock()
	c.subTxCh = rcvr
	return "2", nil
}
func (c *AwaitableRPCClient) Unsubscribe(id string) error { return nil }

func TestNewWaiter(t *testing.T) {
	w := waiter.New((actor.RPCActor)(nil), nil)
	_, ok := w.(waiter.NullWaiter)
	require.True(t, ok)

	w = waiter.New(&RPCClient{}, &result.Version{})
	_, ok = w.(*waiter.PollingWaiter)
	require.True(t, ok)

	w = waiter.New(&AwaitableRPCClient{RPCClient: RPCClient{}}, &result.Version{})
	_, ok = w.(*waiter.EventWaiter)
	require.True(t, ok)
}

func TestPollingWaiter_Wait(t *testing.T) {
	h := util.Uint256{1, 2, 3}
	bCount := uint32(5)
	appLog := &result.ApplicationLog{Container: h, Executions: []state.Execution{{}}}
	expected := &state.AppExecResult{Container: h, Execution: state.Execution{}}
	c := &RPCClient{appLog: appLog}
	c.bCount.Store(bCount)
	w := waiter.New(c, &result.Version{Protocol: result.Protocol{MillisecondsPerBlock: 1}}) // reduce testing time.
	_, ok := w.(*waiter.PollingWaiter)
	require.True(t, ok)

	// Wait with error.
	someErr := errors.New("some error")
	_, err := w.Wait(h, bCount, someErr)
	require.ErrorIs(t, err, someErr)

	// AER is in chain immediately.
	aer, err := w.Wait(h, bCount-1, nil)
	require.NoError(t, err)
	require.Equal(t, expected, aer)

	// Missing AER after VUB.
	c.appLog = nil
	_, err = w.Wait(h, bCount-2, nil)
	require.ErrorIs(t, waiter.ErrTxNotAccepted, err)

	checkErr := func(t *testing.T, trigger func(), target error) {
		errCh := make(chan error)
		go func() {
			_, err = w.Wait(h, bCount, nil)
			errCh <- err
		}()
		timer := time.NewTimer(time.Second)
		var triggerFired bool
	waitloop:
		for {
			select {
			case err = <-errCh:
				require.ErrorIs(t, err, target)
				break waitloop
			case <-timer.C:
				if triggerFired {
					t.Fatal("failed to await result")
				}
				trigger()
				triggerFired = true
				timer.Reset(time.Second * 2)
			}
		}
		require.True(t, triggerFired)
	}

	// Tx is accepted before VUB.
	c.appLog = nil
	c.bCount.Store(bCount)
	checkErr(t, func() { c.bCount.Store(bCount + 1) }, waiter.ErrTxNotAccepted)

	// Context is cancelled.
	c.appLog = nil
	c.bCount.Store(bCount)
	ctx, cancel := context.WithCancel(context.Background())
	c.context = ctx
	checkErr(t, cancel, waiter.ErrContextDone)
}

func TestWSWaiter_Wait(t *testing.T) {
	h := util.Uint256{1, 2, 3}
	bCount := uint32(5)
	appLog := &result.ApplicationLog{Container: h, Executions: []state.Execution{{}}}
	expected := &state.AppExecResult{Container: h, Execution: state.Execution{}}
	c := &AwaitableRPCClient{RPCClient: RPCClient{appLog: appLog}}
	c.bCount.Store(bCount)
	w := waiter.New(c, &result.Version{Protocol: result.Protocol{MillisecondsPerBlock: 1}}) // reduce testing time.
	_, ok := w.(*waiter.EventWaiter)
	require.True(t, ok)

	// Wait with error.
	someErr := errors.New("some error")
	_, err := w.Wait(h, bCount, someErr)
	require.ErrorIs(t, err, someErr)

	// AER is in chain immediately.
	aer, err := w.Wait(h, bCount-1, nil)
	require.NoError(t, err)
	require.Equal(t, expected, aer)

	// Auxiliary things for asynchronous tests.
	doneCh := make(chan struct{})
	check := func(t *testing.T, trigger func()) {
		timer := time.NewTimer(time.Second)
		var triggerFired bool
	waitloop:
		for {
			select {
			case <-doneCh:
				break waitloop
			case <-timer.C:
				if triggerFired {
					t.Fatal("failed to await result")
				}
				trigger()
				triggerFired = true
				timer.Reset(time.Second * 2)
			}
		}
		require.True(t, triggerFired)
	}

	// AER received after the subscription.
	c.RPCClient.appLog = nil
	go func() {
		aer, err = w.Wait(h, bCount-1, nil)
		require.NoError(t, err)
		require.Equal(t, expected, aer)
		doneCh <- struct{}{}
	}()
	check(t, func() {
		c.chLock.RLock()
		defer c.chLock.RUnlock()
		c.subTxCh <- expected
	})

	// Missing AER after VUB.
	go func() {
		_, err = w.Wait(h, bCount-2, nil)
		require.ErrorIs(t, err, waiter.ErrTxNotAccepted)
		doneCh <- struct{}{}
	}()
	check(t, func() {
		c.chLock.RLock()
		defer c.chLock.RUnlock()
		c.subBlockCh <- &block.Block{}
	})
}

func TestRPCWaiterRPCClientCompat(t *testing.T) {
	_ = waiter.RPCPollingWaiter(&rpcclient.Client{})
	_ = waiter.RPCPollingWaiter(&rpcclient.WSClient{})
	_ = waiter.RPCEventWaiter(&rpcclient.WSClient{})
}
