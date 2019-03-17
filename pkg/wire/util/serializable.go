package util

// Serializable defines the binary encoding/decoding interface.
type Serializable interface {
	Size() int
	Decode(br *BinReader)
	Encode(bw *BinWriter)
}
