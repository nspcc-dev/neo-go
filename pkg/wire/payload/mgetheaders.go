package payload

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	checksum "github.com/CityOfZion/neo-go/pkg/wire/util/Checksum"
)

type GetHeadersMessage struct {
	w         *bytes.Buffer
	cmd       command.Type
	hashStart []util.Uint256
	hashStop  util.Uint256
}

// Start contains the list of all headers you want to fetch
// End contains the list of the highest header hash you would like to fetch
func NewGetHeadersMessage(start []util.Uint256, stop util.Uint256) (*GetHeadersMessage, error) {
	getHeaders := &GetHeadersMessage{new(bytes.Buffer), command.GetHeaders, start, stop}
	if err := getHeaders.EncodePayload(getHeaders.w); err != nil {
		return nil, err
	}
	return getHeaders, nil

}

func newAbstractGetHeaders(start []util.Uint256, stop util.Uint256, cmd command.Type) (*GetHeadersMessage, error) {
	getHeaders, err := NewGetHeadersMessage(start, stop)

	if err != nil {
		return nil, err
	}
	getHeaders.cmd = cmd
	return getHeaders, nil
}

// Implements Messager interface
func (v *GetHeadersMessage) DecodePayload(r io.Reader) error {

	buf, err := util.ReaderToBuffer(r)
	if err != nil {
		return err
	}

	v.w = buf

	r = bytes.NewReader(buf.Bytes())

	br := util.BinReader{R: r}
	lenStart := br.VarUint()
	v.hashStart = make([]util.Uint256, lenStart)
	br.Read(&v.hashStart)
	br.Read(&v.hashStop)

	return br.Err
}

// Implements messager interface
func (v *GetHeadersMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	bw.VarUint(uint64(len(v.hashStart)))
	bw.Write(v.hashStart)
	bw.Write(v.hashStop)
	return bw.Err
}

// Implements messager interface
func (v *GetHeadersMessage) PayloadLength() uint32 {
	return util.CalculatePayloadLength(v.w)
}

// Implements messager interface
func (v *GetHeadersMessage) Checksum() uint32 {
	return checksum.FromBuf(v.w)
}

// Implements messager interface
func (v *GetHeadersMessage) Command() command.Type {
	return v.cmd
}
