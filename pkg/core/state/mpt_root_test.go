package state

import (
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/stretchr/testify/require"
)

func testStateRoot() *MPTRoot {
	return &MPTRoot{
		MPTRootBase: MPTRootBase{
			Version:  byte(rand.Uint32()),
			Index:    rand.Uint32(),
			PrevHash: random.Uint256(),
			Root:     random.Uint256(),
		},
	}
}

func TestStateRoot_Serializable(t *testing.T) {
	r := testStateRoot()
	testserdes.EncodeDecodeBinary(t, r, new(MPTRoot))

	t.Run("WithWitness", func(t *testing.T) {
		r.Witness = &transaction.Witness{
			InvocationScript:   random.Bytes(10),
			VerificationScript: random.Bytes(11),
		}
		testserdes.EncodeDecodeBinary(t, r, new(MPTRoot))
	})
}

func TestStateRootEquals(t *testing.T) {
	r1 := testStateRoot()
	r2 := *r1
	require.True(t, r1.Equals(&r2.MPTRootBase))

	r2.MPTRootBase.Index++
	require.False(t, r1.Equals(&r2.MPTRootBase))
}

func TestMPTRootState_Serializable(t *testing.T) {
	rs := &MPTRootState{
		MPTRoot: *testStateRoot(),
		Flag:    0x04,
	}
	rs.MPTRoot.Witness = &transaction.Witness{
		InvocationScript:   random.Bytes(10),
		VerificationScript: random.Bytes(11),
	}
	testserdes.EncodeDecodeBinary(t, rs, new(MPTRootState))
}

func TestMPTRootStateUnverifiedByDefault(t *testing.T) {
	var r MPTRootState
	require.Equal(t, Unverified, r.Flag)
}
