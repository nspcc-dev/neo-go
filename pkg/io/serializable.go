package io

// Serializable defines the binary encoding/decoding interface. Errors are
// returned via BinReader/BinWriter Err field. These functions must have safe
// behavior when passed BinReader/BinWriter with Err already set. Invocations
// to these functions tend to be nested, with this mechanism only the top-level
// caller should handle the error once and all the other code should just not
// panic in presence of error.
type Serializable interface {
	DecodeBinary(BinaryReader)
	EncodeBinary(BinaryWriter)
}

type decodable interface {
	DecodeBinary(BinaryReader)
}

type encodable interface {
	EncodeBinary(BinaryWriter)
}
