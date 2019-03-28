// This impl uses channels to simulate the queue handler with the actor model.
// A suitable number k ,should be set for channel size, because if #numOfMsg > k,
// we lose determinism. k chosen should be large enough that when filled, it shall indicate that
// the peer has stopped responding, since we do not have a pingMSG, we will need another way to shut down
// peers

package peer

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/command"

	"github.com/CityOfZion/neo-go/pkg/peer/stall"
	"github.com/CityOfZion/neo-go/pkg/wire"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

const (
	maxOutboundConnections = 100
	protocolVer            = protocol.DefaultVersion
	handshakeTimeout       = 30 * time.Second
	idleTimeout            = 5 * time.Minute // If no message received after idleTimeout, then peer disconnects

	// nodes will have `responseTime` seconds to reply with a response
	responseTime = 120 * time.Second

	// the stall detector will check every `tickerInterval` to see if messages
	// are overdue. Should be less than `responseTime`
	tickerInterval = 30 * time.Second

	// The input buffer size is the amount of mesages that
	// can be buffered into the channel to receive at once before
	// blocking, and before determinism is broken
	inputBufferSize = 100

	// The output buffer size is the amount of messages that
	// can be buffered into the channel to send at once before
	// blocking, and before determinism is broken.
	outputBufferSize = 100

	// pingInterval = 20 * time.Second //Not implemented in neo clients
)

var (
	errHandShakeTimeout = errors.New("Handshake timed out, peers have " + string(handshakeTimeout) + " Seconds to Complete the handshake")
)

// Peer represents a peer on the neo network
type Peer struct {
	config LocalConfig
	conn   net.Conn

	startHeight uint32

	// atomic vals
	disconnected int32

	//unchangeable state: concurrent safe
	addr      string
	protoVer  protocol.Version
	port      uint16
	inbound   bool
	userAgent string
	services  protocol.ServiceFlag
	createdAt time.Time
	relay     bool

	statemutex     sync.Mutex
	verackReceived bool
	versionKnown   bool

	*stall.Detector

	inch   chan func() // will handle all incoming connections from peer
	outch  chan func() // will handle all outcoming connections from peer
	quitch chan struct{}
}

// NewPeer returns a new NEO peer
func NewPeer(con net.Conn, inbound bool, cfg LocalConfig) *Peer {
	return &Peer{
		inch:        make(chan func(), inputBufferSize),
		outch:       make(chan func(), outputBufferSize),
		quitch:      make(chan struct{}, 1),
		inbound:     inbound,
		config:      cfg,
		conn:        con,
		createdAt:   time.Now(),
		startHeight: 0,
		addr:        con.RemoteAddr().String(),
		Detector:    stall.NewDetector(responseTime, tickerInterval),
	}
}

// Write to a peer
func (p *Peer) Write(msg wire.Messager) error {
	return wire.WriteMessage(p.conn, p.config.Net, msg)
}

// Read to a peer
func (p *Peer) Read() (wire.Messager, error) {
	return wire.ReadMessage(p.conn, p.config.Net)
}

// Disconnect disconnects a peer and closes the connection
func (p *Peer) Disconnect() {

	// return if already disconnected
	if atomic.LoadInt32(&p.disconnected) != 0 {
		return
	}

	atomic.AddInt32(&p.disconnected, 1)

	p.Detector.Quit()
	close(p.quitch)
	p.conn.Close()

	fmt.Println("Disconnected Peer with address", p.RemoteAddr().String())
}

// Port returns the peers port
func (p *Peer) Port() uint16 {
	return p.port
}

// CreatedAt returns the time at which the connection was made
func (p *Peer) CreatedAt() time.Time {
	return p.createdAt
}

// Height returns the latest recorded height of this peer
func (p *Peer) Height() uint32 {
	return p.startHeight
}

// CanRelay returns true, if the peer can relay information
func (p *Peer) CanRelay() bool {
	return p.relay
}

// LocalAddr returns this node's local address
func (p *Peer) LocalAddr() net.Addr {
	return p.conn.LocalAddr()
}

// RemoteAddr returns the remote address of the connected peer
func (p *Peer) RemoteAddr() net.Addr {
	return p.conn.RemoteAddr()
}

// Services returns the services offered by the peer
func (p *Peer) Services() protocol.ServiceFlag {
	return p.config.Services
}

//Inbound returns true whether this peer is an inbound peer
func (p *Peer) Inbound() bool {
	return p.inbound
}

// IsVerackReceived returns true, if this node has
// received a verack from this peer
func (p *Peer) IsVerackReceived() bool {
	return p.verackReceived
}

//NotifyDisconnect returns once the peer has disconnected
// Blocking
func (p *Peer) NotifyDisconnect() {
	fmt.Println("Peer has not disconnected yet")
	<-p.quitch
	fmt.Println("Peer has just disconnected")
}

//End of Exposed API functions//

// PingLoop not impl. in neo yet, adding it now
// will cause this client to disconnect from all other implementations
func (p *Peer) PingLoop() { /*not implemented in other neo clients*/ }

// Run is used to start communicating with the peer
// completes the handshake and starts observing
// for messages coming in
func (p *Peer) Run() error {

	err := p.Handshake()
	if err != nil {
		return err
	}
	go p.StartProtocol()
	go p.ReadLoop()
	go p.WriteLoop()

	//go p.PingLoop() // since it is not implemented. It will disconnect all other impls.
	return nil
}

// StartProtocol run as a go-routine, will act as our queue for messages
// should be ran after handshake
func (p *Peer) StartProtocol() {
loop:
	for atomic.LoadInt32(&p.disconnected) == 0 {
		select {
		case f := <-p.inch:
			f()
		case <-p.quitch:
			break loop
		case <-p.Detector.Quitch:
			fmt.Println("Peer stalled, disconnecting")
			break loop
		}
	}
	p.Disconnect()
}

// ReadLoop Will block on the read until a message is read
// Should only be called after handshake is complete
// on a seperate go-routine.
func (p *Peer) ReadLoop() {

	idleTimer := time.AfterFunc(idleTimeout, func() {
		fmt.Println("Timing out peer")
		p.Disconnect()
	})

loop:
	for atomic.LoadInt32(&p.disconnected) == 0 {

		idleTimer.Reset(idleTimeout) // reset timer on each loop

		readmsg, err := p.Read()

		// Message read; stop Timer
		idleTimer.Stop()

		if err != nil {
			fmt.Println("Err on read", err) // This will also happen if Peer is disconnected
			break loop
		}

		// Remove message as pending from the stall detector
		p.Detector.RemoveMessage(readmsg.Command())

		switch msg := readmsg.(type) {

		case *payload.VersionMessage:
			fmt.Println("Already received a Version, disconnecting. " + p.RemoteAddr().String())
			break loop // We have already done the handshake, break loop and disconnect
		case *payload.VerackMessage:
			if p.verackReceived {
				fmt.Println("Already received a Verack, disconnecting. " + p.RemoteAddr().String())
				break loop
			}
			p.statemutex.Lock() // This should not happen, however if it does, then we should set it.
			p.verackReceived = true
			p.statemutex.Unlock()
		case *payload.AddrMessage:
			p.OnAddr(msg)
		case *payload.GetAddrMessage:
			p.OnGetAddr(msg)
		case *payload.GetBlocksMessage:
			p.OnGetBlocks(msg)
		case *payload.BlockMessage:
			p.OnBlocks(msg)
		case *payload.HeadersMessage:
			p.OnHeaders(msg)
		case *payload.GetHeadersMessage:
			p.OnGetHeaders(msg)
		case *payload.InvMessage:
			p.OnInv(msg)
		case *payload.GetDataMessage:
			p.OnGetData(msg)
		case *payload.TXMessage:
			p.OnTX(msg)
		default:
			fmt.Println("Cannot recognise message", msg.Command()) //Do not disconnect peer, just Log Message
		}
	}

	idleTimer.Stop()
	p.Disconnect()
}

// WriteLoop will Queue all messages to be written to the peer.
func (p *Peer) WriteLoop() {
	for atomic.LoadInt32(&p.disconnected) == 0 {
		select {
		case f := <-p.outch:
			f()
		case <-p.Detector.Quitch: // if the detector quits, disconnect peer
			p.Disconnect()
		}
	}
}

// Outgoing Requests

// RequestHeaders will write a getheaders to this peer
func (p *Peer) RequestHeaders(hash util.Uint256) error {
	c := make(chan error, 0)
	p.outch <- func() {
		getHeaders, err := payload.NewGetHeadersMessage([]util.Uint256{hash}, util.Uint256{})
		err = p.Write(getHeaders)
		if err != nil {
			p.Detector.AddMessage(command.GetHeaders)
		}
		c <- err
	}
	return <-c
}

// RequestBlocks will ask this peer for a set of blocks
func (p *Peer) RequestBlocks(hashes []util.Uint256) error {
	c := make(chan error, 0)

	p.outch <- func() {
		getdata, err := payload.NewGetDataMessage(payload.InvTypeBlock)
		err = getdata.AddHashes(hashes)
		if err != nil {
			c <- err
			return
		}

		err = p.Write(getdata)
		if err != nil {
			p.Detector.AddMessage(command.GetData)
		}

		c <- err
	}
	return <-c
}
