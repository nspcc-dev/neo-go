package header

import "github.com/CityOfZion/neo-go/pkg/core"

// GetIndex returns the index of the given header.
func GetIndex(header *core.Header) uint32 { return 0 }

// GetHash returns the hash of the given header.
func GetHash(header *core.Header) []byte { return nil }

// GetHash returns the version of the given header.
func GetVersion(header *core.Header) uint32 { return 0 }

// GetHash returns the previous hash of the given header.
func GetPrevHash(header *core.Header) []byte { return nil }

// GetHash returns the merkle root of the given header.
func GetMerkleRoot(header *core.Header) []byte { return nil }

// GetHash returns the timestamp of the given header.
func GetTimestamp(header *core.Header) uint32 { return 0 }

// GetHash returns the next validator address of the given header.
func GetNextConsensus(header *core.Header) []byte { return nil }
