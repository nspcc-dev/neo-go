package core

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestGetAndUpdate(t *testing.T) {
	store := storage.NewMemoryStore()
	state1 := getDefaultValidator()
	state2 := getDefaultValidator()
	validators := make(Validators)
	validators[state1.PublicKey] = state1
	validators[state2.PublicKey] = state2
	err := validators.commit(store)
	require.NoError(t, err)

	state, err := validators.getAndUpdate(store, state1.PublicKey)
	require.NoError(t, err)
	require.Equal(t, state1, state)
}

func TestCommit(t *testing.T) {
	store := storage.NewMemoryStore()
	state1 := getDefaultValidator()
	state2 := getDefaultValidator()
	validators := make(Validators)
	validators[state1.PublicKey] = state1
	validators[state2.PublicKey] = state2
	err := validators.commit(store)
	require.NoError(t, err)

	validatorsFromStore := getValidatorsFromStore(store)
	// 2 equal validators will be stored as 1 unique
	require.Len(t, validatorsFromStore, 1)
	require.Equal(t, state1, validatorsFromStore[0])
}

func TestPutAndGet(t *testing.T) {
	store := storage.NewMemoryStore()
	state := getDefaultValidator()
	err := putValidatorStateIntoStore(store, state)
	require.NoError(t, err)
	validatorFromStore, err := getValidatorStateFromStore(store, state.PublicKey)
	require.NoError(t, err)
	require.Equal(t, state.PublicKey, validatorFromStore.PublicKey)
}

func TestGetFromStore_NoKey(t *testing.T) {
	store := storage.NewMemoryStore()
	state := getDefaultValidator()
	_, err := getValidatorStateFromStore(store, state.PublicKey)
	require.Errorf(t, err, "key not found")
}

func TestValidatorState_DecodeEncodeBinary(t *testing.T) {
	state := &ValidatorState{
		PublicKey:  &keys.PublicKey{},
		Registered: false,
		Votes:      util.Fixed8(10),
	}
	buf := io.NewBufBinWriter()
	state.EncodeBinary(buf.BinWriter)
	require.NoError(t, buf.Err)

	decodedState := &ValidatorState{}
	reader := io.NewBinReaderFromBuf(buf.Bytes())
	decodedState.DecodeBinary(reader)
	require.NoError(t, reader.Err)
	require.Equal(t, state, decodedState)
}

func TestRegisteredAndHasVotes_Registered(t *testing.T) {
	state := &ValidatorState{
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
	state := &ValidatorState{
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
	state := &ValidatorState{
		PublicKey: &keys.PublicKey{
			X: big.NewInt(1),
			Y: big.NewInt(1),
		},
		Registered: false,
		Votes:      1,
	}
	require.False(t, state.RegisteredAndHasVotes())
}

func getDefaultValidator() *ValidatorState {
	return &ValidatorState{
		PublicKey:  &keys.PublicKey{},
		Registered: false,
		Votes:      0,
	}
}
