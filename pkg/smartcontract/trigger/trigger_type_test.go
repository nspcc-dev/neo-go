package trigger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringer(t *testing.T) {
	tests := map[Type]string{
		OnPersist:    "OnPersist",
		PostPersist:  "PostPersist",
		Application:  "Application",
		Verification: "Verification",
	}
	for o, s := range tests {
		assert.Equal(t, s, o.String())
	}
}

func TestEncodeBynary(t *testing.T) {
	tests := map[Type]byte{
		OnPersist:    0x01,
		PostPersist:  0x02,
		Verification: 0x20,
		Application:  0x40,
	}
	for o, b := range tests {
		assert.Equal(t, b, byte(o))
	}
}

func TestDecodeBynary(t *testing.T) {
	tests := map[Type]byte{
		OnPersist:    0x01,
		PostPersist:  0x02,
		Verification: 0x20,
		Application:  0x40,
	}
	for o, b := range tests {
		assert.Equal(t, o, Type(b))
	}
}

func TestFromString(t *testing.T) {
	testCases := map[string]Type{
		"OnPersist":    OnPersist,
		"PostPersist":  PostPersist,
		"Application":  Application,
		"Verification": Verification,
		"All":          All,
	}
	for str, expected := range testCases {
		actual, err := FromString(str)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	errorCases := []string{
		"",
		"Unknown",
	}
	for _, str := range errorCases {
		_, err := FromString(str)
		require.Error(t, err)
	}
}
