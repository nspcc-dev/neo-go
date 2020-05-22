package block

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// ConsensusData represents primary index and nonce of block in the chain.
type ConsensusData struct {
	// Primary index
	PrimaryIndex uint32 `json:"primary"`
	// Random number
	Nonce uint64 `json:"nonce"`
	// Hash of the ConsensusData (single SHA256)
	hash util.Uint256
}

// DecodeBinary implements Serializable interface.
func (c *ConsensusData) DecodeBinary(br *io.BinReader) {
	c.PrimaryIndex = uint32(br.ReadVarUint())
	c.Nonce = br.ReadU64LE()
}

// EncodeBinary encodes implements Serializable interface.
func (c *ConsensusData) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarUint(uint64(c.PrimaryIndex))
	bw.WriteU64LE(c.Nonce)
}

// Hash returns the hash of the consensus data.
func (c *ConsensusData) Hash() util.Uint256 {
	if c.hash.Equals(util.Uint256{}) {
		if c.createHash() != nil {
			panic("failed to compute hash!")
		}
	}
	return c.hash
}

// createHash creates the hash of the consensus data.
func (c *ConsensusData) createHash() error {
	buf := io.NewBufBinWriter()
	c.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}

	b := buf.Bytes()
	c.hash = hash.Sha256(b)
	return nil
}
