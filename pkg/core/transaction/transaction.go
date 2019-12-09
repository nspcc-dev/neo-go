package transaction

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
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
	Attributes []Attribute `json:"attributes"`

	// The inputs of the transaction.
	Inputs []Input `json:"vin"`

	// The outputs of the transaction.
	Outputs []Output `json:"vout"`

	// The scripts that comes with this transaction.
	// Scripts exist out of the verification script
	// and invocation script.
	Scripts []Witness `json:"scripts"`

	// Hash of the transaction (double SHA256).
	hash util.Uint256

	// Hash of the transaction used to verify it (single SHA256).
	verificationHash util.Uint256

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

// Hash returns the hash of the transaction.
func (t *Transaction) Hash() util.Uint256 {
	if t.hash.Equals(util.Uint256{}) {
		if t.createHash() != nil {
			panic("failed to compute hash!")
		}
	}
	return t.hash
}

// VerificationHash returns the hash of the transaction used to verify it.
func (t *Transaction) VerificationHash() util.Uint256 {
	if t.verificationHash.Equals(util.Uint256{}) {
		if t.createHash() != nil {
			panic("failed to compute hash!")
		}
	}
	return t.verificationHash
}

// AddOutput adds the given output to the transaction outputs.
func (t *Transaction) AddOutput(out *Output) {
	t.Outputs = append(t.Outputs, *out)
}

// AddInput adds the given input to the transaction inputs.
func (t *Transaction) AddInput(in *Input) {
	t.Inputs = append(t.Inputs, *in)
}

// DecodeBinary implements Serializable interface.
func (t *Transaction) DecodeBinary(br *io.BinReader) {
	br.ReadLE(&t.Type)
	br.ReadLE(&t.Version)
	t.decodeData(br)

	br.ReadArray(&t.Attributes)
	br.ReadArray(&t.Inputs)
	br.ReadArray(&t.Outputs)
	br.ReadArray(&t.Scripts)

	// Create the hash of the transaction at decode, so we dont need
	// to do it anymore.
	if br.Err == nil {
		br.Err = t.createHash()
	}
}

func (t *Transaction) decodeData(r *io.BinReader) {
	switch t.Type {
	case InvocationType:
		t.Data = &InvocationTX{Version: t.Version}
		t.Data.(*InvocationTX).DecodeBinary(r)
	case MinerType:
		t.Data = &MinerTX{}
		t.Data.(*MinerTX).DecodeBinary(r)
	case ClaimType:
		t.Data = &ClaimTX{}
		t.Data.(*ClaimTX).DecodeBinary(r)
	case ContractType:
		t.Data = &ContractTX{}
		t.Data.(*ContractTX).DecodeBinary(r)
	case RegisterType:
		t.Data = &RegisterTX{}
		t.Data.(*RegisterTX).DecodeBinary(r)
	case IssueType:
		t.Data = &IssueTX{}
		t.Data.(*IssueTX).DecodeBinary(r)
	case EnrollmentType:
		t.Data = &EnrollmentTX{}
		t.Data.(*EnrollmentTX).DecodeBinary(r)
	case PublishType:
		t.Data = &PublishTX{Version: t.Version}
		t.Data.(*PublishTX).DecodeBinary(r)
	case StateType:
		t.Data = &StateTX{}
		t.Data.(*StateTX).DecodeBinary(r)
	default:
		r.Err = fmt.Errorf("invalid TX type %x", t.Type)
	}
}

// EncodeBinary implements Serializable interface.
func (t *Transaction) EncodeBinary(bw *io.BinWriter) {
	t.encodeHashableFields(bw)
	bw.WriteArray(t.Scripts)
}

// encodeHashableFields encodes the fields that are not used for
// signing the transaction, which are all fields except the scripts.
func (t *Transaction) encodeHashableFields(bw *io.BinWriter) {
	bw.WriteLE(t.Type)
	bw.WriteLE(t.Version)

	// Underlying TXer.
	if t.Data != nil {
		t.Data.EncodeBinary(bw)
	}

	// Attributes
	bw.WriteArray(t.Attributes)

	// Inputs
	bw.WriteArray(t.Inputs)

	// Outputs
	bw.WriteArray(t.Outputs)
}

// createHash creates the hash of the transaction.
func (t *Transaction) createHash() error {
	buf := io.NewBufBinWriter()
	t.encodeHashableFields(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}

	b := buf.Bytes()
	t.hash = hash.DoubleSha256(b)
	t.verificationHash = hash.Sha256(b)

	return nil
}

// GroupInputsByPrevHash groups all TX inputs by their previous hash.
func (t *Transaction) GroupInputsByPrevHash() map[util.Uint256][]*Input {
	m := make(map[util.Uint256][]*Input)
	for i := range t.Inputs {
		hash := t.Inputs[i].PrevHash
		m[hash] = append(m[hash], &t.Inputs[i])
	}
	return m
}

// GroupOutputByAssetID groups all TX outputs by their assetID.
func (t Transaction) GroupOutputByAssetID() map[util.Uint256][]*Output {
	m := make(map[util.Uint256][]*Output)
	for i := range t.Outputs {
		hash := t.Outputs[i].AssetID
		m[hash] = append(m[hash], &t.Outputs[i])
	}
	return m
}

// Bytes converts the transaction to []byte
func (t *Transaction) Bytes() []byte {
	buf := io.NewBufBinWriter()
	t.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil
	}
	return buf.Bytes()
}
