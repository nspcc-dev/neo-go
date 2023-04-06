package result

import (
	"encoding/json"
	"net"
	"strconv"
)

type (
	// GetPeers payload for outputting peers in `getpeers` RPC call.
	GetPeers struct {
		Unconnected Peers `json:"unconnected"`
		Connected   Peers `json:"connected"`
		Bad         Peers `json:"bad"`
	}

	// Peers represents a slice of peers.
	Peers []Peer

	// Peer represents a peer.
	Peer struct {
		Address string `json:"address"`
		Port    uint16 `json:"port"`
	}
)

// NewGetPeers creates a new GetPeers structure.
func NewGetPeers() GetPeers {
	return GetPeers{
		Unconnected: []Peer{},
		Connected:   []Peer{},
		Bad:         []Peer{},
	}
}

// AddUnconnected adds a set of peers to the unconnected peers slice.
func (g *GetPeers) AddUnconnected(addrs []string) {
	g.Unconnected.addPeers(addrs)
}

// AddConnected adds a set of peers to the connected peers slice.
func (g *GetPeers) AddConnected(addrs []string) {
	g.Connected.addPeers(addrs)
}

// AddBad adds a set of peers to the bad peers slice.
func (g *GetPeers) AddBad(addrs []string) {
	g.Bad.addPeers(addrs)
}

// addPeers adds a set of peers to the given peer slice.
func (p *Peers) addPeers(addrs []string) {
	for i := range addrs {
		host, portStr, err := net.SplitHostPort(addrs[i])
		if err != nil {
			continue
		}
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			port = 0
		}
		peer := Peer{
			Address: host,
			Port:    uint16(port),
		}

		*p = append(*p, peer)
	}
}

func (p *Peer) UnmarshalJSON(data []byte) error {
	type NewPeer Peer
	var np NewPeer

	err := json.Unmarshal(data, &np)
	if err == nil {
		*p = Peer(np)
		return nil
	}

	type OldPeer struct {
		Address string `json:"address"`
		Port    string `json:"port"`
	}
	var op OldPeer

	err = json.Unmarshal(data, &op)
	if err == nil {
		port, err := strconv.ParseUint(op.Port, 10, 16)
		if err != nil {
			return err
		}

		*p = Peer{
			Address: op.Address,
			Port:    uint16(port),
		}
	}
	return err
}
