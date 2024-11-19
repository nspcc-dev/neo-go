package result

import (
	"net"
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/network"
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
		Address         string `json:"address"`
		Port            uint16 `json:"port"`
		UserAgent       string `json:"useragent,omitempty"`
		LastKnownHeight uint32 `json:"lastknownheight,omitempty"`
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

// AddConnected adds a set of connected peers to the connected peers slice.
func (g *GetPeers) AddConnected(connectedPeers []network.PeerInfo) {
	g.Connected.addConnectedPeers(connectedPeers)
}

// AddBad adds a set of peers to the bad peers slice.
func (g *GetPeers) AddBad(addrs []string) {
	g.Bad.addPeers(addrs)
}

// addPeers adds a set of peers to the given peer slice.
func (p *Peers) addPeers(addrs []string) {
	for i := range addrs {
		host, port, err := parseHostPort(addrs[i])
		if err != nil {
			continue
		}
		peer := Peer{
			Address: host,
			Port:    port,
		}

		*p = append(*p, peer)
	}
}

// addConnectedPeers adds a set of connected peers to the given peer slice.
func (p *Peers) addConnectedPeers(connectedPeers []network.PeerInfo) {
	for i := range connectedPeers {
		host, port, err := parseHostPort(connectedPeers[i].Address)
		if err != nil {
			continue
		}
		peer := Peer{
			Address:         host,
			Port:            port,
			UserAgent:       connectedPeers[i].UserAgent,
			LastKnownHeight: connectedPeers[i].Height,
		}

		*p = append(*p, peer)
	}
}

// parseHostPort parses host and port from the given address.
// An improperly formatted port string will return zero port.
func parseHostPort(addr string) (string, uint16, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, _ := strconv.ParseUint(portStr, 10, 16)
	return host, uint16(port), nil
}
