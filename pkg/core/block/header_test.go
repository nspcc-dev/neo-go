package block

import (
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestHeaderEncodeDecode(t *testing.T) {
	header := Header{Base: Base{
		Version:       0,
		PrevHash:      hash.Sha256([]byte("prevhash")),
		MerkleRoot:    hash.Sha256([]byte("merkleroot")),
		Timestamp:     uint64(time.Now().UTC().Unix() * 1000),
		Index:         3445,
		ConsensusData: 394949,
		NextConsensus: util.Uint160{},
		Script: transaction.Witness{
			InvocationScript:   []byte{0x10},
			VerificationScript: []byte{0x11},
		},
	}}

	_ = header.Hash()
	headerDecode := &Header{}
	testserdes.EncodeDecodeBinary(t, &header, headerDecode)

	assert.Equal(t, header.Version, headerDecode.Version, "expected both versions to be equal")
	assert.Equal(t, header.PrevHash, headerDecode.PrevHash, "expected both prev hashes to be equal")
	assert.Equal(t, header.MerkleRoot, headerDecode.MerkleRoot, "expected both merkle roots to be equal")
	assert.Equal(t, header.Index, headerDecode.Index, "expected both indexes to be equal")
	assert.Equal(t, header.ConsensusData, headerDecode.ConsensusData, "expected both consensus data fields to be equal")
	assert.Equal(t, header.NextConsensus, headerDecode.NextConsensus, "expected both next consensus fields to be equal")
	assert.Equal(t, header.Script.InvocationScript, headerDecode.Script.InvocationScript, "expected equal invocation scripts")
	assert.Equal(t, header.Script.VerificationScript, headerDecode.Script.VerificationScript, "expected equal verification scripts")
}
