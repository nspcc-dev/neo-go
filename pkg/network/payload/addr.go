package payload

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Endpoint host + port of a node, compatible with net.Addr.
type Endpoint struct {
	IP   [16]byte // TODO: make a uint128 type
	Port uint16
}

// Network implements the net.Addr interface.
func (e Endpoint) Network() string { return "tcp" }

// String implements the net.Addr interface.
func (e Endpoint) String() string {
	b := make([]uint8, 4)
	for i := 0; i < 4; i++ {
		b[i] = byte(e.IP[len(e.IP)-4+i])
	}
	return fmt.Sprintf("%d.%d.%d.%d:%d", b[0], b[1], b[2], b[3], e.Port)
}

// AddrWithTime payload
type AddrWithTime struct {
	// Timestamp the node connected to the network.
	Timestamp uint32
	Services  uint64
	Addr      Endpoint
}

// Size implements the payloader interface.
func (p *AddrWithTime) Size() uint32 {
	return 30
}

// UnmarshalBinary implements the Payloader interface.
func (p *AddrWithTime) UnmarshalBinary(b []byte) error {
	p.Timestamp = binary.LittleEndian.Uint32(b[0:4])
	p.Services = binary.LittleEndian.Uint64(b[4:12])
	binary.Read(bytes.NewReader(b[12:28]), binary.BigEndian, &p.Addr.IP)
	p.Addr.Port = binary.BigEndian.Uint16(b[28:30])
	return nil
}

// MarshalBinary implements the Payloader interface.
func (p *AddrWithTime) MarshalBinary() ([]byte, error) {
	return nil, nil
}

// AddressList contains a slice of AddrWithTime.
type AddressList struct {
	Addrs []*AddrWithTime
}

// UnmarshalBinary implements the Payloader interface.
func (p *AddressList) UnmarshalBinary(b []byte) error {
	var lenList uint8
	binary.Read(bytes.NewReader(b[0:1]), binary.LittleEndian, &lenList)

	offset := 1 // skip the uint8 length byte.
	size := 30  // size of AddrWithTime
	for i := 0; i < int(lenList); i++ {
		address := &AddrWithTime{}
		address.UnmarshalBinary(b[offset : offset+size])
		p.Addrs = append(p.Addrs, address)
		offset += size
	}

	return nil
}

// MarshalBinary implements the Payloader interface.
func (p *AddressList) MarshalBinary() ([]byte, error) {
	return nil, nil
}

// Size implements the Payloader interface.
func (p *AddressList) Size() uint32 {
	return uint32(len(p.Addrs) * 30)
}
