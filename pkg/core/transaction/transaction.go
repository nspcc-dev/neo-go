package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
)

const (
	// MaxTransactionSize is the upper limit size in bytes that a transaction can reach. It is
	// set to be 102400.
	MaxTransactionSize = 102400
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
func (t *Transaction) DecodeBinary(br *io.BinReader) error {
	br.ReadLE(&t.Type)
	br.ReadLE(&t.Version)
	if br.Err != nil {
		return br.Err
	}
	if err := t.decodeData(br); err != nil {
		return err
	}

	lenAttrs := br.ReadVarUint()
	t.Attributes = make([]*Attribute, lenAttrs)
	for i := 0; i < int(lenAttrs); i++ {
		t.Attributes[i] = &Attribute{}
		if err := t.Attributes[i].DecodeBinary(br); err != nil {
			log.Warnf("failed to decode TX %s", t.hash)
			return err
		}
	}

	lenInputs := br.ReadVarUint()
	t.Inputs = make([]*Input, lenInputs)
	for i := 0; i < int(lenInputs); i++ {
		t.Inputs[i] = &Input{}
		if err := t.Inputs[i].DecodeBinary(br); err != nil {
			return err
		}
	}

	lenOutputs := br.ReadVarUint()
	t.Outputs = make([]*Output, lenOutputs)
	for i := 0; i < int(lenOutputs); i++ {
		t.Outputs[i] = &Output{}
		if err := t.Outputs[i].DecodeBinary(br); err != nil {
			return err
		}
	}

	lenScripts := br.ReadVarUint()
	t.Scripts = make([]*Witness, lenScripts)
	for i := 0; i < int(lenScripts); i++ {
		t.Scripts[i] = &Witness{}
		if err := t.Scripts[i].DecodeBinary(br); err != nil {
			return err
		}
	}

	if br.Err != nil {
		return br.Err
	}
	// Create the hash of the transaction at decode, so we dont need
	// to do it anymore.
	return t.createHash()
}

func (t *Transaction) decodeData(r *io.BinReader) error {
	switch t.Type {
	case InvocationType:
		t.Data = &InvocationTX{Version: t.Version}
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
		t.Data = &PublishTX{Version: t.Version}
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
func (t *Transaction) EncodeBinary(bw *io.BinWriter) error {
	if err := t.encodeHashableFields(bw); err != nil {
		return err
	}
	bw.WriteVarUint(uint64(len(t.Scripts)))
	if bw.Err != nil {
		return bw.Err
	}
	for _, s := range t.Scripts {
		if err := s.EncodeBinary(bw); err != nil {
			return err
		}
	}
	return nil
}

// encodeHashableFields will only encode the fields that are not used for
// signing the transaction, which are all fields except the scripts.
func (t *Transaction) encodeHashableFields(bw *io.BinWriter) error {
	bw.WriteLE(t.Type)
	bw.WriteLE(t.Version)
	if bw.Err != nil {
		return bw.Err
	}

	// Underlying TXer.
	if t.Data != nil {
		if err := t.Data.EncodeBinary(bw); err != nil {
			return err
		}
	}

	// Attributes
	bw.WriteVarUint(uint64(len(t.Attributes)))
	if bw.Err != nil {
		return bw.Err
	}
	for _, attr := range t.Attributes {
		if err := attr.EncodeBinary(bw); err != nil {
			return err
		}
	}

	// Inputs
	bw.WriteVarUint(uint64(len(t.Inputs)))
	if bw.Err != nil {
		return bw.Err
	}
	for _, in := range t.Inputs {
		if err := in.EncodeBinary(bw); err != nil {
			return err
		}
	}

	// Outputs
	bw.WriteVarUint(uint64(len(t.Outputs)))
	if bw.Err != nil {
		return bw.Err
	}
	for _, out := range t.Outputs {
		if err := out.EncodeBinary(bw); err != nil {
			return err
		}
	}
	return nil
}

// createHash creates the hash of the transaction.
func (t *Transaction) createHash() error {
	buf := io.NewBufBinWriter()
	if err := t.encodeHashableFields(buf.BinWriter); err != nil {
		return err
	}

	t.hash = hash.DoubleSha256(buf.Bytes())

	return nil
}

// GroupInputsByPrevHash groups all TX inputs by their previous hash.
func (t *Transaction) GroupInputsByPrevHash() map[util.Uint256][]*Input {
	m := make(map[util.Uint256][]*Input)
	for _, in := range t.Inputs {
		m[in.PrevHash] = append(m[in.PrevHash], in)
	}
	return m
}

// GroupOutputByAssetID groups all TX outputs by their assetID.
func (t Transaction) GroupOutputByAssetID() map[util.Uint256][]*Output {
	m := make(map[util.Uint256][]*Output)
	for _, out := range t.Outputs {
		m[out.AssetID] = append(m[out.AssetID], out)
	}
	return m
}

// Bytes convert the transaction to []byte
func (t *Transaction) Bytes() []byte {
	buf := io.NewBufBinWriter()
	if err := t.EncodeBinary(buf.BinWriter); err != nil {
		return nil
	}
	return buf.Bytes()

}
