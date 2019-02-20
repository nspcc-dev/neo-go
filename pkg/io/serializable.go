package io

import "io"

// Serializable defines the binary encoding/decoding interface.
type Serializable interface {
	Size() int
	DecodeBinary(io.Reader) error
	EncodeBinary(io.Writer) error
}
