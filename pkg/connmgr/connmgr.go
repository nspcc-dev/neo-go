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

	// maxRetries is the maximum amount of successive retries that
	// we can have before we stop dialing that peer
	maxRetries = uint8(5)
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
	fmt.Println("Connecting")
	c.Connect(r)

}

func (c *Connmgr) Connect(r *Request) error {

	r.Retries++

	conn, err := c.Dial(r.Addr)
	if err != nil {
		c.failed(r)
		return err
	}

	r.Conn = conn
	r.Inbound = true

	// r.Permanent is set by the caller. default is false
	// The permanent connections will be the ones that are hardcoded, e.g seed3.ngd.network

	return c.connected(r)
}

func (cm *Connmgr) Disconnect(addr string) {

	// fetch from connected list
	r, ok := cm.ConnectedList[addr]

	if !ok {
		// If not in connected, check pending
		r, ok = cm.PendingList[addr]
	}

	cm.disconnected(r)

}

// Dial is used to dial up connections given the addres and ip in the form address:port
func (c *Connmgr) Dial(addr string) (net.Conn, error) {
	dialTimeout := 1 * time.Second
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		if !isConnected() {
			return nil, errors.New("Fatal Error: You do not seem to be connected to the internet")
		}
		return conn, err
	}
	return conn, nil
}
func (cm *Connmgr) failed(r *Request) {

	cm.actionch <- func() {
		// priority to check if it is permanent or inbound
		// if so then these peers are valuable in NEO and so we will just retry another time
		if r.Inbound || r.Permanent {

			multiplier := time.Duration(r.Retries * 10)
			time.AfterFunc(multiplier*time.Second,
				func() {
					cm.Connect(r)
				},
			)
			// if not then we should check if this request has had maxRetries
			// if it has then get a new address
			// if not then call Connect on it again
		} else if r.Retries > maxRetries {
			if cm.config.GetAddress != nil {
				go cm.NewRequest()
			}
			fmt.Println("This peer has been tried the maximum amount of times and a source of new address has not been specified.")
		} else {
			go cm.Connect(r)
		}

	}

}

// Disconnected is called when a peer disconnects.
// we take the addr from peer, which is also it's key in the map
// and we use it to remove it from the connectedList
func (c *Connmgr) disconnected(r *Request) error {

	errChan := make(chan error, 0)

	c.actionch <- func() {

		var err error

		if r == nil {
			err = errors.New("Request object is nil")
		}

		r2 := *r // dereference it, so that r.Addr is not lost on delete

		// if for some reason the underlying connection is not closed, close it
		r.Conn.Close()
		r.Conn = nil
		// if for some reason it is in pending list, remove it
		delete(c.PendingList, r.Addr)
		delete(c.ConnectedList, r.Addr)
		c.failed(&r2)
		errChan <- err
	}

	return <-errChan
}

//Connected is called when the connection manager
// makes a successful connection.
func (c *Connmgr) connected(r *Request) error {

	errorChan := make(chan error, 0)

	c.actionch <- func() {

		var err error

		// This should not be the case, since we connected
		// Keeping it here to be safe
		if r == nil {
			err = errors.New("Request object as nil inside of the connected function")
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

		errorChan <- err
	}

	return <-errorChan
}

// Pending is synchronous, we do not want to continue with logic
// until we are certain it has been added to the pendingList
func (c *Connmgr) pending(r *Request) error {

	errChan := make(chan error, 0)

	c.actionch <- func() {

		var err error

		if r == nil {
			err = errors.New("Error : Request object is nil")
		}

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
