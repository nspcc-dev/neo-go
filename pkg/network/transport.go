package network

import "time"

// Transporter is an interface that allows us to abstract
// the network communication between the server and its peers.
type Transporter interface {
	Consumer() <-chan protoTuple
	Dial(addr string, timeout time.Duration) error
	Accept(*Server)
	Proto() string
	Close()
}
