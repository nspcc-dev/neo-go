package wire

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/wire/util"
)

type InvType uint32

//Inventory types
const (
	InvTypeTx        InvType = 0x01
	InvTypeBlock     InvType = 0x02
	InvTypeConsensus InvType = 0xe0
)

type InvVect struct {
	Type InvType
	Hash Uint256
}

func NewInvVect(typ InvType, hash Uint256) *InvVect {
	return &InvVect{
		Type: typ,
		Hash: hash,
	}
}

// MARK: This has been Encoded and Decoded according to the specs, however not compatible with the
// Rest of network
func (I *InvVect) DecodeInv(r io.Reader) error {
	br := &util.BinReader{R: r}
	br.Read(&I.Type)
	br.Read(&I.Hash)
	return br.Err
}
func (I *InvVect) EncodeInv(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	bw.Write(I.Type)
	bw.Write(I.Hash)
	return bw.Err
}
