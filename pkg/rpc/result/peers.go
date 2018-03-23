package result

import (
	"strings"
)

type (
	// Peers payload for outputting peers in `getpeers` RPC call.
	Peers struct {
		Unconnected []Peer `json:"unconnected"`
		Connected   []Peer `json:"connected"`
		Bad         []Peer `json:"bad"`
	}

	// Peer represents the peer.
	Peer struct {
		Address string `json:"address"`
		Port    string `json:"port"`
	}
)

// NewPeers creates a new Peers struct.
func NewPeers() Peers {
	return Peers{
		Unconnected: []Peer{},
		Connected:   []Peer{},
		Bad:         []Peer{},
	}
}

// AddPeer adds a peer to the given peer type slice.
func (p *Peers) AddPeer(peerType string, addr string) {
	addressParts := strings.Split(addr, ":")
	peer := Peer{
		Address: addressParts[0],
		Port:    addressParts[1],
	}

	switch peerType {
	case "unconnected":
		p.Unconnected = append(
			p.Unconnected,
			peer,
		)

	case "connected":
		p.Connected = append(
			p.Connected,
			peer,
		)

	case "bad":
		p.Bad = append(
			p.Bad,
			peer,
		)
	}
}
