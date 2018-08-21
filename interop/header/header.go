package header

// Package header provides function signatures that can be used inside
// smart contracts that are written in the neo-go-sc framework.

// Header stubs a NEO block header type.
type Header struct{}

// GetIndex returns the index of the given header.
func GetIndex(h Header) int {
	return 0
}

// GetHash returns the hash of the given header.
func GetHash(h Header) []byte {
	return nil
}

// GetPrevHash returns the previous hash of the given header.
func GetPrevHash(h Header) []byte {
	return nil
}

// GetTimestamp returns the timestamp of the given header.
func GetTimestamp(h Header) int {
	return 0
}

// GetVersion returns the version of the given header.
func GetVersion(h Header) int {
	return 0
}

// GetMerkleRoot returns the merkle root of the given header.
func GetMerkleRoot(h Header) []byte {
	return nil
}

// GetConsensusData returns the consensus data of the given header.
func GetConsensusData(h Header) int {
	return 0
}

// GetNextConsensus returns the next consensus of the given header.
func GetNextConsensus(h Header) []byte {
	return nil
}
