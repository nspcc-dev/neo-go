package state

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func testStateRoot() *MPTRoot {
	return &MPTRoot{
		Version: byte(rand.Uint32()),
		Index:   rand.Uint32(),
		Root:    random.Uint256(),
		Witness: []transaction.Witness{},
	}
}

func TestStateRoot_Serializable(t *testing.T) {
	r := testStateRoot()
	testserdes.EncodeDecodeBinary(t, r, new(MPTRoot))

	t.Run("WithWitness", func(t *testing.T) {
		r.Witness = []transaction.Witness{{
			InvocationScript:   random.Bytes(10),
			VerificationScript: random.Bytes(11),
		}}
		testserdes.EncodeDecodeBinary(t, r, new(MPTRoot))
	})
}

func TestMPTRoot_MarshalJSON(t *testing.T) {
	t.Run("Good", func(t *testing.T) {
		r := testStateRoot()
		testserdes.MarshalUnmarshalJSON(t, r, new(MPTRoot))
	})

	t.Run("Compatibility", func(t *testing.T) {
		js := []byte(`{
            "version": 1,
            "index": 3000000,
            "roothash": "0xb2fd7e368a848ef70d27cf44940a35237333ed05f1d971c9408f0eb285e0b6f3"
        }`)

		rs := new(MPTRoot)
		require.NoError(t, json.Unmarshal(js, &rs))

		require.EqualValues(t, 1, rs.Version)
		require.EqualValues(t, 3000000, rs.Index)
		require.Equal(t, 0, len(rs.Witness))

		u, err := util.Uint256DecodeStringLE("b2fd7e368a848ef70d27cf44940a35237333ed05f1d971c9408f0eb285e0b6f3")
		require.NoError(t, err)
		require.Equal(t, u, rs.Root)
	})
}
