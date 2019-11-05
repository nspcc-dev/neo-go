package network

import (
	"net"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendVersion(t *testing.T) {
	var (
		s = newTestServer()
		p = newLocalPeer(t)
	)
	s.Port = 3000
	s.UserAgent = "/test/"

	p.messageHandler = func(t *testing.T, msg *Message) {
		assert.Equal(t, CMDVersion, msg.CommandType())
		assert.IsType(t, msg.Payload, &payload.Version{})
		version := msg.Payload.(*payload.Version)
		assert.NotZero(t, version.Nonce)
		assert.Equal(t, uint16(3000), version.Port)
		assert.Equal(t, uint64(1), version.Services)
		assert.Equal(t, uint32(0), version.Version)
		assert.Equal(t, []byte("/test/"), version.UserAgent)
		assert.Equal(t, uint32(0), version.StartHeight)
	}

	if err := s.sendVersion(p); err != nil {
		t.Fatal(err)
	}
}

// Server should reply with a verack after receiving a valid version.
func TestVerackAfterHandleVersionCmd(t *testing.T) {
	var (
		s = newTestServer()
		p = newLocalPeer(t)
	)
	na, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:3000")
	p.netaddr = *na

	// Should have a verack
	p.messageHandler = func(t *testing.T, msg *Message) {
		assert.Equal(t, CMDVerack, msg.CommandType())
	}
	version := payload.NewVersion(1337, 3000, "/NEO-GO/", 0, true)

	if err := s.handleVersionCmd(p, version); err != nil {
		t.Fatal(err)
	}
}

// Server should not reply with a verack after receiving a
// invalid version and disconnects the peer.
func TestServerNotSendsVerack(t *testing.T) {
	var (
		s  = newTestServer()
		p  = newLocalPeer(t)
		p2 = newLocalPeer(t)
	)
	s.id = 1
	go s.run()

	na, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:3000")
	p.netaddr = *na
	p2.netaddr = *na
	s.register <- p

	// identical id's
	version := payload.NewVersion(1, 3000, "/NEO-GO/", 0, true)
	err := s.handleVersionCmd(p, version)
	assert.NotNil(t, err)
	assert.Equal(t, errIdenticalID, err)

	// Different IDs, make handshake pass.
	version.Nonce = 2
	require.NoError(t, s.handleVersionCmd(p, version))
	require.NoError(t, p.HandleVersionAck())
	require.Equal(t, true, p.Handshaked())

	// Second handshake from the same peer should fail.
	s.register <- p2
	err = s.handleVersionCmd(p2, version)
	assert.NotNil(t, err)
	require.Equal(t, errAlreadyConnected, err)
}

func TestRequestHeaders(t *testing.T) {
	var (
		s = newTestServer()
		p = newLocalPeer(t)
	)
	p.messageHandler = func(t *testing.T, msg *Message) {
		assert.IsType(t, &payload.GetBlocks{}, msg.Payload)
		assert.Equal(t, CMDGetHeaders, msg.CommandType())
	}
	s.requestHeaders(p)
}
