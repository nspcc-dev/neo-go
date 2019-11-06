package network

import (
	"errors"
	"net"
	"strconv"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

type handShakeStage uint8

const (
	versionSent handShakeStage = 1 << iota
	versionReceived
	verAckSent
	verAckReceived
)

var (
	errStateMismatch = errors.New("tried to send protocol message before handshake completed")
)

// TCPPeer represents a connected remote node in the
// network over TCP.
type TCPPeer struct {
	// underlying TCP connection.
	conn net.Conn

	// The version of the peer.
	version *payload.Version

	lock      sync.RWMutex
	handShake handShakeStage

	done chan error

	wg sync.WaitGroup
}

// NewTCPPeer returns a TCPPeer structure based on the given connection.
func NewTCPPeer(conn net.Conn) *TCPPeer {
	return &TCPPeer{
		conn: conn,
		done: make(chan error, 1),
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
		w := io.NewBinWriterFromIO(p.conn)
		return msg.Encode(w)
	}
}

// Handshaked returns status of the handshake, whether it's completed or not.
func (p *TCPPeer) Handshaked() bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.handShake == (verAckReceived | verAckSent | versionReceived | versionSent)
}

// SendVersion checks for the handshake state and sends a message to the peer.
func (p *TCPPeer) SendVersion(msg *Message) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.handShake&versionSent != 0 {
		return errors.New("invalid handshake: already sent Version")
	}
	err := p.writeMsg(msg)
	if err == nil {
		p.handShake |= versionSent
	}
	return err
}

// HandleVersion checks for the handshake state and version message contents.
func (p *TCPPeer) HandleVersion(version *payload.Version) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.handShake&versionReceived != 0 {
		return errors.New("invalid handshake: already received Version")
	}
	p.version = version
	p.handShake |= versionReceived
	return nil
}

// SendVersionAck checks for the handshake state and sends a message to the peer.
func (p *TCPPeer) SendVersionAck(msg *Message) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.handShake&versionReceived == 0 {
		return errors.New("invalid handshake: tried to send VersionAck, but no version received yet")
	}
	if p.handShake&verAckSent != 0 {
		return errors.New("invalid handshake: already sent VersionAck")
	}
	err := p.writeMsg(msg)
	if err == nil {
		p.handShake |= verAckSent
	}
	return err
}

// HandleVersionAck checks handshake sequence correctness when VerAck message
// is received.
func (p *TCPPeer) HandleVersionAck() error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.handShake&versionSent == 0 {
		return errors.New("invalid handshake: received VersionAck, but no version sent yet")
	}
	if p.handShake&verAckReceived != 0 {
		return errors.New("invalid handshake: already received VersionAck")
	}
	p.handShake |= verAckReceived
	return nil
}

// RemoteAddr implements the Peer interface.
func (p *TCPPeer) RemoteAddr() net.Addr {
	return p.conn.RemoteAddr()
}

// PeerAddr implements the Peer interface.
func (p *TCPPeer) PeerAddr() net.Addr {
	remote := p.conn.RemoteAddr()
	// The network can be non-tcp in unit tests.
	if !p.Handshaked() || remote.Network() != "tcp" {
		return p.RemoteAddr()
	}
	host, _, err := net.SplitHostPort(remote.String())
	if err != nil {
		return p.RemoteAddr()
	}
	addrString := net.JoinHostPort(host, strconv.Itoa(int(p.version.Port)))
	tcpAddr, err := net.ResolveTCPAddr("tcp", addrString)
	if err != nil {
		return p.RemoteAddr()
	}
	return tcpAddr
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
	select {
	case p.done <- err:
		// one message to the queue
	default:
		// the other side may already be gone, it's OK
	}
}

// Version implements the Peer interface.
func (p *TCPPeer) Version() *payload.Version {
	return p.version
}
