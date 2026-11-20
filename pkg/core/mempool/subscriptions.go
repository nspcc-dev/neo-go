package mempool

import "github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"

const (
	subsNotStarted int32 = iota
	subsRunning
	subsStopped
)

// RunSubscriptions runs subscriptions goroutine if mempool subscriptions are enabled.
// You should manually free the resources by calling StopSubscriptions on mempool shutdown.
func (mp *Pool) RunSubscriptions() {
	if !mp.subscriptionsEnabled {
		panic("subscriptions are disabled")
	}
	if mp.subsState.CompareAndSwap(subsNotStarted, subsRunning) {
		go mp.notificationDispatcher()
	}
}

// StopSubscriptions stops mempool events loop and waits for it to finish
// processing in-flight events.
func (mp *Pool) StopSubscriptions() {
	if !mp.subscriptionsEnabled {
		panic("subscriptions are disabled")
	}
	if mp.subsState.CompareAndSwap(subsRunning, subsStopped) {
		close(mp.stopCh)
	}
	if mp.subsState.Load() == subsStopped {
		<-mp.dispToExitCh
	}
}

// SubscribeForTransactions adds the given channel to the new mempool event broadcasting, so when
// there is a new transactions added to the mempool or an existing transaction removed from
// the mempool, you'll receive it via this channel. Make sure you're not changing the received
// mempool events, as it may affect the functionality of other subscribers.
func (mp *Pool) SubscribeForTransactions(ch chan<- mempoolevent.Event) {
	if mp.subsState.Load() == subsRunning {
		select {
		case mp.subCh <- ch:
		case <-mp.stopCh:
		}
	}
}

// UnsubscribeFromTransactions unsubscribes the given channel from new mempool notifications,
// you can close it afterwards. Passing non-subscribed channel is a no-op.
func (mp *Pool) UnsubscribeFromTransactions(ch chan<- mempoolevent.Event) {
	if mp.subsState.Load() != subsNotStarted {
		select {
		case mp.unsubCh <- ch:
		case <-mp.dispToExitCh:
		}
	}
}

func (mp *Pool) notify(ev mempoolevent.Event) {
	if mp.subsState.Load() == subsRunning {
		select {
		case mp.events <- ev:
		case <-mp.stopCh:
		}
	}
}

// notificationDispatcher manages subscription to events and broadcasts new events.
func (mp *Pool) notificationDispatcher() {
	defer close(mp.dispToExitCh)
	var (
		// These are just sets of subscribers, though modelled as maps
		// for ease of management (not a lot of subscriptions is really
		// expected, but maps are convenient for adding/deleting elements).
		txFeed = make(map[chan<- mempoolevent.Event]bool)
	)
	for {
		select {
		case <-mp.stopCh:
			return
		case sub := <-mp.subCh:
			txFeed[sub] = true
		case unsub := <-mp.unsubCh:
			delete(txFeed, unsub)
		case event := <-mp.events:
			for ch := range txFeed {
				ch <- event
			}
		}
	}
}
