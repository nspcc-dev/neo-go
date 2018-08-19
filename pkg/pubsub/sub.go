package pubsub

type Subscriber interface {
	Topics() []EventType
	Emit(Event)
}
