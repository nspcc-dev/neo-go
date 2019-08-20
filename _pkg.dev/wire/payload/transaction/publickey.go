package transaction

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// PublicKey represents a public key on the neo network
type PublicKey struct {
	Key []byte
}

//Encode encodes a public key into a binary writer
func (p *PublicKey) Encode(bw *util.BinWriter) {
	bw.Write(p.Key)
}

// Decode decodes a bianry reader into a public key
func (p *PublicKey) Decode(br *util.BinReader) {
	var prefix uint8
	br.Read(&prefix)

	// Compressed public keys.
	if prefix == 0x02 || prefix == 0x03 {
		p.Key = make([]byte, 32)
		br.Read(p.Key)
	} else if prefix == 0x04 {
		p.Key = make([]byte, 65)
		br.Read(p.Key)
	} else if prefix == 0x00 {
		// do nothing, For infinity, the p.Key == 0x00, included in the prefix
	} else {
		br.Err = errors.New("Prefix not recognised for public key")
		return
	}
	p.Key = append([]byte{prefix}, p.Key...)
}
