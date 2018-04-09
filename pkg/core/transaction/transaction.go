package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
)

// Transaction is a process recorded in the NEO blockchain.
type Transaction struct {
	// The type of the transaction.
	Type TXType `json:"type"`

	// The trading version which is currently 0.
	Version uint8 `json:"version"`

	// Data specific to the type of the transaction.
	// This is always a pointer to a <Type>Transaction.
	Data TXer `json:"-"`

	// Transaction attributes.
	Attributes []*Attribute `json:"attributes"`

	// The inputs of the transaction.
	Inputs []*Input `json:"vin"`

	// The outputs of the transaction.
	Outputs []*Output `json:"vout"`

	// The scripts that comes with this transaction.
	// Scripts exist out of the verification script
	// and invocation script.
	Scripts []*Witness `json:"scripts"`

	// hash of the transaction
	hash util.Uint256

	// Trimmed indicates this is a transaction from trimmed
	// data.
	Trimmed bool `json:"-"`
}

// NewTrimmedTX returns a trimmed transaction with only its hash
// and Trimmed to true.
func NewTrimmedTX(hash util.Uint256) *Transaction {
	return &Transaction{
		hash:    hash,
		Trimmed: true,
	}
}

// Hash return the hash of the transaction.
func (t *Transaction) Hash() util.Uint256 {
	if t.hash.Equals(util.Uint256{}) {
		t.createHash()
	}
	return t.hash
}

// AddOutput adds the given output to the transaction outputs.
func (t *Transaction) AddOutput(out *Output) {
	t.Outputs = append(t.Outputs, out)
}

// AddInput adds the given input to the transaction inputs.
func (t *Transaction) AddInput(in *Input) {
	t.Inputs = append(t.Inputs, in)
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
			// @TODO: remove this when TX attribute decode bug is solved.
			log.Warnf("failed to decode TX %s", t.hash)
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

	// Create the hash of the transaction at decode, so we dont need
	// to do it anymore.
	return t.createHash()
}

func (t *Transaction) decodeData(r io.Reader) error {
	switch t.Type {
	case InvocationType:
		t.Data = &InvocationTX{}
		return t.Data.(*InvocationTX).DecodeBinary(r)
	case MinerType:
		t.Data = &MinerTX{}
		return t.Data.(*MinerTX).DecodeBinary(r)
	case ClaimType:
		t.Data = &ClaimTX{}
		return t.Data.(*ClaimTX).DecodeBinary(r)
	case ContractType:
		t.Data = &ContractTX{}
		return t.Data.(*ContractTX).DecodeBinary(r)
	case RegisterType:
		t.Data = &RegisterTX{}
		return t.Data.(*RegisterTX).DecodeBinary(r)
	case IssueType:
		t.Data = &IssueTX{}
		return t.Data.(*IssueTX).DecodeBinary(r)
	case EnrollmentType:
		t.Data = &EnrollmentTX{}
		return t.Data.(*EnrollmentTX).DecodeBinary(r)
	case PublishType:
		t.Data = &PublishTX{}
		return t.Data.(*PublishTX).DecodeBinary(r)
	case StateType:
		t.Data = &StateTX{}
		return t.Data.(*StateTX).DecodeBinary(r)
	default:
		log.Warnf("invalid TX type %s", t.Type)
	}
	return nil
}

// EncodeBinary implements the payload interface.
func (t *Transaction) EncodeBinary(w io.Writer) error {
	if err := t.encodeHashableFields(w); err != nil {
		return err
	}
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

// encodeHashableFields will only encode the fields that are not used for
// signing the transaction, which are all fields except the scripts.
func (t *Transaction) encodeHashableFields(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, t.Type); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, t.Version); err != nil {
		return err
	}

	// Underlying TXer.
	if t.Data != nil {
		if err := t.Data.EncodeBinary(w); err != nil {
			return err
		}
	}

	// Attributes
	lenAttrs := uint64(len(t.Attributes))
	if err := util.WriteVarUint(w, lenAttrs); err != nil {
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
	return nil
}

// createHash creates the hash of the transaction.
func (t *Transaction) createHash() error {
	buf := new(bytes.Buffer)
	if err := t.encodeHashableFields(buf); err != nil {
		return err
	}

	var hash util.Uint256
	hash = sha256.Sum256(buf.Bytes())
	hash = sha256.Sum256(hash.Bytes())
	t.hash = hash

	return nil
}

// GroupTXInputsByPrevHash groups all TX inputs by their previous hash.
func (t *Transaction) GroupInputsByPrevHash() map[util.Uint256][]*Input {
	m := make(map[util.Uint256][]*Input)
	for _, in := range t.Inputs {
		m[in.PrevHash] = append(m[in.PrevHash], in)
	}
	return m
}
