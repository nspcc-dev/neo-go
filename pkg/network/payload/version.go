package payload

import (
	"io"
	"time"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Size of the payload not counting UserAgent encoding (which is at least 1 byte
// for zero-length string)
const minVersionSize = 27

// List of Services offered by the node
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

// DecodeBinary implements the Payload interface.
func (p *Version) DecodeBinary(r io.Reader) error {
	br := util.NewBinReaderFromIO(r)
	br.ReadLE(&p.Version)
	br.ReadLE(&p.Services)
	br.ReadLE(&p.Timestamp)
	br.ReadLE(&p.Port)
	br.ReadLE(&p.Nonce)
	p.UserAgent = br.ReadBytes()
	br.ReadLE(&p.StartHeight)
	br.ReadLE(&p.Relay)
	return br.Err
}

// EncodeBinary implements the Payload interface.
func (p *Version) EncodeBinary(w io.Writer) error {
	br := util.NewBinWriterFromIO(w)
	br.WriteLE(p.Version)
	br.WriteLE(p.Services)
	br.WriteLE(p.Timestamp)
	br.WriteLE(p.Port)
	br.WriteLE(p.Nonce)

	br.WriteBytes(p.UserAgent)
	br.WriteLE(p.StartHeight)
	br.WriteLE(&p.Relay)
	return br.Err
}

// Size implements the payloader interface.
func (p *Version) Size() uint32 {
	return uint32(minVersionSize + util.GetVarSize(p.UserAgent))
}
