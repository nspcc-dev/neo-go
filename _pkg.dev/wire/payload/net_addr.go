package payload

import (
	"net"
	"strconv"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

//NetAddr is an abstraction for the IP layer
type NetAddr struct {
	Timestamp uint32
	IP        [16]byte
	Port      uint16
	Service   protocol.ServiceFlag
}

//NewNetAddr returns a NetAddr object
func NewNetAddr(time uint32, ip [16]byte, port uint16, service protocol.ServiceFlag) (*NetAddr, error) {
	return &NetAddr{time, ip, port, service}, nil
}

//NewAddrFromVersionMessage returns a NetAddr object from a version message
func NewAddrFromVersionMessage(version VersionMessage) (*NetAddr, error) {

	var ip [16]byte

	copy(ip[:], []byte(version.IP)[:16])

	return NewNetAddr(version.Timestamp, ip, version.Port, version.Services)
}

// EncodePayload Implements messager interface
func (n *NetAddr) EncodePayload(bw *util.BinWriter) {

	bw.Write(uint32(time.Now().Unix()))
	bw.Write(protocol.NodePeerService)
	bw.WriteBigEnd(n.IP)
	bw.WriteBigEnd(n.Port)
}

// DecodePayload Implements Messager interface
func (n *NetAddr) DecodePayload(br *util.BinReader) {

	br.Read(&n.Timestamp)
	br.Read(&n.Service)
	br.ReadBigEnd(&n.IP)
	br.ReadBigEnd(&n.Port)
}

//IPPort returns the IPPort from the NetAddr
func (n *NetAddr) IPPort() string {
	ip := net.IP(n.IP[:]).String()
	port := strconv.Itoa(int(n.Port))
	ipport := ip + ":" + port
	return ipport
}
