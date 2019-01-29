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
	assert.Equal(t, nil, err)
	assert.Equal(t, haltState, s)

	s, err = StateFromString("BREAK")
	assert.Equal(t, nil, err)
	assert.Equal(t, breakState, s)

	s, err = StateFromString("FAULT")
	assert.Equal(t, nil, err)
	assert.Equal(t, faultState, s)

	s, err = StateFromString("NONE")
	assert.Equal(t, nil, err)
	assert.Equal(t, noneState, s)

	s, err = StateFromString("HALT, BREAK")
	assert.Equal(t, nil, err)
	assert.Equal(t, haltState|breakState, s)

	s, err = StateFromString("FAULT, BREAK")
	assert.Equal(t, nil, err)
	assert.Equal(t, faultState|breakState, s)

	s, err = StateFromString("HALT, KEK")
	assert.NotEqual(t, nil, err)
}

func TestState_HasFlag(t *testing.T) {
	assert.Equal(t, true, haltState.HasFlag(haltState))
	assert.Equal(t, true, breakState.HasFlag(breakState))
	assert.Equal(t, true, faultState.HasFlag(faultState))
	assert.Equal(t, true, (haltState | breakState).HasFlag(haltState))
	assert.Equal(t, true, (haltState | breakState).HasFlag(breakState))

	assert.Equal(t, false, haltState.HasFlag(breakState))
	assert.Equal(t, false, noneState.HasFlag(haltState))
	assert.Equal(t, false, (faultState | breakState).HasFlag(haltState))
}

func TestState_UnmarshalJSON(t *testing.T) {
	var (
		s   State
		err error
	)

	err = json.Unmarshal([]byte(`"HALT, BREAK"`), &s)
	assert.Equal(t, nil, err)
	assert.Equal(t, haltState|breakState, s)

	err = json.Unmarshal([]byte(`"FAULT, BREAK"`), &s)
	assert.Equal(t, nil, err)
	assert.Equal(t, faultState|breakState, s)

	err = json.Unmarshal([]byte(`"NONE"`), &s)
	assert.Equal(t, nil, err)
	assert.Equal(t, noneState, s)
}
