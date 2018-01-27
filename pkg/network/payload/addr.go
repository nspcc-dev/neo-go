package payload

import (
	"io"
	"net"
	"unsafe"
)

// AddrWithTime payload
type AddrWithTime struct {
	Timestamp uint32
	Services  uint64
	Addr      net.Addr
}

func (p *AddrWithTime) Size() uint32 {
	return 4 + 8 + uint32(unsafe.Sizeof(p.Addr))
}

func (p *AddrWithTime) Encode(r io.Reader) error {
	return nil
}

func (p *AddrWithTime) Decode(w io.Writer) error {
	return nil
}

// AddressList is a slice of AddrWithTime.
type AddressList []*AddrWithTime

func (p AddressList) Encode(r io.Reader) error {
	return nil
}

func (p AddressList) Decode(w io.Writer) error {
	return nil
}
