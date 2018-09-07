package transaction

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type encodeExclusiveFields func(bw *util.BinWriter)
type decodeExclusiveFields func(br *util.BinReader)

// Transactioner is the interface that will unite the
// transaction types. Each transaction will implement this interface
// and so wil be a transactioner

type Transactioner interface {
	Encode(w io.Writer) error
	Decode(r io.Reader) error
	ID() (util.Uint256, error)
}

// Base transaction is the template for all other transactions
// It contains all of the shared fields between transactions and
// the additional encodeExclusive and decodeExclusive methods, which
// can be overwitten in the other transactions to encode the non shared fields
type Base struct {
	Type            types.TX
	Version         version.TX
	Inputs          []*Input
	Outputs         []*Output
	Attributes      []*Attribute
	Witnesses       []*Witness
	Hash            util.Uint256
	encodeExclusive encodeExclusiveFields
	decodeExclusive decodeExclusiveFields
}

func createBaseTransaction(typ types.TX, ver version.TX) *Base {
	return &Base{
		Type:       typ,
		Version:    ver,
		Inputs:     []*Input{},
		Outputs:    []*Output{},
		Attributes: []*Attribute{},
		Witnesses:  []*Witness{},
	}

}

func (b *Base) Decode(r io.Reader) error {
	br := &util.BinReader{R: r}
	return b.DecodePayload(br)
}
func (b *Base) Encode(w io.Writer) error {
	bw := &util.BinWriter{W: w}
	b.EncodePayload(bw)
	return bw.Err
}

func (b *Base) EncodePayload(bw *util.BinWriter) {
	b.encodeHashableFields(bw)

	lenWitnesses := uint64(len(b.Witnesses))
	bw.VarUint(lenWitnesses)

	for _, witness := range b.Witnesses {
		witness.Encode(bw)
	}
}

func (b *Base) DecodePayload(br *util.BinReader) error {
	b.decodeHashableFields(br)

	lenWitnesses := br.VarUint()

	b.Witnesses = make([]*Witness, lenWitnesses)
	for i := 0; i < int(lenWitnesses); i++ {
		b.Witnesses[i] = &Witness{}
		b.Witnesses[i].Decode(br)
	}

	if br.Err != nil {
		return br.Err
	}

	return b.createHash()
}

func (b *Base) encodeHashableFields(bw *util.BinWriter) {
	b.Type.Encode(bw)
	b.Version.Encode(bw)

	b.encodeExclusive(bw)

	lenAttrs := uint64(len(b.Attributes))
	lenInputs := uint64(len(b.Inputs))
	lenOutputs := uint64(len(b.Outputs))

	bw.VarUint(lenAttrs)
	for _, attr := range b.Attributes {
		attr.Encode(bw)
	}

	bw.VarUint(lenInputs)
	for _, input := range b.Inputs {
		input.Encode(bw)
	}

	bw.VarUint(lenOutputs)
	for _, output := range b.Outputs {
		output.Encode(bw)
	}
}

// created for consistency
func (b *Base) decodeHashableFields(br *util.BinReader) {
	b.Type.Decode(br)

	b.Version.Decode(br)

	b.decodeExclusive(br)

	lenAttrs := br.VarUint()
	b.Attributes = make([]*Attribute, lenAttrs)
	for i := 0; i < int(lenAttrs); i++ {

		b.Attributes[i] = &Attribute{}
		b.Attributes[i].Decode(br)
	}

	lenInputs := br.VarUint()

	b.Inputs = make([]*Input, lenInputs)
	for i := 0; i < int(lenInputs); i++ {
		b.Inputs[i] = &Input{}
		b.Inputs[i].Decode(br)
	}

	lenOutputs := br.VarUint()
	b.Outputs = make([]*Output, lenOutputs)
	for i := 0; i < int(lenOutputs); i++ {
		b.Outputs[i] = &Output{}
		b.Outputs[i].Decode(br)
	}

}

func (b *Base) AddInput(i *Input) {
	b.Inputs = append(b.Inputs, i)
}
func (b *Base) AddOutput(o *Output) {
	b.Outputs = append(b.Outputs, o)
}
func (b *Base) AddAttribute(a *Attribute) {
	b.Attributes = append(b.Attributes, a)
}
func (b *Base) AddWitness(w *Witness) {
	b.Witnesses = append(b.Witnesses, w)
}

func (b *Base) createHash() error {

	hash, err := util.CalculateHash(b.encodeHashableFields)
	b.Hash = hash
	return err
}

// TXHash returns the TXID of the transactions
func (b *Base) ID() (util.Uint256, error) {
	var emptyHash util.Uint256
	var err error
	if b.Hash == emptyHash {
		err = b.createHash()
	}
	return b.Hash, err
}
