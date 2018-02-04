package network

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

// TODO this should be moved to localPeer test.

func TestHandleVersionFailWrongPort(t *testing.T) {
	s := NewServer(ModeDevNet)
	go s.loop()

	p := NewLocalPeer(s)

	version := payload.NewVersion(1337, 1, "/NEO:0.0.0/", 0, true)
	msg := newMessage(ModeDevNet, cmdVersion, version)
	if err := s.handleVersionCmd(msg, p); err == nil {
		t.Fatal("expected error got nil")
	}
}

func TestHandleVersionFailIdenticalNonce(t *testing.T) {
	s := NewServer(ModeDevNet)
	go s.loop()

	p := NewLocalPeer(s)

	version := payload.NewVersion(s.id, 1, "/NEO:0.0.0/", 0, true)
	msg := newMessage(ModeDevNet, cmdVersion, version)
	if err := s.handleVersionCmd(msg, p); err == nil {
		t.Fatal("expected error got nil")
	}
}

func TestHandleVersion(t *testing.T) {
	s := NewServer(ModeDevNet)
	go s.loop()

	p := NewLocalPeer(s)

	version := payload.NewVersion(1337, p.addr().Port, "/NEO:0.0.0/", 0, true)
	msg := newMessage(ModeDevNet, cmdVersion, version)

	if err := s.handleVersionCmd(msg, p); err != nil {
		t.Fatal(err)
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
