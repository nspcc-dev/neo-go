package testserdes

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/require"
)

// MarshalUnmarshalJSON checks if expected stays the same after
// marshal/unmarshal via JSON.
func MarshalUnmarshalJSON(t *testing.T, expected, actual interface{}) {
	data, err := json.Marshal(expected)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, actual))
	require.Equal(t, expected, actual)
}

// EncodeDecodeBinary checks if expected stays the same after
// serializing/deserializing via io.Serializable methods.
func EncodeDecodeBinary(t *testing.T, expected, actual io.Serializable) {
	data, err := EncodeBinary(expected)
	require.NoError(t, err)
	require.NoError(t, DecodeBinary(data, actual))
	require.Equal(t, expected, actual)
}

// EncodeBinary serializes a to a byte slice.
func EncodeBinary(a io.Serializable) ([]byte, error) {
	w := io.NewBufBinWriter()
	a.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		return nil, w.Err
	}
	return w.Bytes(), nil
}

// DecodeBinary deserializes a from a byte slice.
func DecodeBinary(data []byte, a io.Serializable) error {
	r := io.NewBinReaderFromBuf(data)
	a.DecodeBinary(r)
	return r.Err
}
