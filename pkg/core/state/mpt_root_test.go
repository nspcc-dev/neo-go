package state

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
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

func TestMPTRoot_MarshalJSON(t *testing.T) {
	t.Run("Good", func(t *testing.T) {
		r := testStateRoot()
		rs := &MPTRootState{
			MPTRoot: *r,
			Flag:    Verified,
		}
		testserdes.MarshalUnmarshalJSON(t, rs, new(MPTRootState))
	})

	t.Run("Compatibility", func(t *testing.T) {
		js := []byte(`{
        "flag": "Unverified",
        "stateroot": {
            "version": 1,
            "index": 3000000,
            "prehash": "0x4f30f43af8dd2262fc331c45bfcd9066ebbacda204e6e81371cbd884fe7d6c90",
            "stateroot": "0xb2fd7e368a848ef70d27cf44940a35237333ed05f1d971c9408f0eb285e0b6f3"
        }}`)

		rs := new(MPTRootState)
		require.NoError(t, json.Unmarshal(js, &rs))

		require.EqualValues(t, 1, rs.Version)
		require.EqualValues(t, 3000000, rs.Index)
		require.Nil(t, rs.Witness)

		u, err := util.Uint256DecodeStringLE("4f30f43af8dd2262fc331c45bfcd9066ebbacda204e6e81371cbd884fe7d6c90")
		require.NoError(t, err)
		require.Equal(t, u, rs.PrevHash)

		u, err = util.Uint256DecodeStringLE("b2fd7e368a848ef70d27cf44940a35237333ed05f1d971c9408f0eb285e0b6f3")
		require.NoError(t, err)
		require.Equal(t, u, rs.Root)
	})
}
