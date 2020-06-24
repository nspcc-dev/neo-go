package payload

import (
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
)

func TestStateRoots_Serializable(t *testing.T) {
	expected := &StateRoots{
		Roots: []state.MPTRoot{
			{
				MPTRootBase: state.MPTRootBase{
					Index:    rand.Uint32(),
					PrevHash: random.Uint256(),
					Root:     random.Uint256(),
				},
				Witness: &transaction.Witness{
					InvocationScript:   random.Bytes(10),
					VerificationScript: random.Bytes(11),
				},
			},
			{
				MPTRootBase: state.MPTRootBase{
					Index:    rand.Uint32(),
					PrevHash: random.Uint256(),
					Root:     random.Uint256(),
				},
				Witness: &transaction.Witness{
					InvocationScript:   random.Bytes(10),
					VerificationScript: random.Bytes(11),
				},
			},
		},
	}

	testserdes.EncodeDecodeBinary(t, expected, new(StateRoots))
}

func TestGetStateRoots_Serializable(t *testing.T) {
	expected := &GetStateRoots{
		Start: rand.Uint32(),
		Count: rand.Uint32(),
	}

	testserdes.EncodeDecodeBinary(t, expected, new(GetStateRoots))
}
