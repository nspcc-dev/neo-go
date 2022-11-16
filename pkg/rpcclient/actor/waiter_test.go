package actor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

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
	w := newWaiter((RPCActor)(nil), nil)
	_, ok := w.(NullWaiter)
	require.True(t, ok)

	w = newWaiter(&RPCClient{}, &result.Version{})
	_, ok = w.(*PollingWaiter)
	require.True(t, ok)

	w = newWaiter(&AwaitableRPCClient{RPCClient: RPCClient{}}, &result.Version{})
	_, ok = w.(*EventWaiter)
	require.True(t, ok)
}

func TestPollingWaiter_Wait(t *testing.T) {
	h := util.Uint256{1, 2, 3}
	bCount := uint32(5)
	appLog := &result.ApplicationLog{Container: h, Executions: []state.Execution{{}}}
	expected := &state.AppExecResult{Container: h, Execution: state.Execution{}}
	c := &RPCClient{appLog: appLog}
	c.bCount.Store(bCount)
	w := newWaiter(c, &result.Version{Protocol: result.Protocol{MillisecondsPerBlock: 1}}) // reduce testing time.
	_, ok := w.(*PollingWaiter)
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
	require.ErrorIs(t, ErrTxNotAccepted, err)

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
	checkErr(t, func() { c.bCount.Store(bCount + 1) }, ErrTxNotAccepted)

	// Context is cancelled.
	c.appLog = nil
	c.bCount.Store(bCount)
	ctx, cancel := context.WithCancel(context.Background())
	c.context = ctx
	checkErr(t, cancel, ErrContextDone)
}

func TestWSWaiter_Wait(t *testing.T) {
	h := util.Uint256{1, 2, 3}
	bCount := uint32(5)
	appLog := &result.ApplicationLog{Container: h, Executions: []state.Execution{{}}}
	expected := &state.AppExecResult{Container: h, Execution: state.Execution{}}
	c := &AwaitableRPCClient{RPCClient: RPCClient{appLog: appLog}}
	c.bCount.Store(bCount)
	w := newWaiter(c, &result.Version{Protocol: result.Protocol{MillisecondsPerBlock: 1}}) // reduce testing time.
	_, ok := w.(*EventWaiter)
	require.True(t, ok)

	// Wait with error.
	someErr := errors.New("some error")
	_, err := w.Wait(h, bCount, someErr)
	require.ErrorIs(t, err, someErr)

	// AER is in chain immediately.
	doneCh := make(chan struct{})
	go func() {
		aer, err := w.Wait(h, bCount-1, nil)
		require.NoError(t, err)
		require.Equal(t, expected, aer)
		doneCh <- struct{}{}
	}()
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
	check(t, func() {
		c.chLock.RLock()
		defer c.chLock.RUnlock()
		c.subTxCh <- expected
	})

	// Missing AER after VUB.
	c.RPCClient.appLog = nil
	go func() {
		_, err = w.Wait(h, bCount-2, nil)
		require.ErrorIs(t, err, ErrTxNotAccepted)
		doneCh <- struct{}{}
	}()
	check(t, func() {
		c.chLock.RLock()
		defer c.chLock.RUnlock()
		c.subBlockCh <- &block.Block{}
	})
}
