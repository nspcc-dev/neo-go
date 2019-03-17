package pubsub

// Subscriber will listen for Events from publishers
type Subscriber interface {
	Topics() []EventType
	Emit(Event)
}
