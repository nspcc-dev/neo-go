package network

import (
	"testing"
)

func TestHandleVersion(t *testing.T) {
	// s := NewServer(ModeDevNet)
	// go s.Start(":3000", nil)

	// p := NewLocalPeer()
	// s.register <- p

	// version := payload.NewVersion(1337, p.endpoint().Port, "/NEO:0.0.0/.", 0, true)
	// s.handleVersionCmd(version, p)

	// if len(s.peers) != 1 {
	// 	t.Fatalf("expecting the server to have %d peers got %d", 1, len(s.peers))
	// }
	// if p.id() != 1337 {
	// 	t.Fatalf("expecting peer's id to be %d got %d", 1337, p._id)
	// }
	// if !p.verack() {
	// 	t.Fatal("expecting peer to be verified")
	// }
}
