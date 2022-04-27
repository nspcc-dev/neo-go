package io

// Serializable defines the binary encoding/decoding interface. Errors are
// returned via BinReader/BinWriter Err field. These functions must have safe
// behavior when the passed BinReader/BinWriter with Err is already set. Invocations
// to these functions tend to be nested, with this mechanism only the top-level
// caller should handle an error once and all the other code should just not
// panic while there is an error.
type Serializable interface {
	DecodeBinary(*BinReader)
	EncodeBinary(*BinWriter)
}

type decodable interface {
	DecodeBinary(*BinReader)
}

type encodable interface {
	EncodeBinary(*BinWriter)
}
