package network

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/fakechain"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type testDiscovery struct {
	sync.Mutex
	bad          []string
	connected    []string
	unregistered []string
	backfill     []string
}

func newTestDiscovery([]string, time.Duration, Transporter) Discoverer { return new(testDiscovery) }

func (d *testDiscovery) BackFill(addrs ...string) {
	d.Lock()
	defer d.Unlock()
	d.backfill = append(d.backfill, addrs...)
}
func (d *testDiscovery) Close()         {}
func (d *testDiscovery) PoolCount() int { return 0 }
func (d *testDiscovery) RegisterBadAddr(addr string) {
	d.Lock()
	defer d.Unlock()
	d.bad = append(d.bad, addr)
}
func (d *testDiscovery) RegisterGoodAddr(string, capability.Capabilities) {}
func (d *testDiscovery) RegisterConnectedAddr(addr string) {
	d.Lock()
	defer d.Unlock()
	d.connected = append(d.connected, addr)
}
func (d *testDiscovery) UnregisterConnectedAddr(addr string) {
	d.Lock()
	defer d.Unlock()
	d.unregistered = append(d.unregistered, addr)
}
func (d *testDiscovery) UnconnectedPeers() []string {
	d.Lock()
	defer d.Unlock()
	return d.unregistered
}
func (d *testDiscovery) RequestRemote(n int) {}
func (d *testDiscovery) BadPeers() []string {
	d.Lock()
	defer d.Unlock()
	return d.bad
}
func (d *testDiscovery) GoodPeers() []AddressWithCapabilities { return []AddressWithCapabilities{} }

var defaultMessageHandler = func(t *testing.T, msg *Message) {}

type localPeer struct {
	netaddr        net.TCPAddr
	server         *Server
	version        *payload.Version
	lastBlockIndex uint32
	handshaked     bool
	isFullNode     bool
	t              *testing.T
	messageHandler func(t *testing.T, msg *Message)
	pingSent       int
	getAddrSent    int
	droppedWith    atomic.Value
}

func newLocalPeer(t *testing.T, s *Server) *localPeer {
	naddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	return &localPeer{
		t:              t,
		server:         s,
		netaddr:        *naddr,
		messageHandler: defaultMessageHandler,
	}
}

func (p *localPeer) RemoteAddr() net.Addr {
	return &p.netaddr
}
func (p *localPeer) PeerAddr() net.Addr {
	return &p.netaddr
}
func (p *localPeer) StartProtocol() {}
func (p *localPeer) Disconnect(err error) {
	if p.droppedWith.Load() == nil {
		p.droppedWith.Store(err)
	}
	fmt.Println("peer dropped:", err)
	p.server.unregister <- peerDrop{p, err}
}

func (p *localPeer) EnqueueMessage(msg *Message) error {
	b, err := msg.Bytes()
	if err != nil {
		return err
	}
	return p.EnqueuePacket(true, b)
}
func (p *localPeer) EnqueuePacket(block bool, m []byte) error {
	return p.EnqueueHPPacket(block, m)
}
func (p *localPeer) EnqueueP2PMessage(msg *Message) error {
	return p.EnqueueMessage(msg)
}
func (p *localPeer) EnqueueP2PPacket(m []byte) error {
	return p.EnqueueHPPacket(true, m)
}
func (p *localPeer) EnqueueHPPacket(_ bool, m []byte) error {
	msg := &Message{}
	r := io.NewBinReaderFromBuf(m)
	err := msg.Decode(r)
	if err == nil {
		p.messageHandler(p.t, msg)
	}
	return nil
}
func (p *localPeer) Version() *payload.Version {
	return p.version
}
func (p *localPeer) LastBlockIndex() uint32 {
	return p.lastBlockIndex
}
func (p *localPeer) HandleVersion(v *payload.Version) error {
	p.version = v
	return nil
}
func (p *localPeer) SendVersion() error {
	m, err := p.server.getVersionMsg()
	if err != nil {
		return err
	}
	_ = p.EnqueueMessage(m)
	return nil
}
func (p *localPeer) SendVersionAck(m *Message) error {
	_ = p.EnqueueMessage(m)
	return nil
}
func (p *localPeer) HandleVersionAck() error {
	p.handshaked = true
	return nil
}
func (p *localPeer) SendPing(m *Message) error {
	p.pingSent++
	_ = p.EnqueueMessage(m)
	return nil
}
func (p *localPeer) HandlePing(ping *payload.Ping) error {
	p.lastBlockIndex = ping.LastBlockIndex
	return nil
}

func (p *localPeer) HandlePong(pong *payload.Ping) error {
	p.lastBlockIndex = pong.LastBlockIndex
	p.pingSent--
	return nil
}

func (p *localPeer) Handshaked() bool {
	return p.handshaked
}

func (p *localPeer) IsFullNode() bool {
	return p.isFullNode
}

func (p *localPeer) AddGetAddrSent() {
	p.getAddrSent++
}
func (p *localPeer) CanProcessAddr() bool {
	p.getAddrSent--
	return p.getAddrSent >= 0
}

func newTestServer(t *testing.T, serverConfig ServerConfig) *Server {
	return newTestServerWithChain(t, serverConfig, fakechain.NewFakeChain())
}

func newTestServerWithChain(t *testing.T, serverConfig ServerConfig, chain blockchainer.Blockchainer) *Server {
	s, err := newServerFromConstructors(serverConfig, chain, zaptest.NewLogger(t),
		newFakeTransp, newFakeConsensus, newTestDiscovery)
	require.NoError(t, err)
	t.Cleanup(s.discovery.Close)
	return s
}
