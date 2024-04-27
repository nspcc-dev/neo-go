package transaction

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func TestWitnessSerDes(t *testing.T) {
	var good1 = &Witness{
		InvocationScript:   make([]byte, 64),
		VerificationScript: make([]byte, 32),
	}
	var good2 = &Witness{
		InvocationScript:   make([]byte, MaxInvocationScript),
		VerificationScript: make([]byte, MaxVerificationScript),
	}
	var bad1 = &Witness{
		InvocationScript:   make([]byte, MaxInvocationScript+1),
		VerificationScript: make([]byte, 32),
	}
	var bad2 = &Witness{
		InvocationScript:   make([]byte, 128),
		VerificationScript: make([]byte, MaxVerificationScript+1),
	}
	var exp = new(Witness)
	testserdes.MarshalUnmarshalJSON(t, good1, exp)
	testserdes.MarshalUnmarshalJSON(t, good2, exp)
	testserdes.EncodeDecodeBinary(t, good1, exp)
	testserdes.EncodeDecodeBinary(t, good2, exp)
	bin1, err := testserdes.EncodeBinary(bad1)
	require.NoError(t, err)
	bin2, err := testserdes.EncodeBinary(bad2)
	require.NoError(t, err)
	require.Error(t, testserdes.DecodeBinary(bin1, exp))
	require.Error(t, testserdes.DecodeBinary(bin2, exp))
}

func TestWitnessCopy(t *testing.T) {
	original := &Witness{
		InvocationScript:   []byte{1, 2, 3},
		VerificationScript: []byte{3, 2, 1},
	}

	cp := original.Copy()
	require.Equal(t, *original, cp)

	original.InvocationScript[0] = 0x05
	require.NotEqual(t, *original, cp)
}
