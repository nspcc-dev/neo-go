package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
)

type handShakeStage uint8

const (
	versionSent handShakeStage = 1 << iota
	versionReceived
	verAckSent
	verAckReceived

	requestQueueSize   = 32
	p2pMsgQueueSize    = 16
	hpRequestQueueSize = 4
	incomingQueueSize  = 1 // Each message can be up to 32MB in size.
)

var (
	errGone           = errors.New("the peer is gone already")
	errStateMismatch  = errors.New("tried to send protocol message before handshake completed")
	errPingPong       = errors.New("ping/pong timeout")
	errUnexpectedPong = errors.New("pong message wasn't expected")
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
	// pre-handshake non-canonical connection address.
	addr string

	lock                  sync.RWMutex
	finale                sync.Once
	handShake             handShakeStage
	isFullNode            bool
	isCompressionDisabled bool

	done     chan struct{}
	sendQ    chan []byte
	p2pSendQ chan []byte
	hpSendQ  chan []byte
	incoming chan *Message

	// track outstanding getaddr requests.
	getAddrSent atomic.Int32

	// number of sent pings.
	pingSent  int
	pingTimer *time.Timer
}

// NewTCPPeer returns a TCPPeer structure based on the given connection.
func NewTCPPeer(conn net.Conn, addr string, s *Server) *TCPPeer {
	return &TCPPeer{
		conn:                  conn,
		server:                s,
		addr:                  addr,
		isCompressionDisabled: true, // disabled until peer capabilities are received
		done:                  make(chan struct{}),
		sendQ:                 make(chan []byte, requestQueueSize),
		p2pSendQ:              make(chan []byte, p2pMsgQueueSize),
		hpSendQ:               make(chan []byte, hpRequestQueueSize),
		incoming:              make(chan *Message, incomingQueueSize),
	}
}

// putPacketIntoQueue puts the given message into the given queue if
// the peer has done handshaking using the given context.
func (p *TCPPeer) putPacketIntoQueue(ctx context.Context, queue chan<- []byte, msg []byte) error {
	if !p.Handshaked() {
		return errStateMismatch
	}
	select {
	case queue <- msg:
	case <-p.done:
		return errGone
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

// BroadcastPacket implements the Peer interface.
func (p *TCPPeer) BroadcastPacket(ctx context.Context, msg []byte) error {
	return p.putPacketIntoQueue(ctx, p.sendQ, msg)
}

// BroadcastHPPacket implements the Peer interface. It the peer is not yet
// handshaked it's a noop.
func (p *TCPPeer) BroadcastHPPacket(ctx context.Context, msg []byte) error {
	return p.putPacketIntoQueue(ctx, p.hpSendQ, msg)
}

// putMsgIntoQueue serializes the given Message and puts it into given queue if
// the peer has done handshaking.
func (p *TCPPeer) putMsgIntoQueue(queue chan<- []byte, msg *Message) error {
	b, err := msg.BytesCompressed(p.SupportsCompression())
	if err != nil {
		return err
	}
	return p.putPacketIntoQueue(context.Background(), queue, b)
}

// EnqueueP2PMessage implements the Peer interface.
func (p *TCPPeer) EnqueueP2PMessage(msg *Message) error {
	return p.putMsgIntoQueue(p.p2pSendQ, msg)
}

// EnqueueHPMessage implements the Peer interface.
func (p *TCPPeer) EnqueueHPMessage(msg *Message) error {
	return p.putMsgIntoQueue(p.hpSendQ, msg)
}

// EnqueueP2PPacket implements the Peer interface.
func (p *TCPPeer) EnqueueP2PPacket(b []byte) error {
	return p.putPacketIntoQueue(context.Background(), p.p2pSendQ, b)
}

// EnqueueHPPacket implements the Peer interface.
func (p *TCPPeer) EnqueueHPPacket(b []byte) error {
	return p.putPacketIntoQueue(context.Background(), p.hpSendQ, b)
}

func (p *TCPPeer) writeMsg(msg *Message) error {
	b, err := msg.BytesCompressed(p.supportsCompression())
	if err != nil {
		return err
	}

	_, err = p.conn.Write(b)

	return err
}

// handleConn handles the read side of the connection, it should be started as
// a goroutine right after a new peer setup.
func (p *TCPPeer) handleConn() {
	var err error

	p.server.register <- p

	go p.handleQueues()
	go p.handleIncoming()
	// When a new peer is connected, we send out our version immediately.
	err = p.SendVersion()
	if err == nil {
		r := io.NewBinReaderFromIO(p.conn)
	loop:
		for {
			msg := &Message{}
			err = msg.Decode(r)

			if errors.Is(err, payload.ErrTooManyHeaders) {
				p.server.log.Warn("not all headers were processed")
				r.Err = nil
			} else if err != nil {
				break
			}
			select {
			case p.incoming <- msg:
			case <-p.done:
				break loop
			}
		}
	}
	p.Disconnect(err)
	close(p.incoming)
}

func (p *TCPPeer) handleIncoming() {
	var err error
	for msg := range p.incoming {
		err = p.server.handleMessage(p, msg)
		if err != nil {
			if p.Handshaked() {
				err = fmt.Errorf("handling %s message: %w", msg.Command.String(), err)
			}
			break
		}
	}
	p.Disconnect(err)
}

// handleQueues is a goroutine that is started automatically to handle
// send queues.
func (p *TCPPeer) handleQueues() {
	var err error
	// p2psend queue shares its time with send queue in around
	// ((p2pSkipDivisor - 1) * 2 + 1)/1 ratio, ratio because the third
	// select can still choose p2psend over send.
	var p2pSkipCounter uint32
	const p2pSkipDivisor = 4

	var writeTimeout = max(time.Duration(p.server.chain.GetMillisecondsPerBlock())*time.Millisecond, time.Second)
	for {
		var msg []byte

		// This one is to give priority to the hp queue
		select {
		case <-p.done:
			return
		case msg = <-p.hpSendQ:
		default:
		}

		// Skip this select every p2pSkipDivisor iteration.
		if msg == nil && p2pSkipCounter%p2pSkipDivisor != 0 {
			// Then look at the p2p queue.
			select {
			case <-p.done:
				return
			case msg = <-p.hpSendQ:
			case msg = <-p.p2pSendQ:
			default:
			}
		}
		// If there is no message in HP or P2P queues, block until one
		// appears in any of the queues.
		if msg == nil {
			select {
			case <-p.done:
				return
			case msg = <-p.hpSendQ:
			case msg = <-p.p2pSendQ:
			case msg = <-p.sendQ:
			}
		}
		err = p.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if err != nil {
			break
		}
		_, err = p.conn.Write(msg)
		if err != nil {
			break
		}
		p2pSkipCounter++
	}
	p.Disconnect(err)
drainloop:
	for {
		select {
		case <-p.hpSendQ:
		case <-p.p2pSendQ:
		case <-p.sendQ:
		default:
			break drainloop
		}
	}
}

// StartProtocol starts a long running background loop that interacts
// every ProtoTickInterval with the peer. It's only good to run after the
// handshake.
func (p *TCPPeer) StartProtocol() {
	var err error

	p.server.handshake <- p

	err = p.server.requestBlocksOrHeaders(p)
	if err != nil {
		p.Disconnect(err)
		return
	}

	var ticker = time.NewTicker(p.server.ProtoTickInterval)
	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			// Try to sync in headers and block with the peer if his block height is higher than ours.
			err = p.server.requestBlocksOrHeaders(p)
		}
		if err != nil {
			p.Disconnect(err)
			return
		}
	}
}

// Handshaked returns status of the handshake, whether it's completed or not.
func (p *TCPPeer) Handshaked() bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.handshaked()
}

// handshaked is internal unlocked version of Handshaked().
func (p *TCPPeer) handshaked() bool {
	return p.handShake == (verAckReceived | verAckSent | versionReceived | versionSent)
}

// IsFullNode returns whether the node has full capability or TCP/WS only.
func (p *TCPPeer) IsFullNode() bool {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.handshaked() && p.isFullNode
}

// SupportsCompression returns whether the node supports compressed P2P payloads
// decoding.
func (p *TCPPeer) SupportsCompression() bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.supportsCompression()
}

func (p *TCPPeer) supportsCompression() bool {
	return !p.isCompressionDisabled
}

// SendVersion checks for the handshake state and sends a message to the peer.
func (p *TCPPeer) SendVersion() error {
	msg, err := p.server.getVersionMsg(p.conn.LocalAddr())
	if err != nil {
		return err
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.handShake&versionSent != 0 {
		return errors.New("invalid handshake: already sent Version")
	}
	err = p.writeMsg(msg)
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
	p.isCompressionDisabled = false
	for _, cap := range version.Capabilities {
		switch cap.Type {
		case capability.FullNode:
			p.isFullNode = true
			p.lastBlockIndex = cap.Data.(*capability.Node).StartHeight
		case capability.DisableCompressionNode:
			p.isCompressionDisabled = true
		default:
			continue
		}
	}

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

// ConnectionAddr implements the Peer interface.
func (p *TCPPeer) ConnectionAddr() string {
	if p.addr != "" {
		return p.addr
	}
	return p.conn.RemoteAddr().String()
}

// RemoteAddr implements the Peer interface.
func (p *TCPPeer) RemoteAddr() net.Addr {
	return p.conn.RemoteAddr()
}

// PeerAddr implements the Peer interface.
func (p *TCPPeer) PeerAddr() net.Addr {
	remote := p.conn.RemoteAddr()
	// The network can be non-tcp in unit tests.
	if p.version == nil || remote.Network() != "tcp" {
		return p.RemoteAddr()
	}
	host, _, err := net.SplitHostPort(remote.String())
	if err != nil {
		return p.RemoteAddr()
	}
	var port uint16
	for _, cap := range p.version.Capabilities {
		if cap.Type == capability.TCPServer {
			port = cap.Data.(*capability.Server).Port
		}
	}
	if port == 0 {
		return p.RemoteAddr()
	}
	addrString := net.JoinHostPort(host, strconv.Itoa(int(port)))
	tcpAddr, err := net.ResolveTCPAddr("tcp", addrString)
	if err != nil {
		return p.RemoteAddr()
	}
	return tcpAddr
}

// Disconnect will fill the peer's done channel with the given error.
func (p *TCPPeer) Disconnect(err error) {
	p.finale.Do(func() {
		close(p.done)
		p.conn.Close()
		p.server.unregister <- peerDrop{p, err}
	})
}

// Version implements the Peer interface.
func (p *TCPPeer) Version() *payload.Version {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.version
}

// LastBlockIndex returns the last block index.
func (p *TCPPeer) LastBlockIndex() uint32 {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.lastBlockIndex
}

// SetPingTimer adds an outgoing ping to the counter and sets a PingTimeout timer
// that will shut the connection down in case of no response.
func (p *TCPPeer) SetPingTimer() {
	p.lock.Lock()
	p.pingSent++
	if p.pingTimer == nil {
		p.pingTimer = time.AfterFunc(p.server.PingTimeout, func() {
			p.Disconnect(errPingPong)
		})
	}
	p.lock.Unlock()
}

// HandlePing handles a ping message received from the peer.
func (p *TCPPeer) HandlePing(ping *payload.Ping) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.lastBlockIndex = ping.LastBlockIndex
	return nil
}

// HandlePong handles a pong message received from the peer and does an appropriate
// accounting of outstanding pings and timeouts.
func (p *TCPPeer) HandlePong(pong *payload.Ping) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.pingTimer != nil && !p.pingTimer.Stop() {
		return errPingPong
	}
	p.pingTimer = nil
	p.pingSent--
	if p.pingSent < 0 {
		return errUnexpectedPong
	}
	p.lastBlockIndex = pong.LastBlockIndex
	return nil
}

// AddGetAddrSent increments internal outstanding getaddr requests counter. Then,
// the peer can only send one addr reply per getaddr request.
func (p *TCPPeer) AddGetAddrSent() {
	p.getAddrSent.Add(1)
}

// CanProcessAddr decrements internal outstanding getaddr requests counter and
// answers whether the addr command from the peer can be safely processed.
func (p *TCPPeer) CanProcessAddr() bool {
	v := p.getAddrSent.Add(-1)
	return v >= 0
}
