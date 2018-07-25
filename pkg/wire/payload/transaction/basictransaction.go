package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type encodeExclusiveFields func(bw *util.BinWriter)
type decodeExclusiveFields func(br *util.BinReader)

type BasicTransaction struct {
	Type            TXType
	Version         TXVersion
	Inputs          []*Input
	Outputs         []*Output
	Attributes      []*Attribute
	Witnesses       []*Witness
	Hash            util.Uint256
	encodeExclusive encodeExclusiveFields // Overwrite this in Other transaction structs like Invocation to provide custom implementations
	decodeExclusive decodeExclusiveFields
}

func createBasicTransaction(Type TXType, Version TXVersion) *BasicTransaction {
	return &BasicTransaction{
		Type:       ContractType,
		Version:    Version,
		Inputs:     []*Input{},
		Outputs:    []*Output{},
		Attributes: []*Attribute{},
		Witnesses:  []*Witness{},
	}

}

func (b *BasicTransaction) EncodePayload(bw *util.BinWriter) {
	b.encodeHashableFields(bw)

	lenWitnesses := uint64(len(b.Witnesses))
	bw.VarUint(lenWitnesses)

	for _, witness := range b.Witnesses {
		witness.Encode(bw)
	}
}

func (b *BasicTransaction) encodeHashableFields(bw *util.BinWriter) {
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

// TODO
func DecodePayload(br *util.BinReader, DecodeExclusive decodeExclusiveFields) error {
	return br.Err
}

func (b *BasicTransaction) AddInput(i *Input) {
	b.Inputs = append(b.Inputs, i)
}
func (b *BasicTransaction) AddOutput(o *Output) {
	b.Outputs = append(b.Outputs, o)
}
func (b *BasicTransaction) AddAttribute(a *Attribute) {
	b.Attributes = append(b.Attributes, a)
}
func (b *BasicTransaction) AddWitness(w *Witness) {
	b.Witnesses = append(b.Witnesses, w)
}

func (b *BasicTransaction) createHash() error {

	hash, err := util.CalculateHash(b.encodeHashableFields)
	b.Hash = hash
	return err
}
