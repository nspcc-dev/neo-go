package rpcsrv

import (
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
)

type (
	// intEvent is an internal event that has both a proper structure and
	// a websocket-ready message. It's used to serve websocket-based clients
	// as well as internal ones.
	intEvent struct {
		msg *websocket.PreparedMessage
		ntf *neorpc.Notification
	}
	// subscriber is an event subscriber.
	subscriber struct {
		writer    chan<- intEvent
		overflown atomic.Bool
		// These work like slots as there is not a lot of them (it's
		// cheaper doing it this way rather than creating a map),
		// pointing to an EventID is an obvious overkill at the moment, but
		// that's not for long.
		feeds [maxFeeds]feed
	}
	// feed stores subscriber's desired event ID with filter.
	feed struct {
		event  neorpc.EventID
		filter any
	}
)

// EventID implements neorpc.EventComparator interface and returns notification ID.
func (f feed) EventID() neorpc.EventID {
	return f.event
}

// Filter implements neorpc.EventComparator interface and returns notification filter.
func (f feed) Filter() any {
	return f.filter
}

const (
	// Maximum number of subscriptions per one client.
	maxFeeds = 16

	// This sets notification messages buffer depth. It may seem to be quite
	// big, but there is a big gap in speed between internal event processing
	// and networking communication that is combined with spiky nature of our
	// event generation process, which leads to lots of events generated in
	// a short time and they will put some pressure to this buffer (consider
	// ~500 invocation txs in one block with some notifications). At the same
	// time, this channel is about sending pointers, so it's doesn't cost
	// a lot in terms of memory used.
	notificationBufSize = 1024
)
