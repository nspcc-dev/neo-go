package connmgr

import (
	"net"
)

type Request struct {
	Conn      net.Conn
	Addr      string
	Permanent bool
	Inbound   bool
	Retries   uint8 // should not be trying more than 255 tries
}
