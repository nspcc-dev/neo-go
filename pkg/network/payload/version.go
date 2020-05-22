package payload

import (
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/capability"
)

// Version payload.
type Version struct {
	// NetMode of the node
	Magic config.NetMode
	// currently the version of the protocol is 0
	Version uint32
	// timestamp
	Timestamp uint32
	// it's used to distinguish the node from public IP
	Nonce uint32
	// client id
	UserAgent []byte
	// List of available network services
	Capabilities capability.Capabilities
}

// NewVersion returns a pointer to a Version payload.
func NewVersion(magic config.NetMode, id uint32, ua string, c []capability.Capability) *Version {
	return &Version{
		Magic:        magic,
		Version:      0,
		Timestamp:    uint32(time.Now().UTC().Unix()),
		Nonce:        id,
		UserAgent:    []byte(ua),
		Capabilities: c,
	}
}

// DecodeBinary implements Serializable interface.
func (p *Version) DecodeBinary(br *io.BinReader) {
	p.Magic = config.NetMode(br.ReadU32LE())
	p.Version = br.ReadU32LE()
	p.Timestamp = br.ReadU32LE()
	p.Nonce = br.ReadU32LE()
	p.UserAgent = br.ReadVarBytes()
	p.Capabilities.DecodeBinary(br)
}

// EncodeBinary implements Serializable interface.
func (p *Version) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(uint32(p.Magic))
	bw.WriteU32LE(p.Version)
	bw.WriteU32LE(p.Timestamp)
	bw.WriteU32LE(p.Nonce)
	bw.WriteVarBytes(p.UserAgent)
	p.Capabilities.EncodeBinary(bw)
}
