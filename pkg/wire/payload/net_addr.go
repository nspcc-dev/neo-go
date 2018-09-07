package payload

import (
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// Once a VersionMessage is received, we can then store it inside of AddrMessage struct
// TODO: store this inside version message and have a bool to indicate whether to encode ip
// VersionMessage does not encodeIP
type Net_addr struct {
	Timestamp uint32
	IP        [16]byte
	Port      uint16
	Service   protocol.ServiceFlag
}

func NewNetAddr(time uint32, ip [16]byte, port uint16, service protocol.ServiceFlag) (*Net_addr, error) {
	return &Net_addr{time, ip, port, service}, nil
}

func NewAddrFromVersionMessage(version VersionMessage) (*Net_addr, error) {

	var ip [16]byte

	copy(ip[:], []byte(version.IP)[:16])

	return NewNetAddr(version.Timestamp, ip, version.Port, version.Services)
}

func (n *Net_addr) EncodePayload(bw *util.BinWriter) {

	bw.Write(uint32(time.Now().Unix()))
	bw.Write(protocol.NodePeerService)
	bw.WriteBigEnd(n.IP)
	bw.WriteBigEnd(n.Port)
}
func (n *Net_addr) DecodePayload(br *util.BinReader) {

	br.Read(&n.Timestamp)
	br.Read(&n.Service)
	br.ReadBigEnd(&n.IP)
	br.ReadBigEnd(&n.Port)
}
