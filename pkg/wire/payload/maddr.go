package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type AddrMessage struct {
	AddrList []*net_addr
}

func NewAddrMessage() (*AddrMessage, error) {
	addrMess := &AddrMessage{
		nil,
	}
	return addrMess, nil
}

func (a *AddrMessage) AddNetAddr(n *net_addr) error {
	a.AddrList = append(a.AddrList, n)
	// TODO:check if max reached, if so return err. What is max?

	return nil
}

// Implements Messager interface
func (a *AddrMessage) DecodePayload(r io.Reader) error {

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
func (v *AddrMessage) Command() command.Type {
	return command.Addr
}
