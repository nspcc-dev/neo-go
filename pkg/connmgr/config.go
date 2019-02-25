package connmgr

import (
	"net"
)

// Config contains all methods which will be set by the caller to setup the connection manager.
type Config struct {
	// GetAddress will return a single address for the connection manager to connect to
	GetAddress func() (string, error)

	// OnConnection is called by the connection manager when
	// we successfully connect to a peer
	// The caller should ideally inform the address manager that we have connected to this address in this function
	OnConnection func(conn net.Conn, addr string)

	// OnAccept will take a established connection
	OnAccept func(net.Conn)

	// Port is the port in the format "10333"
	Port string

	// DialTimeout is the amount of time, before we can disconnect a pending dialed connection
	DialTimeout int
}
