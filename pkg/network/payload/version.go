package payload

import (
	"encoding/binary"
	"io"
)

const (
	lenUA          = 12
	minVersionSize = 27
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
	// client id
	UserAgent [lenUA]byte
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
		UserAgent:   uaToByteArray(ua),
		StartHeight: 0,
		Relay:       r,
	}
}

// DecodeBinary implements the Payload interface.
func (p *Version) DecodeBinary(r io.Reader) error {
	// TODO: Length of the user agent should be calculated dynamicaly.
	// There is no information about the size or format of this.
	// the only thing we know is by looking at the #c source code.
	// /NEO:{0}/ => /NEO:2.6.0/
	err := binary.Read(r, binary.LittleEndian, &p.Version)
	err = binary.Read(r, binary.LittleEndian, &p.Services)
	err = binary.Read(r, binary.LittleEndian, &p.Timestamp)
	err = binary.Read(r, binary.LittleEndian, &p.Port)
	err = binary.Read(r, binary.LittleEndian, &p.Nonce)
	err = binary.Read(r, binary.LittleEndian, &p.UserAgent)
	err = binary.Read(r, binary.LittleEndian, &p.StartHeight)
	err = binary.Read(r, binary.LittleEndian, &p.Relay)

	return err
}

// EncodeBinary implements the Payload interface.
func (p *Version) EncodeBinary(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, p.Version)
	err = binary.Write(w, binary.LittleEndian, p.Services)
	err = binary.Write(w, binary.LittleEndian, p.Timestamp)
	err = binary.Write(w, binary.LittleEndian, p.Port)
	err = binary.Write(w, binary.LittleEndian, p.Nonce)
	err = binary.Write(w, binary.LittleEndian, p.UserAgent)
	err = binary.Write(w, binary.LittleEndian, p.StartHeight)
	err = binary.Write(w, binary.LittleEndian, p.Relay)

	return err
}

// Size implements the payloader interface.
func (p *Version) Size() uint32 {
	return uint32(minVersionSize + len(p.UserAgent))
}

func uaToByteArray(ua string) [lenUA]byte {
	buf := [lenUA]byte{}
	for i := 0; i < lenUA; i++ {
		buf[i] = ua[i]
	}
	return buf
}
