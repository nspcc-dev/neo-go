package capability

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

const (
	// MaxCapabilities is the maximum number of capabilities per payload.
	MaxCapabilities = 32

	// MaxDataSize is the maximum size of capability payload.
	MaxDataSize = 1024
)

// Capabilities is a list of Capability.
type Capabilities []Capability

// IsArchivalNode denotes whether the node has Archival capability.
func (cs *Capabilities) IsArchivalNode() bool {
	for _, c := range *cs {
		if c.Type == ArchivalNode {
			return true
		}
	}
	return false
}

// DecodeBinary implements io.Serializable.
func (cs *Capabilities) DecodeBinary(br *io.BinReader) {
	br.ReadArray(cs, MaxCapabilities)
	if br.Err == nil {
		br.Err = cs.checkUniqueCapabilities()
	}
}

// EncodeBinary implements io.Serializable.
func (cs *Capabilities) EncodeBinary(br *io.BinWriter) {
	br.WriteArray(*cs)
}

// checkUniqueCapabilities checks whether payload capabilities have a unique type.
func (cs Capabilities) checkUniqueCapabilities() error {
	err := errors.New("capabilities with the same type are not allowed")
	var isFullNode, isArchived, isTCP, isWS, isDisabledCompression bool
	for _, cap := range cs {
		switch cap.Type {
		case ArchivalNode:
			if isArchived {
				return err
			}
			isArchived = true
		case FullNode:
			if isFullNode {
				return err
			}
			isFullNode = true
		case DisableCompressionNode:
			if isDisabledCompression {
				return err
			}
			isDisabledCompression = true
		case TCPServer:
			if isTCP {
				return err
			}
			isTCP = true
		case WSServer:
			if isWS {
				return err
			}
			isWS = true
		default: /* OK to have duplicates */
		}
	}
	return nil
}

// Capability describes a network service available for the node.
type Capability struct {
	Type Type
	Data io.Serializable
}

// DecodeBinary implements io.Serializable.
func (c *Capability) DecodeBinary(br *io.BinReader) {
	c.Type = Type(br.ReadB())
	switch c.Type {
	case ArchivalNode:
		c.Data = &Archival{}
	case FullNode:
		c.Data = &Node{}
	case DisableCompressionNode:
		c.Data = &DisableCompression{}
	case TCPServer, WSServer:
		c.Data = &Server{}
	default:
		c.Data = &Unknown{}
	}
	c.Data.DecodeBinary(br)
}

// EncodeBinary implements io.Serializable.
func (c *Capability) EncodeBinary(bw *io.BinWriter) {
	if c.Data == nil {
		bw.Err = errors.New("capability has no data")
		return
	}
	bw.WriteB(byte(c.Type))
	c.Data.EncodeBinary(bw)
}

// Node represents full node capability with a start height.
type Node struct {
	StartHeight uint32
}

// DecodeBinary implements io.Serializable.
func (n *Node) DecodeBinary(br *io.BinReader) {
	n.StartHeight = br.ReadU32LE()
}

// EncodeBinary implements io.Serializable.
func (n *Node) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(n.StartHeight)
}

// Server represents TCP or WS server capability with a port.
type Server struct {
	// Port is the port this server is listening on.
	Port uint16
}

// DecodeBinary implements io.Serializable.
func (s *Server) DecodeBinary(br *io.BinReader) {
	s.Port = br.ReadU16LE()
}

// EncodeBinary implements io.Serializable.
func (s *Server) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU16LE(s.Port)
}

// Archival represents an archival node that stores all blocks.
type Archival struct{}

// DecodeBinary implements io.Serializable.
func (a *Archival) DecodeBinary(br *io.BinReader) {
	var zero = br.ReadB() // Zero-length byte array as per Unknown.
	if zero != 0 {
		br.Err = errors.New("archival capability with non-zero data")
	}
}

// EncodeBinary implements io.Serializable.
func (a *Archival) EncodeBinary(bw *io.BinWriter) {
	bw.WriteB(0)
}

// DisableCompression represents the node that doesn't compress any P2P payloads.
type DisableCompression struct{}

// DecodeBinary implements io.Serializable.
func (d *DisableCompression) DecodeBinary(br *io.BinReader) {
	var zero = br.ReadB() // Zero-length byte array as per Unknown.
	if zero != 0 {
		br.Err = errors.New("DisableCompression capability with non-zero data")
	}
}

// EncodeBinary implements io.Serializable.
func (d *DisableCompression) EncodeBinary(bw *io.BinWriter) {
	bw.WriteB(0)
}

// Unknown represents an unknown capability with some data. Other nodes can
// decode it even if they can't interpret it. This is not expected to be used
// for sending data directly (proper new types should be used), but it allows
// for easier protocol extensibility (old nodes won't reject new capabilities).
type Unknown []byte

// DecodeBinary implements io.Serializable.
func (u *Unknown) DecodeBinary(br *io.BinReader) {
	*u = br.ReadVarBytes()
}

// EncodeBinary implements io.Serializable.
func (u *Unknown) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarBytes(*u)
}
