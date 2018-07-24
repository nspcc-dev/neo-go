package wire

import (
	"bytes"
	"errors"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/wire/util"
)

// MARK: This and invvect.go have been Encoded and Decoded according to the specs, however not compatible with the

const (
	maxHashes = 0x10000000
)

var (
	MaxHashError = errors.New("Max size For Hashes reached")
)

type InvMessage struct {
	InvList []*InvVect
}

func NewInvMessage(invList []*InvVect) (*InvMessage, error) {
	return &InvMessage{invList}, nil
}

func (i *InvMessage) AddInvVect(v *InvVect) error {
	if len(i.InvList)+1 > maxHashes {
		return MaxHashError
	}
	i.InvList = append(i.InvList, v)
	return nil
}

// Implements Messager interface
func (v *InvMessage) DecodePayload(r io.Reader) error {
	return nil
}

// Implements messager interface
func (v *InvMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	// bw.Write(v.)
	return bw.Err
}

// Implements messager interface
func (v *InvMessage) PayloadLength() uint32 {
	return 0
}

// Implements messager interface
func (v *InvMessage) Checksum() uint32 {
	return calculateCheckSum(new(bytes.Buffer))
}

// Implements messager interface
func (v *InvMessage) Command() CommandType {
	return CMDInv
}
