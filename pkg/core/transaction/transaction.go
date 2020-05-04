package transaction

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// MaxTransactionSize is the upper limit size in bytes that a transaction can reach. It is
	// set to be 102400.
	MaxTransactionSize = 102400
	// MaxValidUntilBlockIncrement is the upper increment size of blockhain height in blocs after
	// exceeding that a transaction should fail validation. It is set to be 2102400.
	MaxValidUntilBlockIncrement = 2102400
)

// Transaction is a process recorded in the NEO blockchain.
type Transaction struct {
	// The type of the transaction.
	Type TXType

	// The trading version which is currently 0.
	Version uint8

	// Random number to avoid hash collision.
	Nonce uint32

	// Address signed the transaction.
	Sender util.Uint160

	// Maximum blockchain height exceeding which
	// transaction should fail verification.
	ValidUntilBlock uint32

	// Data specific to the type of the transaction.
	// This is always a pointer to a <Type>Transaction.
	Data TXer

	// Transaction attributes.
	Attributes []Attribute

	// The inputs of the transaction.
	Inputs []Input

	// The outputs of the transaction.
	Outputs []Output

	// The scripts that comes with this transaction.
	// Scripts exist out of the verification script
	// and invocation script.
	Scripts []Witness

	// Hash of the transaction (double SHA256).
	hash util.Uint256

	// Hash of the transaction used to verify it (single SHA256).
	verificationHash util.Uint256

	// Trimmed indicates this is a transaction from trimmed
	// data.
	Trimmed bool
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
	t.Type = TXType(br.ReadB())
	t.Version = uint8(br.ReadB())
	t.Nonce = br.ReadU32LE()
	t.Sender.DecodeBinary(br)

	t.ValidUntilBlock = br.ReadU32LE()
	t.decodeData(br)

	br.ReadArray(&t.Attributes)
	br.ReadArray(&t.Inputs)
	br.ReadArray(&t.Outputs)
	for i := range t.Outputs {
		if t.Outputs[i].Amount.LessThan(0) {
			br.Err = errors.New("negative output")
			return
		}
	}
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
	noData := t.Type == ContractType
	if t.Data == nil && !noData {
		bw.Err = errors.New("transaction has no data")
		return
	}
	bw.WriteB(byte(t.Type))
	bw.WriteB(byte(t.Version))
	bw.WriteU32LE(t.Nonce)
	t.Sender.EncodeBinary(bw)
	bw.WriteU32LE(t.ValidUntilBlock)

	// Underlying TXer.
	if !noData {
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
	t.verificationHash = hash.Sha256(b)
	t.hash = hash.Sha256(t.verificationHash.BytesBE())

	return nil
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

// GetSignedPart returns a part of the transaction which must be signed.
func (t *Transaction) GetSignedPart() []byte {
	buf := io.NewBufBinWriter()
	t.encodeHashableFields(buf.BinWriter)
	if buf.Err != nil {
		return nil
	}
	return buf.Bytes()
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

// NewTransactionFromBytes decodes byte array into *Transaction
func NewTransactionFromBytes(b []byte) (*Transaction, error) {
	tx := &Transaction{}
	r := io.NewBinReaderFromBuf(b)
	tx.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	return tx, nil
}

// transactionJSON is a wrapper for Transaction and
// used for correct marhalling of transaction.Data
type transactionJSON struct {
	TxID            util.Uint256 `json:"txid"`
	Size            int          `json:"size"`
	Type            TXType       `json:"type"`
	Version         uint8        `json:"version"`
	Nonce           uint32       `json:"nonce"`
	Sender          string       `json:"sender"`
	ValidUntilBlock uint32       `json:"valid_until_block"`
	Attributes      []Attribute  `json:"attributes"`
	Inputs          []Input      `json:"vin"`
	Outputs         []Output     `json:"vout"`
	Scripts         []Witness    `json:"scripts"`

	Claims []Input          `json:"claims,omitempty"`
	Script string           `json:"script,omitempty"`
	Gas    util.Fixed8      `json:"gas,omitempty"`
	Asset  *registeredAsset `json:"asset,omitempty"`
}

// MarshalJSON implements json.Marshaler interface.
func (t *Transaction) MarshalJSON() ([]byte, error) {
	tx := transactionJSON{
		TxID:            t.Hash(),
		Size:            io.GetVarSize(t),
		Type:            t.Type,
		Version:         t.Version,
		Nonce:           t.Nonce,
		Sender:          address.Uint160ToString(t.Sender),
		ValidUntilBlock: t.ValidUntilBlock,
		Attributes:      t.Attributes,
		Inputs:          t.Inputs,
		Outputs:         t.Outputs,
		Scripts:         t.Scripts,
	}
	switch t.Type {
	case ClaimType:
		tx.Claims = t.Data.(*ClaimTX).Claims
	case InvocationType:
		tx.Script = hex.EncodeToString(t.Data.(*InvocationTX).Script)
		tx.Gas = t.Data.(*InvocationTX).Gas
	case RegisterType:
		transaction := *t.Data.(*RegisterTX)
		tx.Asset = &registeredAsset{
			AssetType: transaction.AssetType,
			Name:      json.RawMessage(transaction.Name),
			Amount:    transaction.Amount,
			Precision: transaction.Precision,
			Owner:     transaction.Owner,
			Admin:     address.Uint160ToString(transaction.Admin),
		}
	}
	return json.Marshal(tx)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (t *Transaction) UnmarshalJSON(data []byte) error {
	tx := new(transactionJSON)
	if err := json.Unmarshal(data, tx); err != nil {
		return err
	}
	t.Type = tx.Type
	t.Version = tx.Version
	t.Nonce = tx.Nonce
	t.ValidUntilBlock = tx.ValidUntilBlock
	t.Attributes = tx.Attributes
	t.Inputs = tx.Inputs
	t.Outputs = tx.Outputs
	t.Scripts = tx.Scripts
	sender, err := address.StringToUint160(tx.Sender)
	if err != nil {
		return errors.New("cannot unmarshal tx: bad sender")
	}
	t.Sender = sender
	switch tx.Type {
	case ClaimType:
		t.Data = &ClaimTX{
			Claims: tx.Claims,
		}
	case InvocationType:
		bytes, err := hex.DecodeString(tx.Script)
		if err != nil {
			return err
		}
		t.Data = &InvocationTX{
			Script:  bytes,
			Gas:     tx.Gas,
			Version: tx.Version,
		}
	case RegisterType:
		admin, err := address.StringToUint160(tx.Asset.Admin)
		if err != nil {
			return err
		}
		t.Data = &RegisterTX{
			AssetType: tx.Asset.AssetType,
			Name:      string(tx.Asset.Name),
			Amount:    tx.Asset.Amount,
			Precision: tx.Asset.Precision,
			Owner:     tx.Asset.Owner,
			Admin:     admin,
		}
	case ContractType:
		t.Data = &ContractTX{}
	case IssueType:
		t.Data = &IssueTX{}
	}
	if t.Hash() != tx.TxID {
		return errors.New("txid doesn't match transaction hash")
	}

	return nil
}
