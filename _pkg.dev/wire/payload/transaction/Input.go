package transaction

import "github.com/CityOfZion/neo-go/pkg/wire/util"

// Input represents a Transaction input.
type Input struct {
	// The hash of the previous transaction.
	PrevHash util.Uint256

	// The index of the previous transaction.
	PrevIndex uint16
}

//NewInput returns a transaction input object
func NewInput(prevHash util.Uint256, prevIndex uint16) *Input {
	return &Input{
		prevHash,
		prevIndex,
	}
}

// Encode encodes the given input into a binary writer
func (i *Input) Encode(bw *util.BinWriter) {
	bw.Write(i.PrevHash)
	bw.Write(i.PrevIndex)
}

// Decode decodes a binary reader into an input object
func (i *Input) Decode(br *util.BinReader) {
	br.Read(&i.PrevHash)
	br.Read(&i.PrevIndex)
}
