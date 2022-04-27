package payload

import (
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
)

// MaxAddrsCount is the maximum number of addresses that could be packed into
// one payload.
const MaxAddrsCount = 200

// AddressAndTime payload.
type AddressAndTime struct {
	Timestamp    uint32
	IP           [16]byte
	Capabilities capability.Capabilities
}

// NewAddressAndTime creates a new AddressAndTime object.
func NewAddressAndTime(e *net.TCPAddr, t time.Time, c capability.Capabilities) *AddressAndTime {
	aat := AddressAndTime{
		Timestamp:    uint32(t.UTC().Unix()),
		Capabilities: c,
	}
	copy(aat.IP[:], e.IP)
	return &aat
}

// DecodeBinary implements the Serializable interface.
func (p *AddressAndTime) DecodeBinary(br *io.BinReader) {
	p.Timestamp = br.ReadU32LE()
	br.ReadBytes(p.IP[:])
	p.Capabilities.DecodeBinary(br)
}

// EncodeBinary implements the Serializable interface.
func (p *AddressAndTime) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(p.Timestamp)
	bw.WriteBytes(p.IP[:])
	p.Capabilities.EncodeBinary(bw)
}

// GetTCPAddress makes a string from the IP and the port specified in TCPCapability.
// It returns an error if there's no such capability.
func (p *AddressAndTime) GetTCPAddress() (string, error) {
	var netip = make(net.IP, 16)

	copy(netip, p.IP[:])
	port := -1
	for _, cap := range p.Capabilities {
		if cap.Type == capability.TCPServer {
			port = int(cap.Data.(*capability.Server).Port)
			break
		}
	}
	if port == -1 {
		return "", errors.New("no TCP capability found")
	}
	return net.JoinHostPort(netip.String(), strconv.Itoa(port)), nil
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

// DecodeBinary implements the Serializable interface.
func (p *AddressList) DecodeBinary(br *io.BinReader) {
	br.ReadArray(&p.Addrs, MaxAddrsCount)
	if len(p.Addrs) == 0 {
		br.Err = errors.New("no addresses listed")
	}
}

// EncodeBinary implements the Serializable interface.
func (p *AddressList) EncodeBinary(bw *io.BinWriter) {
	bw.WriteArray(p.Addrs)
}
