package payload

import (
	"encoding/binary"
	"io"
	"time"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// AddressAndTime payload.
type AddressAndTime struct {
	Timestamp uint32
	Services  uint64
	Endpoint  util.Endpoint
}

// NewAddressAndTime creates a new AddressAndTime object.
func NewAddressAndTime(e util.Endpoint, t time.Time) *AddressAndTime {
	return &AddressAndTime{
		Timestamp: uint32(t.UTC().Unix()),
		Services:  1,
		Endpoint:  e,
	}
}

// DecodeBinary implements the Payload interface.
func (p *AddressAndTime) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &p.Timestamp); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &p.Services); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &p.Endpoint.IP); err != nil {
		return err
	}
	return binary.Read(r, binary.BigEndian, &p.Endpoint.Port)
}

// EncodeBinary implements the Payload interface.
func (p *AddressAndTime) EncodeBinary(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, p.Timestamp); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.Services); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, p.Endpoint.IP); err != nil {
		return err
	}
	return binary.Write(w, binary.BigEndian, p.Endpoint.Port)
}

// AddressList is a list with AddrAndTime.
type AddressList struct {
	Addrs []*AddressAndTime
}

// DecodeBinary implements the Payload interface.
func (p *AddressList) DecodeBinary(r io.Reader) error {
	listLen := util.ReadVarUint(r)

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
	if err := util.WriteVarUint(w, uint64(len(p.Addrs))); err != nil {
		return err
	}
	for _, addr := range p.Addrs {
		if err := addr.EncodeBinary(w); err != nil {
			return err
		}
	}
	return nil
}
