package network

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

func TestHandleVersion(t *testing.T) {
	s := NewServer(ModeDevNet)
	go s.loop()

	p := NewLocalPeer(s)

	version := payload.NewVersion(1337, p.addr().Port, "/NEO:0.0.0/", 0, true)
	msg := newMessage(ModeDevNet, cmdVersion, version)

	resp := s.handleVersionCmd(msg, p)
	if resp.commandType() != cmdVerack {
		t.Fatalf("expected response message to be verack got %s", resp.commandType())
	}
	if resp.Payload != nil {
		t.Fatal("verack payload should be nil")
	}
}

func TestPeerCount(t *testing.T) {
	s := NewServer(ModeDevNet)
	go s.loop()

	lenPeers := 10
	for i := 0; i < lenPeers; i++ {
		s.register <- NewLocalPeer(s)
	}

	if have, want := s.peerCount(), lenPeers; want != have {
		t.Fatalf("expected %d connected peers got %d", want, have)
	}
}

func TestHandleAddrCmd(t *testing.T) {
	// todo
}

func TestHandleGetAddrCmd(t *testing.T) {
	// todo
}

func TestHandleInv(t *testing.T) {
	// todo
}
func TestHandleBlockCmd(t *testing.T) {
	// todo
}
