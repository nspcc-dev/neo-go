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
	p.Timestamp = br.ReadU32LE()
	p.Services = br.ReadU64LE()
	br.ReadBytes(p.IP[:])
	p.Port = br.ReadU16BE()
}

// EncodeBinary implements Serializable interface.
func (p *AddressAndTime) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(p.Timestamp)
	bw.WriteU64LE(p.Services)
	bw.WriteBytes(p.IP[:])
	bw.WriteU16BE(p.Port)
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
	br.ReadArray(&p.Addrs)
}

// EncodeBinary implements Serializable interface.
func (p *AddressList) EncodeBinary(bw *io.BinWriter) {
	bw.WriteArray(p.Addrs)
}
