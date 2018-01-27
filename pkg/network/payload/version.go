package payload

import (
	"bytes"
	"encoding/binary"
	"io"
)

const minVersionSize = 27

// Version payload.
type Version struct {
	// currently the version of the protocol is 0
	Version uint32
	// currently 1
	Services uint64
	// timestamp
	Timestamp uint32
	// port this server is listening on
	Port uint16
	// it's used to distinguish the node from public IP
	Nonce uint32
	// client id currently 6 bytes \v/NEO:2.6.0/
	UserAgent []byte
	// Height of the block chain
	StartHeight uint32
	// Whether to receive and forward
	Relay bool
}

// NewVersion returns a pointer to a Version payload.
func NewVersion(p uint16, ua string, h uint32, r bool) *Version {
	return &Version{
		Version:     0,
		Services:    1,
		Timestamp:   12345,
		Port:        p,
		Nonce:       19110,
		UserAgent:   []byte(ua),
		StartHeight: 0,
		Relay:       r,
	}
}

// Size ..
func (p *Version) Size() uint32 {
	n := minVersionSize + len(p.UserAgent)
	return uint32(n)
}

// Decode ..
func (p *Version) Decode(r io.Reader) error {
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r); err != nil {
		return err
	}

	b := buf.Bytes()
	// 27 bytes for the fixed size fields + the length of the user agent
	// which is kinda variable, according to the docs.
	lenUA := len(b) - minVersionSize

	p.Version = binary.LittleEndian.Uint32(b[0:4])
	p.Services = binary.LittleEndian.Uint64(b[4:12])
	p.Timestamp = binary.LittleEndian.Uint32(b[12:16])
	// FIXME: port's byteorder should be big endian according to the docs.
	// but when connecting to the privnet docker image it's little endian.
	p.Port = binary.LittleEndian.Uint16(b[16:18])
	p.Nonce = binary.LittleEndian.Uint32(b[18:22])
	p.UserAgent = b[22 : 22+lenUA]
	curlen := 22 + lenUA
	p.StartHeight = binary.LittleEndian.Uint32(b[curlen : curlen+4])
	p.Relay = b[len(b)-1 : len(b)][0] == 1

	return nil
}

// Encode ..
func (p *Version) Encode(w io.Writer) error {
	buf := make([]byte, p.Size())

	binary.LittleEndian.PutUint32(buf[0:4], p.Version)
	binary.LittleEndian.PutUint64(buf[4:12], p.Services)
	binary.LittleEndian.PutUint32(buf[12:16], p.Timestamp)
	// FIXME: byte order (little / big)?
	binary.LittleEndian.PutUint16(buf[16:18], p.Port)
	binary.LittleEndian.PutUint32(buf[18:22], p.Nonce)
	copy(buf[22:22+len(p.UserAgent)], p.UserAgent) //
	curLen := 22 + len(p.UserAgent)
	binary.LittleEndian.PutUint32(buf[curLen:curLen+4], p.StartHeight)

	// yikes
	var b []byte
	if p.Relay {
		b = []byte{1}
	} else {
		b = []byte{0}
	}

	copy(buf[curLen+4:len(buf)], b)

	_, err := w.Write(buf)
	return err
}
