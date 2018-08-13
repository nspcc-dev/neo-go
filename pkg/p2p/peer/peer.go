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
	"time"

	"github.com/CityOfZion/neo-go/pkg/p2p/peer/stall"
	"github.com/CityOfZion/neo-go/pkg/wire"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

const (
	maxOutboundConnections = 100
	protocolVer            = protocol.DefaultVersion
	handshakeTimeout       = 30 * time.Second
	idleTimeout            = 5 * time.Minute

	// pingInterval = 20 * time.Second //Not implemented in neo clients
)

var (
	errHandShakeTimeout = errors.New("Handshake timed out, peers have " + string(handshakeTimeout) + " Seconds to Complete the handshake")
)

type Peer struct {
	config LocalConfig
	conn   net.Conn

	//unchangeable state: concurrent safe
	addr      string
	port      uint16
	inbound   bool
	net       protocol.Magic
	userAgent string
	services  protocol.ServiceFlag

	statemutex     sync.Mutex
	verackReceived bool

	stall.Detector

	inch   chan func() // will handle all incoming connections from peer
	outch  chan func() // will handle all outcoming connections from peer
	quitch chan struct{}
}

func NewPeer(con net.Conn, in bool, cfg LocalConfig) Peer {
	p := Peer{}
	p.inch = make(chan func(), 10)
	p.outch = make(chan func(), 10)
	p.quitch = make(chan struct{}, 1)
	p.inbound = in
	p.config = cfg
	p.conn = con

	// TODO: set the unchangeable states
	return p
}

// Write to a peer
func (p *Peer) Write(msg wire.Messager) error {
	if err := wire.WriteMessage(p.conn, p.net, msg); err != nil {
		return err
	}
	return nil
}

// Read to a peer
func (p *Peer) Read() (wire.Messager, error) {
	msg, err := wire.ReadMessage(p.conn, p.net)
	return msg, err
}

// Disconnects from a peer
func (p *Peer) Disconnect() {
	p.conn.Close()

	// Close all other channels
	// Should not wait, just close and move on
	fmt.Println("Disconnecting peer")
}

// Exposed API functions below
func (p *Peer) LocalAddr() net.Addr {
	return p.LocalAddr()
}
func (p *Peer) RemoteAddr() net.Addr {
	return p.RemoteAddr()
}
func (p *Peer) Services() protocol.ServiceFlag {
	return p.config.services
}
func (p *Peer) Inbound() bool {
	return p.inbound
}
func (p *Peer) UserAgent() string {
	return p.config.userAgent
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

	// This will be refactored to allow more control over the go-routine
	go p.StartProtocol()
	go p.ReadLoop()

	//go p.PingLoop() // since it is not implemented. It will disconnect all other impls.
	return err

}

// run as a go-routine, will act as our queue for messages
// should be ran after handshake
func (p *Peer) StartProtocol() {
	for {
		select {
		case f := <-p.inch:
			f()
		case <-p.quitch:
			p.Disconnect()
		case <-p.Detector.Quitch:
			p.Disconnect()
		}
	}
}

// Should only be called after handshake is complete
// on a seperate go-routine.
// ReadLoop Will block on the read until a message is
// read

func (p *Peer) ReadLoop() {
loop:
	for {

		readmsg, err := p.Read()

		if err != nil {
			break loop
		}

		switch msg := readmsg.(type) {
		case *payload.VersionMessage:
			break loop // We have already done the handshake, break loop and disconnect
		case *payload.VerackMessage:
			if p.verackReceived {
				break loop
			}
			p.OnVerack()
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
			p.OnGetHeaders(msg) // TODO:Will change to interface once, I test this class
		default:
			fmt.Println("Cannot recognise message", msg.Command())
		}
	}
	// cleanup: disconnect peer and then close channel. Disconnecting first will stop the flow
	// of messages, then closing will drain all channels.
	p.Disconnect()
	close(p.quitch)
}

//
func (p *Peer) WriteLoop() {
	for {
		select {
		case f := <-p.outch:
			f()
		case <-p.Detector.Quitch:
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
		p.config.AddressMessageListener.OnAddr(msg)
		fmt.Println("That was a addr message, please pass func down through config", msg.Command())

	}
}

// OnGetAddr Listener
func (p *Peer) OnGetAddr(msg *payload.GetAddrMessage) {
	p.inch <- func() {
		p.config.AddressMessageListener.OnGetAddr(msg)
		fmt.Println("That was a getaddr message, please pass func down through config", msg.Command())

	}
}

// OnGetBlocks Listener
func (p *Peer) OnGetBlocks(msg *payload.GetBlocksMessage) {
	p.inch <- func() {
		p.config.BlockMessageListener.OnGetBlocks(msg)
		fmt.Println("That was a getblocks message, please pass func down through config", msg.Command())

	}
}

// OnBlocks Listener
func (p *Peer) OnBlocks(msg *payload.BlockMessage) {
	p.inch <- func() {
		p.config.BlockMessageListener.OnBlock(msg)
		fmt.Println("That was a blocks message, please pass func down through config", msg.Command())

	}
}

// OnHeaders Listener
func (p *Peer) OnHeaders(msg *payload.HeadersMessage) {
	p.inch <- func() {
		p.config.HeadersMessageListener.OnHeader(msg)
		fmt.Println("That was a headers message, please pass func down through config", msg.Command())
	}
}

// Since this is ran after handshake, if a verack has already been received
// then this would violate the rules and hence be cause for a disconnect. Not a ban
func (p *Peer) OnVerack() {
	p.statemutex.Lock()
	p.verackReceived = true
	p.statemutex.Unlock()
	p.inch <- func() {
		// No need to process it, we do nothing on verack unless we have received it before
		// If so, this will never run, as the loop would have been broken.
		// We do not have a verack method in config, as not needed.
	}
}
