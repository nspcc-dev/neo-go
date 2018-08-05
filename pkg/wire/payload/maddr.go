package payload

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type AddrMessage struct {
	w        *bytes.Buffer
	AddrList []*net_addr
}

// Implements Messager interface
func (a *AddrMessage) DecodePayload(r io.Reader) error {

	buf, err := util.ReaderToBuffer(r)
	if err != nil {
		return err
	}

	a.w = buf

	br := &util.BinReader{R: r}
	listLen := br.VarUint()

	a.AddrList = make([]*net_addr, listLen)
	for i := 0; i < int(listLen); i++ {
		a.AddrList[i] = &net_addr{}
		a.AddrList[i].DecodePayload(br)
		if br.Err != nil {
			return br.Err
		}
	}
	return br.Err
}

// Implements messager interface
func (v *AddrMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}

	listLen := uint64(len(v.AddrList))
	bw.VarUint(listLen)

	for _, addr := range v.AddrList {
		addr.EncodePayload(bw)
	}
	return bw.Err
}

// Implements messager interface
func (v *AddrMessage) PayloadLength() uint32 {
	return util.CalculatePayloadLength(v.w)
}

// Implements messager interface
func (v *AddrMessage) Checksum() uint32 {
	return util.CalculateCheckSum(v.w)
}

// Implements messager interface
func (v *AddrMessage) Command() command.Type {
	return command.Addr
}
