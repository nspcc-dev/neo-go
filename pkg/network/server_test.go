package network

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestSendVersion(t *testing.T) {
	var (
		s = newTestServer()
		p = newLocalPeer(t)
	)
	s.ListenTCP = 3000
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

	s.sendVersion(p)
}

// Server should reply with a verack after receiving a valid version.
func TestVerackAfterHandleVersionCmd(t *testing.T) {
	var (
		s = newTestServer()
		p = newLocalPeer(t)
	)
	p.endpoint = util.NewEndpoint("0.0.0.0:3000")

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
		s = newTestServer()
		p = newLocalPeer(t)
	)
	s.id = 1
	go s.run()

	p.endpoint = util.NewEndpoint("0.0.0.0:3000")
	s.register <- p

	// Port should mismatch
	version := payload.NewVersion(1337, 2000, "/NEO-GO/", 0, true)
	err := s.handleVersionCmd(p, version)
	assert.NotNil(t, err)
	assert.Equal(t, errPortMismatch, err)

	// identical id's
	version = payload.NewVersion(1, 3000, "/NEO-GO/", 0, true)
	err = s.handleVersionCmd(p, version)
	assert.NotNil(t, err)
	assert.Equal(t, errIdenticalID, err)
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
