package wire

import (
	"bytes"
	"io"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// Once a VersionMessage is received, we can then store it inside of AddrMessage struct

type net_addr struct {
	Timestamp uint32
	IP        [16]byte
	Port      uint16
	Service   ServiceFlag
}

func NewAddrMessage(time uint32, ip [16]byte, port uint16, service ServiceFlag) (*net_addr, error) {
	return &net_addr{time, ip, port, service}, nil
}

func NewAddrFromVersionMessage(version VersionMessage) (*net_addr, error) {

	var ip [16]byte

	copy(ip[:], []byte(version.IP)[:16])

	return NewAddrMessage(version.Timestamp, ip, version.Port, version.Services)
}

func (n *net_addr) EncodePayload(bw *util.BinWriter) {

	bw.Write(uint32(time.Now().Unix()))
	bw.Write(NodePeerService)
	bw.WriteBigEnd(n.IP)
	bw.WriteBigEnd(n.Port)
}
func (n *net_addr) DecodePayload(br *util.BinReader) {

	br.Read(&n.Timestamp)
	br.Read(&n.Service)
	br.ReadBigEnd(&n.IP)
	br.ReadBigEnd(&n.Port)
}

type AddrMessage struct {
	w        *bytes.Buffer
	AddrList []*net_addr
}

// Implements Messager interface
func (a *AddrMessage) DecodePayload(r io.Reader) error {
	br := &util.BinReader{R: r}
	listLen := br.VarUint()

	a.AddrList = make([]*net_addr, listLen)
	for i := 0; i < int(listLen); i++ {
		a.AddrList[i] = &net_addr{}
		a.AddrList[i].DecodePayload(br)
		if br.Err != nil {
			return br.Err
		}
	}
	return br.Err
}

// Implements messager interface
func (v *AddrMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}

	listLen := uint64(len(v.AddrList))
	bw.VarUint(listLen)

	for _, addr := range v.AddrList {
		addr.EncodePayload(bw)
	}
	return bw.Err
}

// Implements messager interface
func (v *AddrMessage) PayloadLength() uint32 {
	return calculatePayloadLength(v.w)
}

// Implements messager interface
func (v *AddrMessage) Checksum() uint32 {
	return calculateCheckSum(v.w)
}

// Implements messager interface
func (v *AddrMessage) Command() CommandType {
	return CMDAddr
}
