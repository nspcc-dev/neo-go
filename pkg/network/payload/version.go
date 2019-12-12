package payload

import (
	"time"

	"github.com/CityOfZion/neo-go/pkg/io"
)

// Size of the payload not counting UserAgent encoding (which is at least 1 byte
// for zero-length string).
const minVersionSize = 27

// List of Services offered by the node.
const (
	nodePeerService uint64 = 1
	// BloomFilerService uint64 = 2 // Not implemented
	// PrunedNode        uint64 = 3 // Not implemented
	// LightNode         uint64 = 4 // Not implemented

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
		Services:    nodePeerService,
		Timestamp:   uint32(time.Now().UTC().Unix()),
		Port:        p,
		Nonce:       id,
		UserAgent:   []byte(ua),
		StartHeight: h,
		Relay:       r,
	}
}

// DecodeBinary implements Serializable interface.
func (p *Version) DecodeBinary(br *io.BinReader) {
	p.Version = br.ReadU32LE()
	p.Services = br.ReadU64LE()
	p.Timestamp = br.ReadU32LE()
	p.Port = br.ReadU16LE()
	p.Nonce = br.ReadU32LE()
	p.UserAgent = br.ReadVarBytes()
	p.StartHeight = br.ReadU32LE()
	p.Relay = br.ReadBool()
}

// EncodeBinary implements Serializable interface.
func (p *Version) EncodeBinary(br *io.BinWriter) {
	br.WriteU32LE(p.Version)
	br.WriteU64LE(p.Services)
	br.WriteU32LE(p.Timestamp)
	br.WriteU16LE(p.Port)
	br.WriteU32LE(p.Nonce)

	br.WriteVarBytes(p.UserAgent)
	br.WriteU32LE(p.StartHeight)
	br.WriteBool(p.Relay)
}
