package payload

import (
	"encoding/binary"
)

const (
	lenUA          = 12
	minVersionSize = 27 + lenUA
)

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
	// client id currently 12 bytes \v/NEO:2.6.0/
	UserAgent []byte
	// Height of the block chain
	StartHeight uint32
	// Whether to receive and forward
	Relay bool
}

// NewVersion returns a pointer to a Version payload.
func NewVersion(id uint32, p uint16, ua string, h uint32, r bool) *Version {
	return &Version{
		Version:     0,
		Services:    1,
		Timestamp:   12345,
		Port:        p,
		Nonce:       id,
		UserAgent:   []byte(ua),
		StartHeight: 0,
		Relay:       r,
	}
}

// UnmarshalBinary implements the Payloader interface.
func (p *Version) UnmarshalBinary(b []byte) error {
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

// MarshalBinary implements the Payloader interface.
func (p *Version) MarshalBinary() ([]byte, error) {
	b := make([]byte, p.Size())

	binary.LittleEndian.PutUint32(b[0:4], p.Version)
	binary.LittleEndian.PutUint64(b[4:12], p.Services)
	binary.LittleEndian.PutUint32(b[12:16], p.Timestamp)
	// FIXME: byte order (little / big)?
	binary.LittleEndian.PutUint16(b[16:18], p.Port)
	binary.LittleEndian.PutUint32(b[18:22], p.Nonce)
	copy(b[22:22+len(p.UserAgent)], p.UserAgent) //
	curLen := 22 + len(p.UserAgent)
	binary.LittleEndian.PutUint32(b[curLen:curLen+4], p.StartHeight)

	// yikes
	var bln []byte
	if p.Relay {
		bln = []byte{1}
	} else {
		bln = []byte{0}
	}

	copy(b[curLen+4:len(b)], bln)

	return b, nil
}

func (p *Version) Size() uint32 {
	return uint32(minVersionSize + len(p.UserAgent))
}
