package network

import "net"

// AddrWithTimestamp payload.
type AddrWithTimestamp struct {
	t        uint32
	services uint64
	endpoint net.Addr
}

func newAddrWithTimestampFromPeer(p *Peer) AddrWithTimestamp {
	return AddrWithTimestamp{
		t:        1223345,
		services: 1,
		endpoint: p.conn.RemoteAddr(),
	}
}

// AddrPayload container a list of known peer addresses.
type AddrPayload []AddrWithTimestamp

func (p AddrPayload) encode() ([]byte, error) {
	return nil, nil
}
