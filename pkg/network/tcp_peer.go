package network

import (
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"go.uber.org/zap"
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
	// The server this peer belongs to.
	server *Server
	// The version of the peer.
	version *payload.Version
	// Index of the last block.
	lastBlockIndex uint32

	lock      sync.RWMutex
	handShake handShakeStage

	done chan error

	wg sync.WaitGroup

	// number of sent pings.
	pingSent int
}

// NewTCPPeer returns a TCPPeer structure based on the given connection.
func NewTCPPeer(conn net.Conn, s *Server) *TCPPeer {
	return &TCPPeer{
		conn:   conn,
		server: s,
		done:   make(chan error, 1),
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
		w := io.NewBufBinWriter()
		if err := msg.Encode(w.BinWriter); err != nil {
			return err
		}

		_, err := p.conn.Write(w.Bytes())

		return err
	}
}

// handleConn handles the read side of the connection, it should be started as
// a goroutine right after the new peer setup.
func (p *TCPPeer) handleConn() {
	var err error

	p.server.register <- p

	// When a new peer is connected we send out our version immediately.
	err = p.server.sendVersion(p)
	if err == nil {
		r := io.NewBinReaderFromIO(p.conn)
		for {
			msg := &Message{}
			err = msg.Decode(r)

			if err == payload.ErrTooManyHeaders {
				p.server.log.Warn("not all headers were processed")
				r.Err = nil
			} else if err != nil {
				break
			}
			if err = p.server.handleMessage(p, msg); err != nil {
				break
			}
		}
	}
	p.Disconnect(err)
}

// StartProtocol starts a long running background loop that interacts
// every ProtoTickInterval with the peer. It's only good to run after the
// handshake.
func (p *TCPPeer) StartProtocol() {
	var err error

	p.server.log.Info("started protocol",
		zap.Stringer("addr", p.RemoteAddr()),
		zap.ByteString("userAgent", p.Version().UserAgent),
		zap.Uint32("startHeight", p.Version().StartHeight),
		zap.Uint32("id", p.Version().Nonce))

	p.server.discovery.RegisterGoodAddr(p.PeerAddr().String())
	if p.server.chain.HeaderHeight() < p.LastBlockIndex() {
		err = p.server.requestHeaders(p)
		if err != nil {
			p.Disconnect(err)
			return
		}
	}

	timer := time.NewTimer(p.server.ProtoTickInterval)
	pingTimer := time.NewTimer(p.server.PingTimeout)
	for {
		select {
		case err = <-p.Done():
			// time to stop
		case m := <-p.server.addrReq:
			err = p.WriteMsg(m)
		case <-timer.C:
			// Try to sync in headers and block with the peer if his block height is higher then ours.
			if p.LastBlockIndex() > p.server.chain.BlockHeight() {
				err = p.server.requestBlocks(p)
			}
			if err == nil {
				timer.Reset(p.server.ProtoTickInterval)
			}
			if p.server.chain.HeaderHeight() >= p.LastBlockIndex() {
				block, errGetBlock := p.server.chain.GetBlock(p.server.chain.CurrentBlockHash())
				if errGetBlock != nil {
					err = errGetBlock
				} else {
					diff := uint32(time.Now().UTC().Unix()) - block.Timestamp
					if diff > uint32(p.server.PingInterval/time.Second) {
						p.UpdatePingSent(p.GetPingSent() + 1)
						err = p.WriteMsg(NewMessage(p.server.Net, CMDPing, payload.NewPing(p.server.id, p.server.chain.HeaderHeight())))
					}
				}
			}
		case <-pingTimer.C:
			if p.GetPingSent() > defaultPingLimit {
				err = errors.New("ping/pong timeout")
			} else {
				pingTimer.Reset(p.server.PingTimeout)
				p.UpdatePingSent(0)
			}
		}
		if err != nil {
			timer.Stop()
			p.Disconnect(err)
			return
		}
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
	p.lastBlockIndex = version.StartHeight
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
	if p.handShake&versionSent == 0 {
		return errors.New("invalid handshake: tried to send VersionAck, but didn't send Version yet")
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
	if p.handShake&versionReceived == 0 {
		return errors.New("invalid handshake: received VersionAck, but no version received yet")
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
	p.server.unregister <- peerDrop{p, err}
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

// LastBlockIndex returns last block index.
func (p *TCPPeer) LastBlockIndex() uint32 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.lastBlockIndex
}

// UpdateLastBlockIndex updates last block index.
func (p *TCPPeer) UpdateLastBlockIndex(newIndex uint32) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.lastBlockIndex = newIndex
}

// GetPingSent returns flag whether ping was sent or not.
func (p *TCPPeer) GetPingSent() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.pingSent
}

// UpdatePingSent updates pingSent value.
func (p *TCPPeer) UpdatePingSent(newValue int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.pingSent = newValue
}
