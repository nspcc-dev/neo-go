/*
Package header contains functions working with block headers.
*/
package header

// Header represents Neo block header type, it's an opaque data structure that
// can be used by functions from this package. You can create it with
// blockchain.GetHeader. In its function it's similar to the Header class
// of the Neo .net framework.
type Header struct{}

// GetIndex returns the index (height) of the given header. It uses
// `Neo.Header.GetIndex` syscall.
func GetIndex(h Header) int {
	return 0
}

// GetHash returns the hash (256-bit BE value packed into 32 byte slice) of the
// given header (which also is a hash of the block). It uses `Neo.Header.GetHash`
// syscall.
func GetHash(h Header) []byte {
	return nil
}

// GetPrevHash returns the hash (256-bit BE value packed into 32 byte slice) of
// the previous block stored in the given header. It uses `Neo.Header.GetPrevHash`
// syscall.
func GetPrevHash(h Header) []byte {
	return nil
}

// GetTimestamp returns the timestamp of the given header. It uses
// `Neo.Header.GetTimestamp` syscall.
func GetTimestamp(h Header) int {
	return 0
}

// GetVersion returns the version of the given header. It uses
// `Neo.Header.GetVersion` syscall.
func GetVersion(h Header) int {
	return 0
}

// GetMerkleRoot returns the Merkle root (256-bit BE value packed into 32 byte
// slice) of the given header. It uses `Neo.Header.GetMerkleRoot` syscall.
func GetMerkleRoot(h Header) []byte {
	return nil
}

// GetConsensusData returns the consensus data (nonce) of the given header.
// It uses `Neo.Header.GetConsensusData` syscall.
func GetConsensusData(h Header) int {
	return 0
}

// GetNextConsensus returns the next consensus field (verification script hash,
// 160-bit BE value packed into 20 byte slice) of the given header. It uses
// `Neo.Header.GetNextConsensus` syscall.
func GetNextConsensus(h Header) []byte {
	return nil
}
