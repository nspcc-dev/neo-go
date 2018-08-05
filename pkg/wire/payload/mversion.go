// Copied and Modified for NEO from: https://github.com/decred/dcrd/blob/master/wire/VersionMessage.go

package payload

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"net"
	"time"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

const (
	minMsgVersionSize = 28
)

type VersionMessage struct {
	w           *bytes.Buffer
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

func NewVersionMessage(addr net.Addr, startHeight uint32, relay bool, pver protocol.Version) (*VersionMessage, error) {
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return nil, ErrInvalidNetAddr
	}
	version := &VersionMessage{
		new(bytes.Buffer),
		pver,
		uint32(time.Now().Unix()),
		protocol.NodePeerService,
		tcpAddr.IP,
		uint16(tcpAddr.Port),
		rand.Uint32(),
		[]byte(protocol.UserAgent),
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

	buf, err := util.ReaderToBuffer(r)
	if err != nil {
		return err
	}

	v.w = buf

	r = bytes.NewReader(buf.Bytes()) // reader is a pointer, so ReaderToBuffer will drain all bytes from it. Repopulate

	br := &util.BinReader{R: r}
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
	return br.Err
}

// Implements messager interface
func (v *VersionMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}

	bw.Write(v.Version)
	bw.Write(v.Services)
	bw.Write(v.Timestamp)
	bw.WriteBigEnd(v.Port)
	bw.Write(v.Nonce)
	bw.Write(uint8(len(v.UserAgent)))
	bw.Write(v.UserAgent)
	bw.Write(v.StartHeight)
	bw.Write(v.Relay)
	return bw.Err
}

// Implements messager interface
func (v *VersionMessage) PayloadLength() uint32 {
	return util.CalculatePayloadLength(v.w)
}

// Implements messager interface
func (v *VersionMessage) Checksum() uint32 {
	return util.CalculateCheckSum(v.w)
}

// Implements messager interface
func (v *VersionMessage) Command() command.Type {
	return command.Version
}
