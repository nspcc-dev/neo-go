package payload

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// AddrWithTime payload
type AddrWithTime struct {
	// Timestamp the node connected to the network.
	Timestamp uint32
	Services  uint64
	Addr      util.Endpoint
}

// NewAddrWithTime return a pointer to AddrWithTime.
func NewAddrWithTime(addr util.Endpoint) *AddrWithTime {
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

	p.Addrs = make([]*AddrWithTime, lenList)
	for i := 0; i < int(4); i++ {
		addr := &AddrWithTime{}
		if err := addr.DecodeBinary(r); err != nil {
			return err
		}
		p.Addrs[i] = addr
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
