package network

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

type handShakeStage uint8

//go:generate stringer -type=handShakeStage
const (
	nothingDone     handShakeStage = 0
	versionSent     handShakeStage = 1
	versionReceived handShakeStage = 2
	verAckSent      handShakeStage = 3
	verAckReceived  handShakeStage = 4
)

var (
	errStateMismatch = errors.New("tried to send protocol message before handshake completed")
)

// TCPPeer represents a connected remote node in the
// network over TCP.
type TCPPeer struct {
	// underlying TCP connection.
	conn net.Conn
	addr net.TCPAddr

	// The version of the peer.
	version *payload.Version

	handShake handShakeStage

	done chan error

	wg sync.WaitGroup
}

// NewTCPPeer returns a TCPPeer structure based on the given connection.
func NewTCPPeer(conn net.Conn) *TCPPeer {
	raddr := conn.RemoteAddr()
	// can't fail because raddr is a real connection
	tcpaddr, _ := net.ResolveTCPAddr(raddr.Network(), raddr.String())
	return &TCPPeer{
		conn: conn,
		done: make(chan error, 1),
		addr: *tcpaddr,
	}
}

// WriteMsg implements the Peer interface. This will write/encode the message
// to the underlying connection, this only works for messages other than Version
// or VerAck.
func (p *TCPPeer) WriteMsg(msg *Message) error {
	if !p.Handshaked() {
		return errStateMismatch
	}
	return p.writeMsg(msg)
}

func (p *TCPPeer) writeMsg(msg *Message) error {
	select {
	case err := <-p.done:
		return err
	default:
		return msg.Encode(p.conn)
	}
}

// Handshaked returns status of the handshake, whether it's completed or not.
func (p *TCPPeer) Handshaked() bool {
	return p.handShake == verAckReceived
}

// SendVersion checks for the handshake state and sends a message to the peer.
func (p *TCPPeer) SendVersion(msg *Message) error {
	if p.handShake != nothingDone {
		return fmt.Errorf("invalid handshake: tried to send Version in %s state", p.handShake.String())
	}
	err := p.writeMsg(msg)
	if err == nil {
		p.handShake = versionSent
	}
	return err
}

// HandleVersion checks for the handshake state and version message contents.
func (p *TCPPeer) HandleVersion(version *payload.Version) error {
	if p.handShake != versionSent {
		return fmt.Errorf("invalid handshake: received Version in %s state", p.handShake.String())
	}
	p.version = version
	p.handShake = versionReceived
	return nil
}

// SendVersionAck checks for the handshake state and sends a message to the peer.
func (p *TCPPeer) SendVersionAck(msg *Message) error {
	if p.handShake != versionReceived {
		return fmt.Errorf("invalid handshake: tried to send VersionAck in %s state", p.handShake.String())
	}
	err := p.writeMsg(msg)
	if err == nil {
		p.handShake = verAckSent
	}
	return err
}

// HandleVersionAck checks handshake sequence correctness when VerAck message
// is received.
func (p *TCPPeer) HandleVersionAck() error {
	if p.handShake != verAckSent {
		return fmt.Errorf("invalid handshake: received VersionAck in %s state", p.handShake.String())
	}
	p.handShake = verAckReceived
	return nil
}

// NetAddr implements the Peer interface.
func (p *TCPPeer) NetAddr() *net.TCPAddr {
	return &p.addr
}

// Done implements the Peer interface and notifies
// all other resources operating on it that this peer
// is no longer running.
func (p *TCPPeer) Done() chan error {
	return p.done
}

// Disconnect will fill the peer's done channel with the given error.
func (p *TCPPeer) Disconnect(err error) {
	p.conn.Close()
	p.done <- err
}

// Version implements the Peer interface.
func (p *TCPPeer) Version() *payload.Version {
	return p.version
}
