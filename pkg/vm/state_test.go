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
	assert.Equal(t, HaltState, s)

	s, err = StateFromString("BREAK")
	assert.NoError(t, err)
	assert.Equal(t, BreakState, s)

	s, err = StateFromString("FAULT")
	assert.NoError(t, err)
	assert.Equal(t, FaultState, s)

	s, err = StateFromString("NONE")
	assert.NoError(t, err)
	assert.Equal(t, NoneState, s)

	s, err = StateFromString("HALT, BREAK")
	assert.NoError(t, err)
	assert.Equal(t, HaltState|BreakState, s)

	s, err = StateFromString("FAULT, BREAK")
	assert.NoError(t, err)
	assert.Equal(t, FaultState|BreakState, s)

	_, err = StateFromString("HALT, KEK")
	assert.Error(t, err)
}

func TestState_HasFlag(t *testing.T) {
	assert.True(t, HaltState.HasFlag(HaltState))
	assert.True(t, BreakState.HasFlag(BreakState))
	assert.True(t, FaultState.HasFlag(FaultState))
	assert.True(t, (HaltState | BreakState).HasFlag(HaltState))
	assert.True(t, (HaltState | BreakState).HasFlag(BreakState))

	assert.False(t, HaltState.HasFlag(BreakState))
	assert.False(t, NoneState.HasFlag(HaltState))
	assert.False(t, (FaultState | BreakState).HasFlag(HaltState))
}

func TestState_MarshalJSON(t *testing.T) {
	var (
		data []byte
		err  error
	)

	data, err = json.Marshal(HaltState | BreakState)
	assert.NoError(t, err)
	assert.Equal(t, data, []byte(`"HALT, BREAK"`))

	data, err = json.Marshal(FaultState)
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
	assert.Equal(t, HaltState|BreakState, s)

	err = json.Unmarshal([]byte(`"FAULT, BREAK"`), &s)
	assert.NoError(t, err)
	assert.Equal(t, FaultState|BreakState, s)

	err = json.Unmarshal([]byte(`"NONE"`), &s)
	assert.NoError(t, err)
	assert.Equal(t, NoneState, s)
}

// TestState_EnumCompat tests that byte value of State matches the C#'s one got from
// https://github.com/neo-project/neo-vm/blob/0028d862e253bda3c12eb8bb007a2d95822d3922/src/neo-vm/VMState.cs#L16.
func TestState_EnumCompat(t *testing.T) {
	assert.Equal(t, byte(0), byte(NoneState))
	assert.Equal(t, byte(1<<0), byte(HaltState))
	assert.Equal(t, byte(1<<1), byte(FaultState))
	assert.Equal(t, byte(1<<2), byte(BreakState))
}
