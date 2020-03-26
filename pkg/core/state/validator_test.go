package state

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestValidatorState_DecodeEncodeBinary(t *testing.T) {
	state := &Validator{
		PublicKey:  &keys.PublicKey{},
		Registered: false,
		Votes:      util.Fixed8(10),
	}

	testserdes.EncodeDecodeBinary(t, state, new(Validator))
}

func TestRegisteredAndHasVotes_Registered(t *testing.T) {
	state := &Validator{
		PublicKey: &keys.PublicKey{
			X: big.NewInt(1),
			Y: big.NewInt(1),
		},
		Registered: true,
		Votes:      0,
	}
	require.False(t, state.RegisteredAndHasVotes())
}

func TestRegisteredAndHasVotes_RegisteredWithVotes(t *testing.T) {
	state := &Validator{
		PublicKey: &keys.PublicKey{
			X: big.NewInt(1),
			Y: big.NewInt(1),
		},
		Registered: true,
		Votes:      1,
	}
	require.True(t, state.RegisteredAndHasVotes())
}

func TestRegisteredAndHasVotes_NotRegisteredWithVotes(t *testing.T) {
	state := &Validator{
		PublicKey: &keys.PublicKey{
			X: big.NewInt(1),
			Y: big.NewInt(1),
		},
		Registered: false,
		Votes:      1,
	}
	require.False(t, state.RegisteredAndHasVotes())
}
