package vm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStateFromString(t *testing.T) {
	var (
		s   State
		err error
	)

	s, err = StateFromString("HALT")
	assert.NoError(t, err)
	assert.Equal(t, haltState, s)

	s, err = StateFromString("BREAK")
	assert.NoError(t, err)
	assert.Equal(t, breakState, s)

	s, err = StateFromString("FAULT")
	assert.NoError(t, err)
	assert.Equal(t, faultState, s)

	s, err = StateFromString("NONE")
	assert.NoError(t, err)
	assert.Equal(t, noneState, s)

	s, err = StateFromString("HALT, BREAK")
	assert.NoError(t, err)
	assert.Equal(t, haltState|breakState, s)

	s, err = StateFromString("FAULT, BREAK")
	assert.NoError(t, err)
	assert.Equal(t, faultState|breakState, s)

	_, err = StateFromString("HALT, KEK")
	assert.Error(t, err)
}

func TestState_HasFlag(t *testing.T) {
	assert.True(t, haltState.HasFlag(haltState))
	assert.True(t, breakState.HasFlag(breakState))
	assert.True(t, faultState.HasFlag(faultState))
	assert.True(t, (haltState | breakState).HasFlag(haltState))
	assert.True(t, (haltState | breakState).HasFlag(breakState))

	assert.False(t, haltState.HasFlag(breakState))
	assert.False(t, noneState.HasFlag(haltState))
	assert.False(t, (faultState | breakState).HasFlag(haltState))
}

func TestState_MarshalJSON(t *testing.T) {
	var (
		data []byte
		err  error
	)

	data, err = json.Marshal(haltState | breakState)
	assert.NoError(t, err)
	assert.Equal(t, data, []byte(`"HALT, BREAK"`))

	data, err = json.Marshal(faultState)
	assert.NoError(t, err)
	assert.Equal(t, data, []byte(`"FAULT"`))
}

func TestState_UnmarshalJSON(t *testing.T) {
	var (
		s   State
		err error
	)

	err = json.Unmarshal([]byte(`"HALT, BREAK"`), &s)
	assert.NoError(t, err)
	assert.Equal(t, haltState|breakState, s)

	err = json.Unmarshal([]byte(`"FAULT, BREAK"`), &s)
	assert.NoError(t, err)
	assert.Equal(t, faultState|breakState, s)

	err = json.Unmarshal([]byte(`"NONE"`), &s)
	assert.NoError(t, err)
	assert.Equal(t, noneState, s)
}
