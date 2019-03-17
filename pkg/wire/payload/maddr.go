package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// AddrMessage represents an address message on the neo network
type AddrMessage struct {
	AddrList []*Net_addr
}

// NewAddrMessage instantiates a new AddrMessage
func NewAddrMessage() (*AddrMessage, error) {
	addrMess := &AddrMessage{
		nil,
	}
	return addrMess, nil
}

// AddNetAddr will add a net address into the Address message
func (a *AddrMessage) AddNetAddr(n *Net_addr) error {
	a.AddrList = append(a.AddrList, n)
	// TODO:check if max reached, if so return err. What is max?

	return nil
}

// DecodePayload Implements Messager interface
func (a *AddrMessage) DecodePayload(r io.Reader) error {

	br := &util.BinReader{R: r}
	listLen := br.VarUint()

	a.AddrList = make([]*Net_addr, listLen)
	for i := 0; i < int(listLen); i++ {
		a.AddrList[i] = &Net_addr{}
		a.AddrList[i].DecodePayload(br)
		if br.Err != nil {
			return br.Err
		}
	}
	return br.Err
}

// EncodePayload Implements messager interface
func (a *AddrMessage) EncodePayload(w io.Writer) error {
	bw := &util.BinWriter{W: w}

	listLen := uint64(len(a.AddrList))
	bw.VarUint(listLen)

	for _, addr := range a.AddrList {
		addr.EncodePayload(bw)
	}
	return bw.Err
}

// Command Implements messager interface
func (a *AddrMessage) Command() command.Type {
	return command.Addr
}
