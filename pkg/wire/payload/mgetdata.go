package payload

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type GetDataMessage struct {
	w      *bytes.Buffer
	Type   InvType
	Hashes []util.Uint256
}

func NewGetDataMessage(typ InvType, Hashes []util.Uint256) (*GetDataMessage, error) {
	getData := &GetDataMessage{nil, typ, Hashes}
	if err := getData.EncodePayload(getData.w); err != nil {
		return nil, err
	}
	return getData, nil
}

func (i *GetDataMessage) AddHash(h util.Uint256) error {
	if len(i.Hashes)+1 > maxHashes {
		return MaxHashError
	}
	i.Hashes = append(i.Hashes, h)
	return nil
}

// Implements Messager interface
func (v *GetDataMessage) DecodePayload(r io.Reader) error {
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
func (v *GetDataMessage) EncodePayload(w io.Writer) error {
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
func (v *GetDataMessage) PayloadLength() uint32 {
	return util.CalculatePayloadLength(v.w)
}

// Implements messager interface
func (v *GetDataMessage) Checksum() uint32 {
	return util.CalculateCheckSum(v.w)
}

// Implements messager interface
func (v *GetDataMessage) Command() command.Type {
	return command.GetData
}
