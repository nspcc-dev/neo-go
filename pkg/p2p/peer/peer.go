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

	"github.com/CityOfZion/neo-go/pkg/p2p/peer/stall"
	"github.com/CityOfZion/neo-go/pkg/wire"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

const (
	maxOutboundConnections = 100
	protocolVer            = protocol.DefaultVersion
	handshakeTimeout       = 30 * time.Second
	idleTimeout            = 5 * time.Minute

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

type Peer struct {
	config LocalConfig
	conn   net.Conn

	// atomic vals
	disconnected int32

	//unchangeable state: concurrent safe
	addr      string
	protoVer  protocol.Version
	port      uint16
	inbound   bool
	userAgent string
	services  protocol.ServiceFlag

	statemutex     sync.Mutex
	verackReceived bool
	versionKnown   bool

	*stall.Detector

	inch   chan func() // will handle all incoming connections from peer
	outch  chan func() // will handle all outcoming connections from peer
	quitch chan struct{}
}

func NewPeer(con net.Conn, inbound bool, cfg LocalConfig) Peer {
	p := Peer{}
	p.inch = make(chan func(), inputBufferSize)
	p.outch = make(chan func(), outputBufferSize)
	p.quitch = make(chan struct{}, 1)
	p.inbound = inbound
	p.config = cfg
	p.conn = con
	p.addr = p.conn.RemoteAddr().String()

	p.Detector = stall.NewDetector(responseTime, tickerInterval)

	// TODO: set the unchangeable states
	return p
}

// Write to a peer
func (p *Peer) Write(msg wire.Messager) error {
	return wire.WriteMessage(p.conn, p.config.Net, msg)
}

// Read to a peer
func (p *Peer) Read() (wire.Messager, error) {
	return wire.ReadMessage(p.conn, p.config.Net)
}

// Disconnects from a peer
func (p *Peer) Disconnect() {

	// Already disconnecting
	if atomic.LoadInt32(&p.disconnected) != 0 {
		return
	}
	fmt.Println("Disconnecting Peer")
	atomic.AddInt32(&p.disconnected, 1)
	p.conn.Close()
}

// Exposed API functions below
func (p *Peer) LocalAddr() net.Addr {
	return p.conn.LocalAddr()
}
func (p *Peer) RemoteAddr() net.Addr {
	return p.conn.RemoteAddr()
}
func (p *Peer) Services() protocol.ServiceFlag {
	return p.config.Services
}
func (p *Peer) Inbound() bool {
	return p.inbound
}
func (p *Peer) UserAgent() string {
	return p.config.UserAgent
}
func (p *Peer) IsVerackReceived() bool {
	return p.verackReceived
}

//End of Exposed API functions//

// Ping not impl. in neo yet, adding it now
// will cause this client to disconnect from all other implementations
func (p *Peer) PingLoop() { /*not implemented in other neo clients*/ }

// Run is used to start communicating with the peer
// completes the handshake and starts observing
// for messages coming in
func (p *Peer) Run() error {

	err := p.Handshake()

	go p.StartProtocol()
	go p.ReadLoop()
	go p.WriteLoop()

	//go p.PingLoop() // since it is not implemented. It will disconnect all other impls.
	return err

}

// run as a go-routine, will act as our queue for messages
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

// Should only be called after handshake is complete
// on a seperate go-routine.
// ReadLoop Will block on the read until a message is
// read

func (p *Peer) ReadLoop() {
loop:
	for atomic.LoadInt32(&p.disconnected) == 0 {
		readmsg, err := p.Read()

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
		default:
			fmt.Println("Cannot recognise message", msg.Command()) //Do not disconnect peer, just Log Message
		}
	}
	p.Disconnect()
}

// WriteLoop will Queue all messages to be written to
// the peer.
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

// OnGetHeaders Listener, outside of the anonymous func will be extra functionality
// like timing
func (p *Peer) OnGetHeaders(msg *payload.GetHeadersMessage) {
	p.inch <- func() {

		fmt.Println("That was a getheaders message, please pass func down through config", msg.Command())

	}
}

// OnAddr Listener
func (p *Peer) OnAddr(msg *payload.AddrMessage) {
	p.inch <- func() {
		p.config.OnAddr(msg)
		fmt.Println("That was a addr message, please pass func down through config", msg.Command())

	}
}

// OnGetAddr Listener
func (p *Peer) OnGetAddr(msg *payload.GetAddrMessage) {
	p.inch <- func() {
		p.config.OnGetAddr(msg)
		fmt.Println("That was a getaddr message, please pass func down through config", msg.Command())

	}
}

// OnGetBlocks Listener
func (p *Peer) OnGetBlocks(msg *payload.GetBlocksMessage) {
	p.inch <- func() {
		p.config.OnGetBlocks(msg)
		fmt.Println("That was a getblocks message, please pass func down through config", msg.Command())

	}
}

// OnBlocks Listener
func (p *Peer) OnBlocks(msg *payload.BlockMessage) {

	p.inch <- func() {
		p.config.OnBlock(msg)
		fmt.Println("That was a blocks message, please pass func down through config", msg.Command())

	}
}

// OnVersion Listener will be called
// during the handshake, any error checking should be done here for the versionMessage.
// This should only ever be called during the handshake. Any other place and the peer will disconnect.
func (p *Peer) OnVersion(msg *payload.VersionMessage) error {
	p.versionKnown = true
	p.port = msg.Port
	p.services = msg.Services
	p.userAgent = string(msg.UserAgent)
	return nil
}

// OnHeaders Listener
func (p *Peer) OnHeaders(msg *payload.HeadersMessage) {

	p.inch <- func() {
		if p.config.OnHeader != nil {
			p.config.OnHeader(p, msg)
		}
		//		p.config.HeadersMessageListener.OnHeader(msg)
		// fmt.Println("That was a headers message, please pass func down through config", msg.Command())
	}
}

// RequestHeaders will write a getheaders to peer
func (p *Peer) RequestHeaders(hash util.Uint256) error {
	c := make(chan error, 0)
	p.outch <- func() {
		p.Detector.AddMessage(command.GetHeaders)
		getHeaders, err := payload.NewGetHeadersMessage([]util.Uint256{hash.Reverse()}, util.Uint256{})
		err = p.Write(getHeaders)
		// return err
		c <- err
	}
	return <-c

}
