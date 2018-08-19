package pubsub

type EventType int

const (
	NewBlock  EventType = iota // When blockchain connects a new block, it will emit an NewBlock Event
	BadBlock                   // When blockchain declines a block, it will emit a new block event
	BadHeader                  // When blockchain rejects a Header, it will emit this event
)

type Event struct {
	Type EventType // E.g. event.NewBlock
	data []byte    // Raw information
}
