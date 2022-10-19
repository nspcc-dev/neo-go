package actor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// PollingWaiterRetryCount is a threshold for a number of subsequent failed
// attempts to get block count from the RPC server for PollingWaiter. If it fails
// to retrieve block count PollingWaiterRetryCount times in a raw then transaction
// awaiting attempt considered to be failed and an error is returned.
const PollingWaiterRetryCount = 3

var (
	// ErrTxNotAccepted is returned when transaction wasn't accepted to the chain
	// even after ValidUntilBlock block persist.
	ErrTxNotAccepted = errors.New("transaction was not accepted to chain")
	// ErrContextDone is returned when Waiter context has been done in the middle
	// of transaction awaiting process and no result was received yet.
	ErrContextDone = errors.New("waiter context done")
)

type (
	// RPCPollingWaiter is an interface that enables transaction awaiting functionality
	// for Actor instance based on periodical BlockCount and ApplicationLog polls.
	RPCPollingWaiter interface {
		// Context should return the RPC client context to be able to gracefully
		// shut down all running processes (if so).
		Context() context.Context
		GetBlockCount() (uint32, error)
		GetApplicationLog(hash util.Uint256, trig *trigger.Type) (*result.ApplicationLog, error)
	}
	// RPCEventWaiter is an interface that enables improved transaction awaiting functionality
	// for Actor instance based on web-socket Block and ApplicationLog notifications.
	RPCEventWaiter interface {
		RPCPollingWaiter

		SubscribeForNewBlocksWithChan(primary *int, since *uint32, till *uint32, rcvrCh chan<- rpcclient.Notification) (string, error)
		SubscribeForTransactionExecutionsWithChan(state *string, container *util.Uint256, rcvrCh chan<- rpcclient.Notification) (string, error)
		Unsubscribe(id string) error
	}
)

// Wait allows to wait until transaction will be accepted to the chain. It can be
// used as a wrapper for Send or SignAndSend and accepts transaction hash,
// ValidUntilBlock value and an error. It returns transaction execution result
// or an error if transaction wasn't accepted to the chain.
func (a *Actor) Wait(h util.Uint256, vub uint32, err error) (*state.AppExecResult, error) {
	if err != nil {
		return nil, err
	}
	if wsW, ok := a.client.(RPCEventWaiter); ok {
		return a.waitWithWSWaiter(wsW, h, vub)
	}
	return a.waitWithSimpleWaiter(a.client, h, vub)
}

// waitWithSimpleWaiter waits until transaction is accepted to the chain and
// returns its execution result or an error if it's missing from chain after
// VUB block.
func (a *Actor) waitWithSimpleWaiter(c RPCPollingWaiter, h util.Uint256, vub uint32) (*state.AppExecResult, error) {
	var (
		currentHeight uint32
		failedAttempt int
		pollTime      = time.Millisecond * time.Duration(a.GetVersion().Protocol.MillisecondsPerBlock) / 2
	)
	if pollTime == 0 {
		pollTime = time.Second
	}
	timer := time.NewTicker(pollTime)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			blockCount, err := c.GetBlockCount()
			if err != nil {
				failedAttempt++
				if failedAttempt > PollingWaiterRetryCount {
					return nil, fmt.Errorf("failed to retrieve block count: %w", err)
				}
				continue
			}
			failedAttempt = 0
			if blockCount-1 > currentHeight {
				currentHeight = blockCount - 1
			}
			t := trigger.Application
			res, err := c.GetApplicationLog(h, &t)
			if err == nil {
				return &state.AppExecResult{
					Container: h,
					Execution: res.Executions[0],
				}, nil
			}
			if currentHeight >= vub {
				return nil, ErrTxNotAccepted
			}

		case <-c.Context().Done():
			return nil, fmt.Errorf("%w: %v", ErrContextDone, c.Context().Err())
		}
	}
}

// waitWithWSWaiter waits until transaction is accepted to the chain and returns
// its execution result or an error if it's missing from chain after VUB block.
// It uses optimized web-socket waiter if possible.
func (a *Actor) waitWithWSWaiter(c RPCEventWaiter, h util.Uint256, vub uint32) (res *state.AppExecResult, waitErr error) {
	var wsWaitErr error
	defer func() {
		if wsWaitErr != nil {
			res, waitErr = a.waitWithSimpleWaiter(c, h, vub)
			if waitErr != nil {
				waitErr = fmt.Errorf("WS waiter error: %w, simple waiter error: %v", wsWaitErr, waitErr)
			}
		}
	}()
	rcvr := make(chan rpcclient.Notification)
	defer func() {
	drainLoop:
		// Drain rcvr to avoid other notification receivers blocking.
		for {
			select {
			case <-rcvr:
			default:
				break drainLoop
			}
		}
		close(rcvr)
	}()
	// Execution event follows the block event, thus wait until the block next to the VUB to be sure.
	since := vub + 1
	blocksID, err := c.SubscribeForNewBlocksWithChan(nil, &since, nil, rcvr)
	if err != nil {
		wsWaitErr = fmt.Errorf("failed to subscribe for new blocks: %w", err)
		return
	}
	defer func() {
		err = c.Unsubscribe(blocksID)
		if err != nil {
			errFmt := "failed to unsubscribe from blocks (id: %s): %v"
			errArgs := []interface{}{blocksID, err}
			if waitErr != nil {
				errFmt += "; wait error: %w"
				errArgs = append(errArgs, waitErr)
			}
			waitErr = fmt.Errorf(errFmt, errArgs...)
		}
	}()
	txsID, err := c.SubscribeForTransactionExecutionsWithChan(nil, &h, rcvr)
	if err != nil {
		wsWaitErr = fmt.Errorf("failed to subscribe for execution results: %w", err)
		return
	}
	defer func() {
		err = c.Unsubscribe(txsID)
		if err != nil {
			errFmt := "failed to unsubscribe from transactions (id: %s): %v"
			errArgs := []interface{}{txsID, err}
			if waitErr != nil {
				errFmt += "; wait error: %w"
				errArgs = append(errArgs, waitErr)
			}
			waitErr = fmt.Errorf(errFmt, errArgs...)
		}
	}()

	for {
		select {
		case ntf := <-rcvr:
			switch ntf.Type {
			case neorpc.BlockEventID:
				waitErr = ErrTxNotAccepted
				return
			case neorpc.ExecutionEventID:
				res = ntf.Value.(*state.AppExecResult)
				return
			case neorpc.MissedEventID:
				// We're toast, retry with non-ws client.
				wsWaitErr = errors.New("some event was missed")
				return
			}
		case <-c.Context().Done():
			waitErr = fmt.Errorf("%w: %v", ErrContextDone, c.Context().Err())
			return
		}
	}
}
