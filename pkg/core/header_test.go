package core

import (
	"bytes"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

func TestHeaderEncodeDecode(t *testing.T) {
	header := Header{BlockBase: BlockBase{
		Version:       0,
		PrevHash:      sha256.Sum256([]byte("prevhash")),
		MerkleRoot:    sha256.Sum256([]byte("merkleroot")),
		Timestamp:     uint32(time.Now().UTC().Unix()),
		Index:         3445,
		ConsensusData: 394949,
		NextConsensus: util.Uint160{},
		Script: &transaction.Witness{
			InvocationScript:   []byte{0x10},
			VerificationScript: []byte{0x11},
		},
	}}

	buf := new(bytes.Buffer)
	if err := header.EncodeBinary(buf); err != nil {
		t.Fatal(err)
	}

	headerDecode := &Header{}
	if err := headerDecode.DecodeBinary(buf); err != nil {
		t.Fatal(err)
	}
	if header.Version != headerDecode.Version {
		t.Fatal("expected both versions to be equal")
	}
	if !header.PrevHash.Equals(headerDecode.PrevHash) {
		t.Fatal("expected both prev hashes to be equal")
	}
	if !header.MerkleRoot.Equals(headerDecode.MerkleRoot) {
		t.Fatal("expected both merkle roots to be equal")
	}
	if header.Index != headerDecode.Index {
		t.Fatal("expected both indexes to be equal")
	}
	if header.ConsensusData != headerDecode.ConsensusData {
		t.Fatal("expected both consensus data fields to be equal")
	}
	if !header.NextConsensus.Equals(headerDecode.NextConsensus) {
		t.Fatalf("expected both next consensus fields to be equal")
	}
	if bytes.Compare(header.Script.InvocationScript, headerDecode.Script.InvocationScript) != 0 {
		t.Fatalf("expected equal invocation scripts %v and %v", header.Script.InvocationScript, headerDecode.Script.InvocationScript)
	}
	if bytes.Compare(header.Script.VerificationScript, headerDecode.Script.VerificationScript) != 0 {
		t.Fatalf("expected equal verification scripts %v and %v", header.Script.VerificationScript, headerDecode.Script.VerificationScript)
	}
}
