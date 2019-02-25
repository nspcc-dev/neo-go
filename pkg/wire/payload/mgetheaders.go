package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type GetHeadersMessage struct {
	cmd       command.Type
	hashStart []util.Uint256
	hashStop  util.Uint256
}

// Start contains the list of all headers you want to fetch
// End contains the list of the highest header hash you would like to fetch
func NewGetHeadersMessage(start []util.Uint256, stop util.Uint256) (*GetHeadersMessage, error) {
	getHeaders := &GetHeadersMessage{command.GetHeaders, start, stop}

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
func (v *GetHeadersMessage) Command() command.Type {
	return v.cmd
}
