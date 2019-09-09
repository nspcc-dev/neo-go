package payload

import (
	"io"
	"net"
	"time"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// AddressAndTime payload.
type AddressAndTime struct {
	Timestamp uint32
	Services  uint64
	IP        [16]byte
	Port      uint16
}

// NewAddressAndTime creates a new AddressAndTime object.
func NewAddressAndTime(e *net.TCPAddr, t time.Time) *AddressAndTime {
	aat := AddressAndTime{
		Timestamp: uint32(t.UTC().Unix()),
		Services:  1,
		Port:      uint16(e.Port),
	}
	copy(aat.IP[:], e.IP)
	return &aat
}

// DecodeBinary implements the Payload interface.
func (p *AddressAndTime) DecodeBinary(r io.Reader) error {
	br := util.BinReader{R: r}
	br.ReadLE(&p.Timestamp)
	br.ReadLE(&p.Services)
	br.ReadBE(&p.IP)
	br.ReadBE(&p.Port)
	return br.Err
}

// EncodeBinary implements the Payload interface.
func (p *AddressAndTime) EncodeBinary(w io.Writer) error {
	bw := util.BinWriter{W: w}
	bw.WriteLE(p.Timestamp)
	bw.WriteLE(p.Services)
	bw.WriteBE(p.IP)
	bw.WriteBE(p.Port)
	return bw.Err
}

// AddressList is a list with AddrAndTime.
type AddressList struct {
	Addrs []*AddressAndTime
}

// DecodeBinary implements the Payload interface.
func (p *AddressList) DecodeBinary(r io.Reader) error {
	br := util.BinReader{R: r}
	listLen := br.ReadVarUint()
	if br.Err != nil {
		return br.Err
	}

	p.Addrs = make([]*AddressAndTime, listLen)
	for i := 0; i < int(listLen); i++ {
		p.Addrs[i] = &AddressAndTime{}
		if err := p.Addrs[i].DecodeBinary(r); err != nil {
			return err
		}
	}
	return nil
}

// EncodeBinary implements the Payload interface.
func (p *AddressList) EncodeBinary(w io.Writer) error {
	bw := util.BinWriter{W: w}
	bw.WriteVarUint(uint64(len(p.Addrs)))
	if bw.Err != nil {
		return bw.Err
	}
	for _, addr := range p.Addrs {
		if err := addr.EncodeBinary(w); err != nil {
			return err
		}
	}
	return nil
}
