package wire

import (
	"bytes"
	"errors"
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

const (
	maxHashes = 0x10000000
)

var (
	MaxHashError = errors.New("Max size For Hashes reached")
)

type InvMessage struct {
	w      *bytes.Buffer
	Type   InvType
	Hashes []Uint256
}

func NewInvMessage(typ InvType, Hashes []Uint256) (*InvMessage, error) {
	inv := &InvMessage{nil, typ, Hashes}
	if err := inv.EncodePayload(inv.w); err != nil {
		return nil, err
	}
	return inv, nil
}

func (i *InvMessage) AddHash(h Uint256) error {
	if len(i.Hashes)+1 > maxHashes {
		return MaxHashError
	}
	i.Hashes = append(i.Hashes, h)
	return nil
}

// Implements Messager interface
func (v *InvMessage) DecodePayload(r io.Reader) error {
	br := &util.BinReader{R: r}

	br.Read(&v.Type)
	listLen := br.VarUint()
	v.Hashes = make([]Uint256, listLen)

	for i := 0; i < int(listLen); i++ {
		br.Read(&v.Hashes[i])
	}
	return nil
}

// Implements messager interface
func (v *InvMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}

	bw.Write(v.Type)

	lenhashes := len(v.Hashes)
	bw.VarUint(uint64(lenhashes))

	for _, hash := range v.Hashes {
		bw.Write(hash)
	}

	return bw.Err
}

// Implements messager interface
func (v *InvMessage) PayloadLength() uint32 {
	return calculatePayloadLength(v.w)
}

// Implements messager interface
func (v *InvMessage) Checksum() uint32 {
	return calculateCheckSum(v.w)
}

// Implements messager interface
func (v *InvMessage) Command() CommandType {
	return CMDInv
}
