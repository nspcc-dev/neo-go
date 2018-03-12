package payload

import (
	"encoding/binary"
	"io"
	"time"
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
	// client id
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
		Timestamp:   uint32(time.Now().UTC().Unix()),
		Port:        p,
		Nonce:       id,
		UserAgent:   []byte(ua),
		StartHeight: h,
		Relay:       r,
	}
}

// DecodeBinary implements the Payload interface.
func (p *Version) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &p.Version); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &p.Services); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &p.Timestamp); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &p.Port); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &p.Nonce); err != nil {
		return err
	}

	var lenUA uint8
	if err := binary.Read(r, binary.LittleEndian, &lenUA); err != nil {
		return err
	}
	p.UserAgent = make([]byte, lenUA)
	if err := binary.Read(r, binary.LittleEndian, &p.UserAgent); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &p.StartHeight); err != nil {
		return err
	}
	return binary.Read(r, binary.LittleEndian, &p.Relay)
}

// EncodeBinary implements the Payload interface.
func (p *Version) EncodeBinary(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, p.Version); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.Services); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.Timestamp); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.Port); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.Nonce); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint8(len(p.UserAgent))); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.UserAgent); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.StartHeight); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, p.Relay)
}

// Size implements the payloader interface.
func (p *Version) Size() uint32 {
	return uint32(minVersionSize + len(p.UserAgent))
}
