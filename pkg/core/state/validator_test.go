package state

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestValidatorState_DecodeEncodeBinary(t *testing.T) {
	state := &Validator{
		PublicKey:  &keys.PublicKey{},
		Registered: false,
		Votes:      util.Fixed8(10),
	}
	buf := io.NewBufBinWriter()
	state.EncodeBinary(buf.BinWriter)
	require.NoError(t, buf.Err)

	decodedState := &Validator{}
	reader := io.NewBinReaderFromBuf(buf.Bytes())
	decodedState.DecodeBinary(reader)
	require.NoError(t, reader.Err)
	require.Equal(t, state, decodedState)
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
