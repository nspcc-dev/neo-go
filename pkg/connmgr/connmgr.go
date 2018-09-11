package connmgr

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

var (
	// maxOutboundConn is the maximum number of active peers
	// that the connection manager will try to have
	maxOutboundConn = 10
)

// Connmgr manages pending/active/failed cnnections
type Connmgr struct {
	config        Config
	PendingList   map[string]*Request
	ConnectedList map[string]*Request
	actionch      chan func()
}

//New creates a new connection manager
func New(cfg Config) *Connmgr {
	return &Connmgr{
		cfg,
		make(map[string]*Request),
		make(map[string]*Request),
		make(chan func(), 300),
	}
}

// NewRequest will make a new connection
// Gets the address from address func in config
// Then dials it and assigns it to pending
func (c *Connmgr) NewRequest() {

	// Fetch address
	addr, err := c.config.GetAddress()
	if err != nil {
		fmt.Println("Error getting address", err)
	}

	// empty request item
	r := &Request{}

	r.Addr = addr

	c.Connect(r)

}

func (c *Connmgr) Connect(r *Request) {
	// dial address
	conn, err := c.Dial(r.Addr)
	if err != nil {
		c.failed(r)
	}
	r.Conn = conn
	r.Inbound = true
	// r.Permanent is set by the caller. default is false
	// The permanent connections will be the ones that are hardcoded, e.g seed3.ngd.network
	c.connected(r)
}

// Dial is used to dial up connections given the addres and ip in the form address:port
func (c *Connmgr) Dial(addr string) (net.Conn, error) {
	dialTimeout := 1 * time.Second
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		if !isConnected() {
			return nil, errors.New("Fatal Error: You do not seem to be connected to the internet")
		}
		return conn, nil
	}
	return conn, nil
}
func (c *Connmgr) failed(r *Request) {

	/// Here we will have retry logic

	c.actionch <- func() {

		fmt.Println("The connecton has failed bro", len(c.actionch))
	}
}

// Disconnected is called when a peer disconnects.
// we take the addr from peer, which is also it's key in the map
// and we use it to remove it from the connectedList
func (c *Connmgr) disconnected(addr string) {

	c.actionch <- func() {
		// if for some reason the underlying connection is not closed, close it
		r, ok := c.ConnectedList[addr]
		if ok {
			r.Conn.Close()
		}
		// if for some reason it is in pending list, remove it
		delete(c.PendingList, addr)
		delete(c.ConnectedList, addr)

		// Now lets check if we should connect to it again
		// Because we have a lot of peers on neo, who connect from their laptops
		// we will check if the directon is inbound/outbound. If we connected to them, then we can retry
		// if they are also permanent then we will retry also

		if r.Inbound && len(c.ConnectedList) < maxOutboundConn || r.Permanent {
			c.Dial(r.Addr)
		}

	}
}

//Connected is called when the connection manager
// makes a successful connection.
func (c *Connmgr) connected(r *Request) {

	c.actionch <- func() {

		// This should not be the case, since we connected
		// Keeping it here to be safe
		if r == nil {
			return
		}

		// reset retries to 0
		r.Retries = 0

		// add to connectedList
		c.ConnectedList[r.Addr] = r

		// remove from pending if it was there
		delete(c.PendingList, r.Addr)

		if c.config.OnConnection != nil {
			c.config.OnConnection(r.Conn, r.Addr)
		}
	}
}

// Pending is synchronous, we do not want to continue with logic
// until we are certain it has been added to the pendingList
func (c *Connmgr) pending(r *Request) error {

	if r == nil {
		return errors.New("Error : Request object is nil")
	}

	errChan := make(chan error, 1)

	c.actionch <- func() {
		var err error
		c.PendingList[r.Addr] = r
		errChan <- err
	}

	return <-errChan
}

func (c *Connmgr) Run() {
	go c.loop()
}

func (c *Connmgr) loop() {
	for {
		select {
		case f := <-c.actionch:
			f()
		}
	}
}

// https://stackoverflow.com/questions/50056144/check-for-internet-connection-from-application
func isConnected() (ok bool) {
	_, err := http.Get("http://clients3.google.com/generate_204")
	if err != nil {
		return false
	}
	return true
}
