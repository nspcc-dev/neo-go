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
func New(cfg Config) (*Connmgr, error) {
	listener, err := net.Listen("tcp", cfg.AddressPort)

	if err != nil {
		return nil, err
	}

	cnnmgr := &Connmgr{
		cfg,
		make(map[string]*Request),
		make(map[string]*Request),
		make(chan func(), 300),
	}

	go func() {
		defer func() {
			listener.Close()
		}()

		for {

			conn, err := listener.Accept()

			if err != nil {
				continue
			}
			go cfg.OnAccept(conn)
		}

	}()

	return cnnmgr, nil
}

// NewRequest will make a new connection gets the address from address func in config
// Then dials it and assigns it to pending
func (c *Connmgr) NewRequest() error {

	// Fetch address
	addr, err := c.config.GetAddress()
	if err != nil {
		return fmt.Errorf("error getting address " + err.Error())
	}

	r := &Request{
		Addr: addr,
	}
	return c.Connect(r)
}

// Connect will dial the address in the Request
// Updating the request object depending on the outcome
func (c *Connmgr) Connect(r *Request) error {

	r.Retries++

	conn, err := c.dial(r.Addr)
	if err != nil {
		c.failed(r)
		return err
	}

	r.Conn = conn
	r.Inbound = true

	// r.Permanent is set by the address manager/caller. default is false
	// The permanent connections will be the ones that are hardcoded, e.g seed3.ngd.network
	// or are reliable. The connmgr will be more leniennt to permanent addresses as they have
	// a track record or reputation of being reliable.

	return c.connected(r)
}

//Disconnect will remove the request from the connected/pending list and close the connection
func (c *Connmgr) Disconnect(addr string) {

	var r *Request

	// fetch from connected list
	r, ok := c.ConnectedList[addr]
	if !ok {
		// If not in connected, check pending
		r, _ = c.PendingList[addr]
	}

	c.disconnected(r)

}

// Dial is used to dial up connections given the addres and ip in the form address:port
func (c *Connmgr) dial(addr string) (net.Conn, error) {
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
func (c *Connmgr) failed(r *Request) {

	c.actionch <- func() {
		// priority to check if it is permanent or inbound
		// if so then these peers are valuable in NEO and so we will just retry another time
		if r.Inbound || r.Permanent {
			multiplier := time.Duration(r.Retries * 10)
			time.AfterFunc(multiplier*time.Second,
				func() {
					c.Connect(r)
				},
			)
			// if not then we should check if this request has had maxRetries
			// if it has then get a new address
			// if not then call Connect on it again
		} else if r.Retries > maxRetries {
			if c.config.GetAddress != nil {
				go c.NewRequest()
			}
		} else {
			go c.Connect(r)
		}
	}

}

// Disconnected is called when a peer disconnects.
// we take the addr from peer, which is also it's key in the map
// and we use it to remove it from the connectedList
func (c *Connmgr) disconnected(r *Request) error {

	if r == nil {
		// if object is nil, we return nil
		return nil
	}

	// if for some reason the underlying connection is not closed, close it
	err := r.Conn.Close()
	if err != nil {
		return err
	}

	// remove from any pending/connected list
	delete(c.PendingList, r.Addr)
	delete(c.ConnectedList, r.Addr)

	// If permanent,then lets retry
	if r.Permanent {
		return c.Connect(r)
	}

	return nil
}

//Connected is called when the connection manager makes a successful connection.
func (c *Connmgr) connected(r *Request) error {

	// This should not be the case, since we connected
	if r == nil {
		return errors.New("request object as nil inside of the connected function")
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

	return nil
}

// Pending is synchronous, we do not want to continue with logic
// until we are certain it has been added to the pendingList
func (c *Connmgr) pending(r *Request) error {

	if r == nil {
		return errors.New("request object is nil")
	}

	c.PendingList[r.Addr] = r

	return nil
}

// Run will start the connection manager
func (c *Connmgr) Run() error {
	fmt.Println("Connection manager started")
	go c.loop()
	return nil
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
