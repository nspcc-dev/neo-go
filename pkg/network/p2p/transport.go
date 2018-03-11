package p2p

// Transporter is an interface that allows us to abstract
// the network communication between the server and its peers.
type Transporter interface {
	Proto() string
	Accept(*Server)
	Close()
}
