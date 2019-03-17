package pubsub

// EventType is an enum
// representing the types of messages we can subscribe to
type EventType int

const (
	// NewBlock is called When blockchain connects a new block, it will emit an NewBlock Event
	NewBlock EventType = iota
	// BadBlock is called When blockchain declines a block, it will emit a new block event
	BadBlock
	// BadHeader is called When blockchain rejects a Header, it will emit this event
	BadHeader
)

// Event represents a new Event that a subscriber can listen to
type Event struct {
	Type EventType // E.g. event.NewBlock
	data []byte    // Raw information
}
