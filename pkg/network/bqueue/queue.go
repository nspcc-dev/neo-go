package bqueue

import (
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Queuer is an interface for a queue.
type Queuer[Q Queueable] interface {
	AddItem(Q) error
	AddItems(...Q) error
	Height() uint32
}

// OperationMode is the mode of operation for the queue.
// Could be either Blocking or NonBlocking.
type OperationMode byte

const (
	// NonBlocking means that Put will return immediately if the queue is full.
	NonBlocking OperationMode = 0
	// Blocking means that Put will wait until there is enough space in the queue.
	Blocking OperationMode = 1
)

// Queueable is an interface for a queue element.
type Queueable interface {
	Indexable
	comparable
}

// Indexable is an interface for an element that has an index.
type Indexable interface {
	GetIndex() uint32
}

// Queue is the queue of queueable elements.
type Queue[Q Queueable] struct {
	log         *zap.Logger
	queueLock   sync.RWMutex
	queue       []Q
	lastQ       uint32
	checkBlocks chan struct{}
	chain       Queuer[Q]
	relayF      func(Q)
	discarded   atomic.Bool
	len         int
	lenUpdateF  func(int)
	cacheSize   int
	mode        OperationMode
	nilQ        Q
}

// DefaultCacheSize is the default amount of Queueable elements above the current height
// which are stored in the queue.
const DefaultCacheSize = 2000

func (bq *Queue[Q]) indexToPosition(i uint32) int {
	return int(i) % bq.cacheSize
}

// New creates an instance of Queue that handles Queueable elements.
func New[Q Queueable](bc Queuer[Q], log *zap.Logger, relayer func(Q), cacheSize int, lenMetricsUpdater func(l int), mode OperationMode) *Queue[Q] {
	if log == nil {
		return nil
	}
	if cacheSize <= 0 {
		cacheSize = DefaultCacheSize
	}
	var nilQ Q
	return &Queue[Q]{
		log:         log,
		queue:       make([]Q, cacheSize),
		checkBlocks: make(chan struct{}, 1),
		chain:       bc,
		relayF:      relayer,
		lenUpdateF:  lenMetricsUpdater,
		cacheSize:   cacheSize,
		mode:        mode,
		nilQ:        nilQ,
	}
}

// Run runs the Queue queueing loop. It must be called in a separate routine.
func (bq *Queue[Q]) Run() {
	var lastHeight = bq.chain.Height()
	for {
		_, ok := <-bq.checkBlocks
		if !ok {
			break
		}
		for {
			h := bq.chain.Height()
			pos := bq.indexToPosition(h + 1)
			bq.queueLock.Lock()
			b := bq.queue[pos]
			// The chain moved forward using elements from other sources (consensus).
			for i := lastHeight; i < h; i++ {
				old := bq.indexToPosition(i + 1)
				if bq.queue[old] != bq.nilQ && bq.queue[old].GetIndex() == i {
					bq.len--
					bq.queue[old] = bq.nilQ
				}
			}
			bq.queueLock.Unlock()
			lastHeight = h
			if b == bq.nilQ {
				break
			}

			err := bq.chain.AddItem(b)
			if err != nil {
				// The element might already be added by the consensus.
				if bq.chain.Height() < b.GetIndex() {
					bq.log.Warn("Queue: failed adding item into the blockchain",
						zap.String("error", err.Error()),
						zap.Uint32("height", bq.chain.Height()),
						zap.Uint32("nextIndex", b.GetIndex()))
				}
			} else if bq.relayF != nil {
				bq.relayF(b)
			}
			bq.queueLock.Lock()
			bq.len--
			l := bq.len
			if bq.queue[pos] == b {
				bq.queue[pos] = bq.nilQ
			}
			bq.queueLock.Unlock()
			if bq.lenUpdateF != nil {
				bq.lenUpdateF(l)
			}
		}
	}
}

// Put enqueues Queueable element to be added to the chain.
func (bq *Queue[Q]) Put(element Q) error {
	h := bq.chain.Height()
	bq.queueLock.Lock()
	defer bq.queueLock.Unlock()
	if bq.discarded.Load() {
		return nil
	}
	// Can easily happen when fetching the same blocks from
	// different peers, thus not considered as error.
	if element.GetIndex() <= h {
		return nil
	}
	if h+uint32(bq.cacheSize) < element.GetIndex() {
		switch bq.mode {
		case NonBlocking:
			return nil
		case Blocking:
			bq.queueLock.Unlock()
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for range t.C {
				if bq.discarded.Load() {
					bq.queueLock.Lock()
					return nil
				}
				h = bq.chain.Height()
				if h+uint32(bq.cacheSize) >= element.GetIndex() {
					bq.queueLock.Lock()
					break
				}
			}
		}
	}
	pos := bq.indexToPosition(element.GetIndex())
	// If we already have it, keep the old element, throw away the new one.
	if bq.queue[pos] == bq.nilQ || bq.queue[pos].GetIndex() < element.GetIndex() {
		bq.len++
		bq.queue[pos] = element
		for pos < bq.cacheSize && bq.queue[pos] != bq.nilQ && bq.lastQ+1 == bq.queue[pos].GetIndex() {
			bq.lastQ = bq.queue[pos].GetIndex()
			pos++
		}
	}
	// update metrics
	if bq.lenUpdateF != nil {
		bq.lenUpdateF(bq.len)
	}
	select {
	case bq.checkBlocks <- struct{}{}:
		// ok, signalled to goroutine processing queue
	default:
		// it's already busy processing elements
	}
	return nil
}

// LastQueued returns the index of the last queued element and the queue's capacity
// left.
func (bq *Queue[Q]) LastQueued() (uint32, int) {
	bq.queueLock.RLock()
	defer bq.queueLock.RUnlock()
	return bq.lastQ, bq.cacheSize - bq.len
}

// Discard stops the queue and prevents it from accepting more elements to enqueue.
func (bq *Queue[Q]) Discard() {
	if bq.discarded.CompareAndSwap(false, true) {
		bq.queueLock.Lock()
		close(bq.checkBlocks)
		// Technically we could bq.queue = nil, but this would cost
		// another if in Run().
		clear(bq.queue)
		bq.len = 0
		bq.queueLock.Unlock()
	}
}
