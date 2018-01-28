package payload

import (
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Endpoint host + port of a node, compatible with net.Addr.
type Endpoint struct {
	IP   [16]byte // TODO: make a uint128 type
	Port uint16
}

// EndpointFromString returns an Endpoint from the given string.
// For now this only handles the most simple hostport form.
// e.g. 127.0.0.1:3000
// This should be enough to work with for now.
func EndpointFromString(s string) (Endpoint, error) {
	hostPort := strings.Split(s, ":")
	if len(hostPort) != 2 {
		return Endpoint{}, fmt.Errorf("invalid address string: %s", s)
	}
	host := hostPort[0]
	port := hostPort[1]

	ch := strings.Split(host, ".")

	buf := [16]byte{}
	var n int
	for i := 0; i < len(ch); i++ {
		n = 12 + i
		nn, _ := strconv.Atoi(ch[i])
		buf[n] = byte(nn)
	}

	p, _ := strconv.Atoi(port)

	return Endpoint{buf, uint16(p)}, nil
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

func NewAddrWithTime(addr Endpoint) *AddrWithTime {
	return &AddrWithTime{
		Timestamp: 1337,
		Services:  1,
		Addr:      addr,
	}
}

// Size implements the payload interface.
func (p *AddrWithTime) Size() uint32 {
	return 30
}

// DecodeBinary implements the Payload interface.
func (p *AddrWithTime) DecodeBinary(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &p.Timestamp)
	err = binary.Read(r, binary.LittleEndian, &p.Services)
	err = binary.Read(r, binary.BigEndian, &p.Addr.IP)
	err = binary.Read(r, binary.BigEndian, &p.Addr.Port)

	return err
}

// EncodeBinary implements the Payload interface.
func (p *AddrWithTime) EncodeBinary(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, p.Timestamp)
	err = binary.Write(w, binary.LittleEndian, p.Services)
	err = binary.Write(w, binary.BigEndian, p.Addr.IP)
	err = binary.Write(w, binary.BigEndian, p.Addr.Port)

	return err
}

// AddressList holds a slice of AddrWithTime.
type AddressList struct {
	Addrs []*AddrWithTime
}

// DecodeBinary implements the Payload interface.
func (p *AddressList) DecodeBinary(r io.Reader) error {
	var lenList uint8
	binary.Read(r, binary.LittleEndian, &lenList)

	for i := 0; i < int(4); i++ {
		address := &AddrWithTime{}
		if err := address.DecodeBinary(r); err != nil {
			return err
		}
		p.Addrs = append(p.Addrs, address)
	}

	return nil
}

// EncodeBinary implements the Payload interface.
func (p *AddressList) EncodeBinary(w io.Writer) error {
	// Write the length of the slice
	binary.Write(w, binary.LittleEndian, uint8(len(p.Addrs)))

	for _, addr := range p.Addrs {
		if err := addr.EncodeBinary(w); err != nil {
			return err
		}
	}

	return nil
}

// Size implements the Payloader interface.
func (p *AddressList) Size() uint32 {
	return uint32(len(p.Addrs) * 30)
}
