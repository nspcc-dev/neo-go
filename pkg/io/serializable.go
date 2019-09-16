package io

// Serializable defines the binary encoding/decoding interface.
type Serializable interface {
	DecodeBinary(*BinReader) error
	EncodeBinary(*BinWriter) error
}
