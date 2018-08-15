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

const (
	minMsgVersionSize = 28
)

// TODO: Refactor to pull out the useragent out of initialiser
// and have a seperate method to add it

type VersionMessage struct {
	// w           *bytes.Buffer
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

var ErrInvalidNetAddr = errors.New("provided net.Addr is not a net.TCPAddr")

func NewVersionMessage(addr net.Addr, startHeight uint32, relay bool, pver protocol.Version, userAgent string, nonce uint32, services protocol.ServiceFlag) (*VersionMessage, error) {

	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return nil, ErrInvalidNetAddr
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

// Implements Messager interface
func (v *VersionMessage) DecodePayload(r io.Reader) error {
	br := &util.BinReader{R: r}
	br.Read(&v.Version)
	br.Read(&v.Services)
	br.Read(&v.Timestamp)
	br.Read(&v.Port) // Port is not BigEndian
	br.Read(&v.Nonce)

	var lenUA uint8
	br.Read(&lenUA)

	v.UserAgent = make([]byte, lenUA)
	br.Read(&v.UserAgent)
	br.Read(&v.StartHeight)
	br.Read(&v.Relay)
	return br.Err
}

// Implements messager interface
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

// Implements messager interface
func (v *VersionMessage) Command() command.Type {
	return command.Version
}
