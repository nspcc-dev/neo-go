package payload

import (
	"net"
	"strconv"
	"time"

	"github.com/CityOfZion/neo-go/pkg/io"
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

// DecodeBinary implements Serializable interface.
func (p *AddressAndTime) DecodeBinary(br *io.BinReader) {
	br.ReadLE(&p.Timestamp)
	br.ReadLE(&p.Services)
	br.ReadBE(&p.IP)
	br.ReadBE(&p.Port)
}

// EncodeBinary implements Serializable interface.
func (p *AddressAndTime) EncodeBinary(bw *io.BinWriter) {
	bw.WriteLE(p.Timestamp)
	bw.WriteLE(p.Services)
	bw.WriteBE(p.IP)
	bw.WriteBE(p.Port)
}

// IPPortString makes a string from IP and port specified.
func (p *AddressAndTime) IPPortString() string {
	var netip = make(net.IP, 16)

	copy(netip, p.IP[:])
	port := strconv.Itoa(int(p.Port))
	return netip.String() + ":" + port
}

// AddressList is a list with AddrAndTime.
type AddressList struct {
	Addrs []*AddressAndTime
}

// NewAddressList creates a list for n AddressAndTime elements.
func NewAddressList(n int) *AddressList {
	alist := AddressList{
		Addrs: make([]*AddressAndTime, n),
	}
	return &alist
}

// DecodeBinary implements Serializable interface.
func (p *AddressList) DecodeBinary(br *io.BinReader) {
	p.Addrs = br.ReadArray(AddressAndTime{}).([]*AddressAndTime)
}

// EncodeBinary implements Serializable interface.
func (p *AddressList) EncodeBinary(bw *io.BinWriter) {
	bw.WriteArray(p.Addrs)
}
