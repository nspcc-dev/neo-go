package block

import (
	"encoding/json"
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// ConsensusData represents primary index and nonce of block in the chain.
type ConsensusData struct {
	// Primary index
	PrimaryIndex uint32
	// Random number
	Nonce uint64
	// Hash of the ConsensusData (single SHA256)
	hash util.Uint256
}

// jsonConsensusData is used for JSON I/O of ConsensusData.
type jsonConsensusData struct {
	Primary uint32 `json:"primary"`
	Nonce   string `json:"nonce"`
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

// MarshalJSON implements json.Marshaler interface.
func (c ConsensusData) MarshalJSON() ([]byte, error) {
	nonce := strconv.FormatUint(c.Nonce, 16)
	for len(nonce) < 16 {
		nonce = "0" + nonce
	}
	return json.Marshal(jsonConsensusData{Primary: c.PrimaryIndex, Nonce: nonce})
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (c *ConsensusData) UnmarshalJSON(data []byte) error {
	jcd := new(jsonConsensusData)
	err := json.Unmarshal(data, jcd)
	if err != nil {
		return err
	}
	nonce, err := strconv.ParseUint(jcd.Nonce, 16, 64)
	if err != nil {
		return err
	}
	c.PrimaryIndex = jcd.Primary
	c.Nonce = nonce
	return nil
}
