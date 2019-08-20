package payload

import (
	"errors"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

//InvType represents the enum of inventory types
type InvType uint8

const (
	// InvTypeTx represents the transaction inventory type
	InvTypeTx InvType = 0x01
	// InvTypeBlock represents the block inventory type
	InvTypeBlock InvType = 0x02
	// InvTypeConsensus represents the consensus inventory type
	InvTypeConsensus InvType = 0xe0
)

const maxHashes = 0x10000000

var errMaxHash = errors.New("max size For Hashes reached")

// InvMessage represents an Inventory message on the neo-network
type InvMessage struct {
	cmd    command.Type
	Type   InvType
	Hashes []util.Uint256
}

//NewInvMessage returns an InvMessage object
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

// AddHash adds a hash to the list of hashes
func (inv *InvMessage) AddHash(h util.Uint256) error {
	if len(inv.Hashes)+1 > maxHashes {
		return errMaxHash
	}
	inv.Hashes = append(inv.Hashes, h)
	return nil
}

// AddHashes adds multiple hashes to the list of hashes
func (inv *InvMessage) AddHashes(hashes []util.Uint256) error {
	var err error
	for _, hash := range hashes {
		err = inv.AddHash(hash)
		if err != nil {
			break
		}
	}
	return err
}

// DecodePayload Implements Messager interface
func (inv *InvMessage) DecodePayload(r io.Reader) error {
	br := &util.BinReader{R: r}

	br.Read(&inv.Type)

	listLen := br.VarUint()
	inv.Hashes = make([]util.Uint256, listLen)

	for i := 0; i < int(listLen); i++ {
		br.Read(&inv.Hashes[i])
	}
	return nil
}

// EncodePayload Implements messager interface
func (inv *InvMessage) EncodePayload(w io.Writer) error {

	bw := &util.BinWriter{W: w}
	bw.Write(inv.Type)

	lenhashes := len(inv.Hashes)
	bw.VarUint(uint64(lenhashes))

	for _, hash := range inv.Hashes {

		bw.Write(hash)

	}

	return bw.Err
}

// Command Implements messager interface
func (inv *InvMessage) Command() command.Type {
	return inv.cmd
}
