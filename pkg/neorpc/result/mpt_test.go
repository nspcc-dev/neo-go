package result

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/require"
)

func testProofWithKey() *ProofWithKey {
	return &ProofWithKey{
		Key: random.Bytes(10),
		Proof: [][]byte{
			random.Bytes(12),
			random.Bytes(0),
			random.Bytes(34),
		},
	}
}

func TestGetProof_MarshalJSON(t *testing.T) {
	t.Run("Good", func(t *testing.T) {
		p := testProofWithKey()
		testserdes.MarshalUnmarshalJSON(t, p, new(ProofWithKey))
	})
	t.Run("Compatibility", func(t *testing.T) {
		js := []byte(`"Bfn///8SBiQBAQ8D6yfHa4wV24kQ9eXarzY5Bw55VFzysUbkJjrz5FipqkjSAAQEBAQEBAMcbFvhto6QJgYoJs/uzqTrZNrPxpkgNiF5Z/ME98copwPQ4q6ZqLA8S7XUXNCrJNF68vMu8Gx3W8Ooo3qwMomm0gQDiT6zHh/siCZ0c2bfBEymPmRNTiXSAKFIammjmnnBnJYD+CNwgcEzBJqYfnc7RMhr8cPhffKN0281w0M7XLQ9BO4D7W+t3cleDNdiNc6tqWR8jyIP+bolh5QnZIyKXPwGHjsEBAQDcpxkuWYJr6g3ilENTh1sztlZsXZvt6Eedmyy6kI2gQoEKQEGDw8PDw8PA33qzf1Q5ILAwmYxBnM2N80A8JtFHKR7UHhVEqo5nQ0eUgADbChDXdc7hSDZpD9xbhYGuJxVxRWqhsVRTR2dE+18gd4DG5gRFexXofB0aNb6G2kzQUSTD+aWVsfmnKGf4HHivzAEBAQEBAQEBAQEBAQEBARSAAQEA2IMPmRKP0b2BqhMB6IgtfpPeuXKJMdMze7Cr1TeJqbmA1vvqQgR5DN9ew+Zp/nc5SBQbjV5gEq7F/tIipWaQJ1hBAQEBAQEBAQEBAQEBAMCAR4="`)

		var p ProofWithKey
		require.NoError(t, json.Unmarshal(js, &p))
		require.Equal(t, 6, len(p.Proof))
		for i := range p.Proof { // smoke test that every chunk is correctly encoded node
			r := io.NewBinReaderFromBuf(p.Proof[i])
			var n mpt.NodeObject
			n.DecodeBinary(r)
			require.NoError(t, r.Err)
			require.NotNil(t, n.Node)
		}
	})
}

func TestProofWithKey_EncodeString(t *testing.T) {
	expected := testProofWithKey()
	var actual ProofWithKey
	require.NoError(t, actual.FromString(expected.String()))
	require.Equal(t, expected, &actual)
}

func TestVerifyProof_MarshalJSON(t *testing.T) {
	t.Run("Good", func(t *testing.T) {
		vp := &VerifyProof{random.Bytes(100)}
		testserdes.MarshalUnmarshalJSON(t, vp, new(VerifyProof))
	})
	t.Run("NoValue", func(t *testing.T) {
		vp := new(VerifyProof)
		testserdes.MarshalUnmarshalJSON(t, vp, &VerifyProof{[]byte{1, 2, 3}})
	})
}
