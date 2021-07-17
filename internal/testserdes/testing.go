package testserdes

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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

// ToFromStackItem checks if expected stays the same after converting to/from
// StackItem.
func ToFromStackItem(t *testing.T, expected, actual stackitem.Convertible) {
	item, err := expected.ToStackItem()
	require.NoError(t, err)
	require.NoError(t, actual.FromStackItem(item))
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

type encodable interface {
	Encode(*io.BinWriter) error
	Decode(*io.BinReader) error
}

// EncodeDecode checks if expected stays the same after
// serializing/deserializing via encodable methods.
func EncodeDecode(t *testing.T, expected, actual encodable) {
	data, err := Encode(expected)
	require.NoError(t, err)
	require.NoError(t, Decode(data, actual))
	require.Equal(t, expected, actual)
}

// Encode serializes a to a byte slice.
func Encode(a encodable) ([]byte, error) {
	w := io.NewBufBinWriter()
	err := a.Encode(w.BinWriter)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

// Decode deserializes a from a byte slice.
func Decode(data []byte, a encodable) error {
	r := io.NewBinReaderFromBuf(data)
	err := a.Decode(r)
	if r.Err != nil {
		return r.Err
	}
	return err
}
