package vmstate

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromString(t *testing.T) {
	var (
		s   State
		err error
	)

	s, err = FromString("HALT")
	assert.NoError(t, err)
	assert.Equal(t, Halt, s)

	s, err = FromString("BREAK")
	assert.NoError(t, err)
	assert.Equal(t, Break, s)

	s, err = FromString("FAULT")
	assert.NoError(t, err)
	assert.Equal(t, Fault, s)

	s, err = FromString("NONE")
	assert.NoError(t, err)
	assert.Equal(t, None, s)

	s, err = FromString("HALT, BREAK")
	assert.NoError(t, err)
	assert.Equal(t, Halt|Break, s)

	s, err = FromString("FAULT, BREAK")
	assert.NoError(t, err)
	assert.Equal(t, Fault|Break, s)

	_, err = FromString("HALT, KEK")
	assert.Error(t, err)
}

func TestState_HasFlag(t *testing.T) {
	assert.True(t, Halt.HasFlag(Halt))
	assert.True(t, Break.HasFlag(Break))
	assert.True(t, Fault.HasFlag(Fault))
	assert.True(t, (Halt | Break).HasFlag(Halt))
	assert.True(t, (Halt | Break).HasFlag(Break))

	assert.False(t, Halt.HasFlag(Break))
	assert.False(t, None.HasFlag(Halt))
	assert.False(t, (Fault | Break).HasFlag(Halt))
}

func TestState_MarshalJSON(t *testing.T) {
	var (
		data []byte
		err  error
	)

	data, err = json.Marshal(Halt | Break)
	assert.NoError(t, err)
	assert.Equal(t, data, []byte(`"HALT, BREAK"`))

	data, err = json.Marshal(Fault)
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
	assert.Equal(t, Halt|Break, s)

	err = json.Unmarshal([]byte(`"FAULT, BREAK"`), &s)
	assert.NoError(t, err)
	assert.Equal(t, Fault|Break, s)

	err = json.Unmarshal([]byte(`"NONE"`), &s)
	assert.NoError(t, err)
	assert.Equal(t, None, s)
}

// TestState_EnumCompat tests that byte value of State matches the C#'s one got from
// https://github.com/neo-project/neo-vm/blob/0028d862e253bda3c12eb8bb007a2d95822d3922/src/neo-vm/VMState.cs#L16.
func TestState_EnumCompat(t *testing.T) {
	assert.Equal(t, byte(0), byte(None))
	assert.Equal(t, byte(1<<0), byte(Halt))
	assert.Equal(t, byte(1<<1), byte(Fault))
	assert.Equal(t, byte(1<<2), byte(Break))
}
