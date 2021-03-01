package block

import (
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func testHeaderEncodeDecode(t *testing.T, stateRootEnabled bool) {
	header := Header{
		Version:       0,
		PrevHash:      hash.Sha256([]byte("prevhash")),
		MerkleRoot:    hash.Sha256([]byte("merkleroot")),
		Timestamp:     uint64(time.Now().UTC().Unix() * 1000),
		Index:         3445,
		NextConsensus: util.Uint160{},
		Script: transaction.Witness{
			InvocationScript:   []byte{0x10},
			VerificationScript: []byte{0x11},
		},
	}
	if stateRootEnabled {
		header.StateRootEnabled = stateRootEnabled
		header.PrevStateRoot = random.Uint256()
	}

	_ = header.Hash()
	headerDecode := &Header{StateRootEnabled: stateRootEnabled}
	testserdes.EncodeDecodeBinary(t, &header, headerDecode)

	assert.Equal(t, header.Version, headerDecode.Version, "expected both versions to be equal")
	assert.Equal(t, header.PrevHash, headerDecode.PrevHash, "expected both prev hashes to be equal")
	assert.Equal(t, header.MerkleRoot, headerDecode.MerkleRoot, "expected both merkle roots to be equal")
	assert.Equal(t, header.Index, headerDecode.Index, "expected both indexes to be equal")
	assert.Equal(t, header.NextConsensus, headerDecode.NextConsensus, "expected both next consensus fields to be equal")
	assert.Equal(t, header.Script.InvocationScript, headerDecode.Script.InvocationScript, "expected equal invocation scripts")
	assert.Equal(t, header.Script.VerificationScript, headerDecode.Script.VerificationScript, "expected equal verification scripts")
	assert.Equal(t, header.PrevStateRoot, headerDecode.PrevStateRoot, "expected equal state roots")
}

func TestHeaderEncodeDecode(t *testing.T) {
	t.Run("NoStateRoot", func(t *testing.T) {
		testHeaderEncodeDecode(t, false)
	})
	t.Run("WithStateRoot", func(t *testing.T) {
		testHeaderEncodeDecode(t, true)
	})
}
