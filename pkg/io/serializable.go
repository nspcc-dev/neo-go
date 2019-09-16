package io

// Serializable defines the binary encoding/decoding interface.
type Serializable interface {
	Size() int
	DecodeBinary(*BinReader) error
	EncodeBinary(*BinWriter) error
}
