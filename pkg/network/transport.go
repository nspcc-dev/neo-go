package network

import "time"

// Transporter is an interface that allows us to abstract
// any form of communication between the server and its peers.
type Transporter interface {
	Dial(addr string, timeout time.Duration) error
	Accept()
	Proto() string
	Close()
}
