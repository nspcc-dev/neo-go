package payload

import (
	"errors"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type InvType uint8

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
	cmd    command.Type
	Type   InvType
	Hashes []util.Uint256
}

func NewInvMessage(typ InvType) (*InvMessage, error) {

	inv := &InvMessage{
		command.Inv,
		typ,
		nil,
	}

	return inv, nil
}

func newAbstractInv(typ InvType, cmd command.Type) (*InvMessage, error) {
	inv, err := NewInvMessage(typ)

	if err != nil {
		return nil, err
	}
	inv.cmd = cmd

	return inv, nil

}

func (i *InvMessage) AddHash(h util.Uint256) error {
	if len(i.Hashes)+1 > maxHashes {
		return MaxHashError
	}
	i.Hashes = append(i.Hashes, h)
	return nil
}
func (i *InvMessage) AddHashes(hashes []util.Uint256) error {
	var err error
	for _, hash := range hashes {
		err = i.AddHash(hash)
		if err != nil {
			break
		}
	}
	return err
}

// Implements Messager interface
func (v *InvMessage) DecodePayload(r io.Reader) error {
	br := &util.BinReader{R: r}

	br.Read(&v.Type)

	listLen := br.VarUint()
	v.Hashes = make([]util.Uint256, listLen)

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
func (v *InvMessage) Command() command.Type {
	return v.cmd
}
