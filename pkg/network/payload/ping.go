package payload

import (
	"time"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Ping payload for ping/pong payloads.
type Ping struct {
	// Index of the last block.
	LastBlockIndex uint32
	// Timestamp.
	Timestamp uint32
	// Nonce of the server.
	Nonce uint32
}

// NewPing creates new Ping payload.
func NewPing(blockIndex uint32, nonce uint32) *Ping {
	return &Ping{
		LastBlockIndex: blockIndex,
		Timestamp:      uint32(time.Now().UTC().Unix()),
		Nonce:          nonce,
	}
}

// DecodeBinary implements Serializable interface.
func (p *Ping) DecodeBinary(br *io.BinReader) {
	p.LastBlockIndex = br.ReadU32LE()
	p.Timestamp = br.ReadU32LE()
	p.Nonce = br.ReadU32LE()
}

// EncodeBinary implements Serializable interface.
func (p *Ping) EncodeBinary(bw io.BinaryWriter) {
	bw.WriteU32LE(p.LastBlockIndex)
	bw.WriteU32LE(p.Timestamp)
	bw.WriteU32LE(p.Nonce)
}
