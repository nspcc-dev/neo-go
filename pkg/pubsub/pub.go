package pubsub

type Publisher struct {
	subs []Subscriber
}

// Send iterates over each subscriber and checks
// if they are interested in the Event
// By looking at their topics, if they are then
// the event is emitted to them
func (p *Publisher) Send(e Event) error {
	for _, sub := range p.subs {
		for _, topic := range sub.Topics() {
			if e.Type == topic {
				sub.Emit(e)
			}
		}
	}
	return nil
}
