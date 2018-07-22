// Copied and Modified for NEO from: https://github.com/decred/dcrd/blob/master/wire/VersionMessage.go

package wire

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"net"
	"time"
)

const (
	minMsgVersionSize = 28
)

type VersionMessage struct {
	w           *bytes.Buffer
	Version     ProtocolVersion
	Timestamp   uint32
	Services    ServiceFlag
	IP          net.IP
	Port        uint16
	Nonce       uint32
	UserAgent   []byte
	StartHeight uint32
	Relay       bool
}

var ErrInvalidNetAddr = errors.New("provided net.Addr is not a net.TCPAddr")

func NewVersionMessage(addr net.Addr, startHeight uint32, relay bool, pver ProtocolVersion) (*VersionMessage, error) {
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return nil, ErrInvalidNetAddr
	}
	version := &VersionMessage{
		new(bytes.Buffer),
		pver,
		uint32(time.Now().Unix()),
		NodePeerService,
		tcpAddr.IP,
		uint16(tcpAddr.Port),
		rand.Uint32(),
		[]byte(UserAgent),
		startHeight,
		relay,
	}

	// saves a buffer of version in version
	if err := version.EncodePayload(version.w); err != nil {
		return nil, err
	}
	return version, nil
}

// Implements Messager interface
func (v *VersionMessage) DecodePayload(r io.Reader) error {
	// Decode into v from reader

	br := &binReader{r: r}
	br.Read(&v.Version)

	br.Read(&v.Services)

	br.Read(&v.Timestamp)

	br.ReadBigEnd(&v.Port)

	br.Read(&v.Nonce)

	var lenUA uint8
	br.Read(&lenUA)

	v.UserAgent = make([]byte, lenUA)
	br.Read(&v.UserAgent)
	br.Read(&v.StartHeight)
	br.Read(&v.Relay)

	v.w = new(bytes.Buffer)
	if err := v.EncodePayload(v.w); err != nil {
		return err
	}
	return br.err
}

// Implements messager interface
func (v *VersionMessage) EncodePayload(w io.Writer) error {
	bw := &binWriter{w: w}

	bw.Write(v.Version)
	bw.Write(v.Services)
	bw.Write(v.Timestamp)
	bw.WriteBigEnd(v.Port)
	bw.Write(v.Nonce)
	bw.Write(uint8(len(v.UserAgent)))
	bw.Write(v.UserAgent)
	bw.Write(v.StartHeight)
	bw.Write(v.Relay)
	return bw.err
}

// Implements messager interface
func (v *VersionMessage) PayloadLength() uint32 {
	return calculatePayloadLength(v.w)
}

// Implements messager interface
func (v *VersionMessage) Checksum() uint32 {
	return calculateCheckSum(v.w)
}

// Implements messager interface
func (v *VersionMessage) Command() CommandType {
	return CMDVersion
}
