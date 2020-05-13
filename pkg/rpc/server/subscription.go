package server

import (
	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
)

type (
	// subscriber is an event subscriber.
	subscriber struct {
		writer chan<- *websocket.PreparedMessage
		ws     *websocket.Conn

		// These work like slots as there is not a lot of them (it's
		// cheaper doing it this way rather than creating a map),
		// pointing to EventID is an obvious overkill at the moment, but
		// that's not for long.
		feeds [maxFeeds]response.EventID
	}
)

const (
	// Maximum number of subscriptions per one client.
	maxFeeds = 16

	// This sets notification messages buffer depth, it may seem to be quite
	// big, but there is a big gap in speed between internal event processing
	// and networking communication that is combined with spiky nature of our
	// event generation process, which leads to lots of events generated in
	// short time and they will put some pressure to this buffer (consider
	// ~500 invocation txs in one block with some notifications). At the same
	// time this channel is about sending pointers, so it's doesn't cost
	// a lot in terms of memory used.
	notificationBufSize = 1024
)
