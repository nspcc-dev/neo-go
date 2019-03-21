package connmgr

import (
	"net"
)

// Request is a layer on top of connection and allows us to add metadata to the net.Conn
// that the connection manager can use to determine whether to retry and other useful heuristics
type Request struct {
	Conn      net.Conn
	Addr      string
	Permanent bool
	Inbound   bool
	Retries   uint8 // should not be trying more than 255 tries
}
