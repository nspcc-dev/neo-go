package transaction

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Transaction is a process recorded in the NEO blockchain.
type Transaction struct {
	// The type of the transaction.
	Type TransactionType

	// The trading version which is currently 0.
	Version uint8

	// Data specific to the type of the transaction.
	// This is always a pointer to a <Type>Transaction.
	Data interface{}

	// Transaction attributes.
	Attributes []*Attribute

	// The inputs of the transaction.
	Inputs []*Input

	// The outputs of the transaction.
	Outputs []*Output

	// The scripts that comes with this transaction.
	// Scripts exist out of the verification script
	// and invocation script.
	Scripts []*Witness
}

// DecodeBinary implements the payload interface.
func (t *Transaction) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &t.Type); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &t.Version); err != nil {
		return err
	}
	if err := t.decodeData(r); err != nil {
		return err
	}

	lenAttrs := util.ReadVarUint(r)
	t.Attributes = make([]*Attribute, lenAttrs)
	for i := 0; i < int(lenAttrs); i++ {
		t.Attributes[i] = &Attribute{}
		if err := t.Attributes[i].DecodeBinary(r); err != nil {
			return err
		}
	}

	lenInputs := util.ReadVarUint(r)
	t.Inputs = make([]*Input, lenInputs)
	for i := 0; i < int(lenInputs); i++ {
		t.Inputs[i] = &Input{}
		if err := t.Inputs[i].DecodeBinary(r); err != nil {
			return err
		}
	}

	lenOutputs := util.ReadVarUint(r)
	t.Outputs = make([]*Output, lenOutputs)
	for i := 0; i < int(lenOutputs); i++ {
		t.Outputs[i] = &Output{}
		if err := t.Outputs[i].DecodeBinary(r); err != nil {
			return err
		}
	}

	lenScripts := util.ReadVarUint(r)
	t.Scripts = make([]*Witness, lenScripts)
	for i := 0; i < int(lenScripts); i++ {
		t.Scripts[i] = &Witness{}
		if err := t.Scripts[i].DecodeBinary(r); err != nil {
			return err
		}
	}

	return nil
}

func (t *Transaction) decodeData(r io.Reader) error {
	switch t.Type {
	case MinerTX:
	case ClaimTX:
		t.Data = &ClaimTransaction{}
		return t.Data.(*ClaimTransaction).DecodeBinary(r)
	}
	return nil
}

// EncodeBinary implements the payload interface.
func (t *Transaction) EncodeBinary(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, t.Type); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, t.Version); err != nil {
		return err
	}
	if err := t.encodeData(w); err != nil {
		return err
	}

	// Attributes
	if err := util.WriteVarUint(w, uint64(len(t.Attributes))); err != nil {
		return err
	}
	for _, attr := range t.Attributes {
		if err := attr.EncodeBinary(w); err != nil {
			return err
		}
	}

	// Inputs
	if err := util.WriteVarUint(w, uint64(len(t.Inputs))); err != nil {
		return err
	}
	for _, in := range t.Inputs {
		if err := in.EncodeBinary(w); err != nil {
			return err
		}
	}

	// Outputs
	if err := util.WriteVarUint(w, uint64(len(t.Outputs))); err != nil {
		return err
	}
	for _, out := range t.Outputs {
		if err := out.EncodeBinary(w); err != nil {
			return err
		}
	}

	// Scripts
	if err := util.WriteVarUint(w, uint64(len(t.Scripts))); err != nil {
		return err
	}
	for _, s := range t.Scripts {
		if err := s.EncodeBinary(w); err != nil {
			return err
		}
	}

	return nil
}

func (t *Transaction) encodeData(w io.Writer) error {
	switch t.Type {
	case MinerTX:
	case ClaimTX:
		return t.Data.(*ClaimTransaction).EncodeBinary(w)
	}
	return nil
}
