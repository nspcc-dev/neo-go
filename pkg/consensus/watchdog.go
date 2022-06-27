package consensus

import (
	"errors"
	"time"

	coreb "github.com/nspcc-dev/neo-go/pkg/core/block"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type Watchdog struct {
	WatchdogConfig

	// blockEvents is used to pass a new block event to the consensus
	// process.
	blockEvents chan *coreb.Block

	log *zap.Logger
	// started is a flag set with Start method that runs an event handling
	// goroutine.
	started  *atomic.Bool
	quit     chan struct{}
	finished chan struct{}
}

type WatchdogConfig struct {
	Logger *zap.Logger
	// Chain is a Ledger instance.
	Chain Ledger
	// ConsensusRestartChan is a channel used to send restart signal to the consensus service caller
	// if consensus watchdog is on.
	ConsensusRestartChan chan struct{}
}

func NewWatchdog(cfg WatchdogConfig) (*Watchdog, error) {
	if cfg.Logger == nil {
		return nil, errors.New("empty logger")
	}
	wd := &Watchdog{
		WatchdogConfig: cfg,
		log:            cfg.Logger,
		blockEvents:    make(chan *coreb.Block, 1),
		started:        atomic.NewBool(false),
		quit:           make(chan struct{}),
		finished:       make(chan struct{}),
	}
	return wd, nil
}
func (w *Watchdog) Start() {
	if w.started.CAS(false, true) {
		w.log.Info("starting consensus watchdog service")
		w.Chain.SubscribeForBlocks(w.blockEvents)
		go w.eventLoop()
	}
}

func (w *Watchdog) eventLoop() {
	cfg := w.Chain.GetConfig()
	latestBlock, err := w.Chain.GetBlock(w.Chain.CurrentBlockHash())
	if err != nil {
		w.log.Error("failed to retrieve last block timestamp",
			zap.Error(err))
		close(w.finished)
		return
	}
	threshold := time.Second * time.Duration(cfg.DBFTWatchdogThresholdMultiplier*cfg.SecondsPerBlock)
	_, resetAfter := calculateReset(latestBlock.Timestamp, threshold)
	timer := time.NewTimer(resetAfter)

events:
	for {
		select {
		case <-w.quit:
			w.Chain.UnsubscribeFromBlocks(w.blockEvents)
			if !timer.Stop() {
				<-timer.C
			}
			break events
		case b := <-w.blockEvents:
			if b.Index > latestBlock.Index {
				latestBlock = b
				_, resetAfter = calculateReset(latestBlock.Timestamp, threshold)
				timer.Reset(resetAfter)
			}
		case <-timer.C:
			now, resetAfter := calculateReset(latestBlock.Timestamp, threshold)
			timer.Reset(resetAfter)
			w.log.Warn("couldn't accept new block, sending signal to restart consensus service",
				zap.Uint32("latest block index", latestBlock.Index),
				zap.Uint64("latest block timestamp", latestBlock.Timestamp),
				zap.Duration("time since latest block", time.Millisecond*time.Duration(now-int64(latestBlock.Timestamp))),
				zap.Duration("time till next restart", resetAfter))
			w.ConsensusRestartChan <- struct{}{}
		}
	}

drainBlocksLoop:
	for {
		select {
		case <-w.blockEvents:
		default:
			break drainBlocksLoop
		}
	}
	close(w.blockEvents)
	close(w.finished)
}

func calculateReset(latestTimestamp uint64, threshold time.Duration) (int64, time.Duration) {
	now := time.Now().UnixMilli()
	delta := time.Millisecond * time.Duration(int64(latestTimestamp)-now)
	resetAfter := delta
	for {
		resetAfter += threshold
		if resetAfter > 0 {
			break
		}
	}
	return now, resetAfter
}

func (w *Watchdog) Name() string {
	return "consensus watchdog"
}

func (w *Watchdog) Shutdown() {
	if w.started.Load() {
		close(w.quit)
		<-w.finished
	}
}
