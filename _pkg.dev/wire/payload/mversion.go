// Copied and Modified for NEO from: https://github.com/decred/dcrd/blob/master/wire/VersionMessage.go

package payload

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

const minMsgVersionSize = 28

var errInvalidNetAddr = errors.New("provided net.Addr is not a net.TCPAddr")

//VersionMessage represents a version message on the neo-network
type VersionMessage struct {
	Version     protocol.Version
	Timestamp   uint32
	Services    protocol.ServiceFlag
	IP          net.IP
	Port        uint16
	Nonce       uint32
	UserAgent   []byte
	StartHeight uint32
	Relay       bool
}

//NewVersionMessage will return a VersionMessage object
func NewVersionMessage(addr net.Addr, startHeight uint32, relay bool, pver protocol.Version, userAgent string, nonce uint32, services protocol.ServiceFlag) (*VersionMessage, error) {

	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return nil, errInvalidNetAddr
	}

	version := &VersionMessage{
		pver,
		uint32(time.Now().Unix()),
		services,
		tcpAddr.IP,
		uint16(tcpAddr.Port),
		nonce,
		[]byte(userAgent),
		startHeight,
		relay,
	}
	return version, nil
}

// DecodePayload Implements Messager interface
func (v *VersionMessage) DecodePayload(r io.Reader) error {
	br := &util.BinReader{R: r}
	br.Read(&v.Version)
	br.Read(&v.Services)
	br.Read(&v.Timestamp)
	br.Read(&v.Port) // Port is not BigEndian as stated in the docs
	br.Read(&v.Nonce)

	var lenUA uint8
	br.Read(&lenUA)

	v.UserAgent = make([]byte, lenUA)
	br.Read(&v.UserAgent)
	br.Read(&v.StartHeight)
	br.Read(&v.Relay)
	return br.Err
}

// EncodePayload Implements messager interface
func (v *VersionMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}

	bw.Write(v.Version)
	bw.Write(v.Services)
	bw.Write(v.Timestamp)
	bw.Write(v.Port) // Not big End
	bw.Write(v.Nonce)
	bw.Write(uint8(len(v.UserAgent)))
	bw.Write(v.UserAgent)
	bw.Write(v.StartHeight)
	bw.Write(v.Relay)
	return bw.Err
}

// Command Implements messager interface
func (v *VersionMessage) Command() command.Type {
	return command.Version
}
