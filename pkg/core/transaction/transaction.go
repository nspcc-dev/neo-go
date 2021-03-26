package transaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// MaxScriptLength is the limit for transaction's script length.
	MaxScriptLength = math.MaxUint16
	// MaxTransactionSize is the upper limit size in bytes that a transaction can reach. It is
	// set to be 102400.
	MaxTransactionSize = 102400
	// MaxValidUntilBlockIncrement is the upper increment size of blockhain height in blocks
	// exceeding that a transaction should fail validation. It is set to estimated daily number
	// of blocks with 15s interval.
	MaxValidUntilBlockIncrement = 5760
	// MaxAttributes is maximum number of attributes including signers that can be contained
	// within a transaction. It is set to be 16.
	MaxAttributes = 16
	// DummyVersion represents reserved transaction version for trimmed transactions.
	DummyVersion = 255
)

// ErrInvalidWitnessNum returns when the number of witnesses does not match signers.
var ErrInvalidWitnessNum = errors.New("number of signers doesn't match witnesses")

// Transaction is a process recorded in the NEO blockchain.
type Transaction struct {
	// The trading version which is currently 0.
	Version uint8

	// Random number to avoid hash collision.
	Nonce uint32

	// Fee to be burned.
	SystemFee int64

	// Fee to be distributed to consensus nodes.
	NetworkFee int64

	// Maximum blockchain height exceeding which
	// transaction should fail verification.
	ValidUntilBlock uint32

	// Code to run in NeoVM for this transaction.
	Script []byte

	// Transaction attributes.
	Attributes []Attribute

	// Transaction signers list (starts with Sender).
	Signers []Signer

	// The scripts that comes with this transaction.
	// Scripts exist out of the verification script
	// and invocation script.
	Scripts []Witness

	// size is transaction's serialized size.
	size int

	// Hash of the transaction (double SHA256).
	hash util.Uint256

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

// New returns a new transaction to execute given script and pay given system
// fee.
func New(script []byte, gas int64) *Transaction {
	return &Transaction{
		Version:    0,
		Nonce:      rand.Uint32(),
		Script:     script,
		SystemFee:  gas,
		Attributes: []Attribute{},
		Signers:    []Signer{},
		Scripts:    []Witness{},
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

// HasAttribute returns true iff t has an attribute of type typ.
func (t *Transaction) HasAttribute(typ AttrType) bool {
	for i := range t.Attributes {
		if t.Attributes[i].Type == typ {
			return true
		}
	}
	return false
}

// GetAttributes returns the list of transaction's attributes of the given type.
// Returns nil in case if attributes not found.
func (t *Transaction) GetAttributes(typ AttrType) []Attribute {
	var result []Attribute
	for _, attr := range t.Attributes {
		if attr.Type == typ {
			result = append(result, attr)
		}
	}
	return result
}

// decodeHashableFields decodes the fields that are used for signing the
// transaction, which are all fields except the scripts.
func (t *Transaction) decodeHashableFields(br *io.BinReader) {
	t.Version = uint8(br.ReadB())
	t.Nonce = br.ReadU32LE()
	t.SystemFee = int64(br.ReadU64LE())
	t.NetworkFee = int64(br.ReadU64LE())
	t.ValidUntilBlock = br.ReadU32LE()
	br.ReadArray(&t.Signers, MaxAttributes)
	br.ReadArray(&t.Attributes, MaxAttributes-len(t.Signers))
	t.Script = br.ReadVarBytes(MaxScriptLength)
	if br.Err == nil {
		br.Err = t.isValid()
	}
}

// DecodeBinary implements Serializable interface.
func (t *Transaction) DecodeBinary(br *io.BinReader) {
	t.decodeHashableFields(br)
	if br.Err != nil {
		return
	}
	br.ReadArray(&t.Scripts, len(t.Signers))
	if len(t.Signers) != len(t.Scripts) {
		br.Err = fmt.Errorf("%w: %d vs %d", ErrInvalidWitnessNum, len(t.Signers), len(t.Scripts))
		return
	}

	// Create the hash of the transaction at decode, so we dont need
	// to do it anymore.
	if br.Err == nil {
		br.Err = t.createHash()
		_ = t.Size()
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
	if len(t.Script) == 0 {
		bw.Err = errors.New("transaction has no script")
		return
	}
	bw.WriteB(byte(t.Version))
	bw.WriteU32LE(t.Nonce)
	bw.WriteU64LE(uint64(t.SystemFee))
	bw.WriteU64LE(uint64(t.NetworkFee))
	bw.WriteU32LE(t.ValidUntilBlock)
	bw.WriteArray(t.Signers)
	bw.WriteArray(t.Attributes)
	bw.WriteVarBytes(t.Script)
}

// EncodeHashableFields returns serialized transaction's fields which are hashed.
func (t *Transaction) EncodeHashableFields() ([]byte, error) {
	bw := io.NewBufBinWriter()
	t.encodeHashableFields(bw.BinWriter)
	if bw.Err != nil {
		return nil, bw.Err
	}
	return bw.Bytes(), nil
}

// createHash creates the hash of the transaction.
func (t *Transaction) createHash() error {
	buf := io.NewBufBinWriter()
	t.encodeHashableFields(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}

	t.hash = hash.Sha256(buf.Bytes())
	return nil
}

// DecodeHashableFields decodes a part of transaction which should be hashed.
func (t *Transaction) DecodeHashableFields(buf []byte) error {
	r := io.NewBinReaderFromBuf(buf)
	t.decodeHashableFields(r)
	if r.Err != nil {
		return r.Err
	}
	// Ensure all the data was read.
	_ = r.ReadB()
	if r.Err == nil {
		return errors.New("additional data after the signed part")
	}
	t.Scripts = make([]Witness, 0)

	t.hash = hash.Sha256(buf)
	return nil
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
	_ = r.ReadB()
	if r.Err == nil {
		return nil, errors.New("additional data after the transaction")
	}
	tx.size = len(b)
	return tx, nil
}

// FeePerByte returns NetworkFee of the transaction divided by
// its size
func (t *Transaction) FeePerByte() int64 {
	return t.NetworkFee / int64(t.Size())
}

// Size returns size of the serialized transaction.
func (t *Transaction) Size() int {
	if t.size == 0 {
		t.size = io.GetVarSize(t)
	}
	return t.size
}

// Sender returns the sender of the transaction which is always on the first place
// in the transaction's signers list.
func (t *Transaction) Sender() util.Uint160 {
	if len(t.Signers) == 0 {
		panic("transaction does not have signers")
	}
	return t.Signers[0].Account
}

// transactionJSON is a wrapper for Transaction and
// used for correct marhalling of transaction.Data
type transactionJSON struct {
	TxID            util.Uint256 `json:"hash"`
	Size            int          `json:"size"`
	Version         uint8        `json:"version"`
	Nonce           uint32       `json:"nonce"`
	Sender          string       `json:"sender"`
	SystemFee       int64        `json:"sysfee,string"`
	NetworkFee      int64        `json:"netfee,string"`
	ValidUntilBlock uint32       `json:"validuntilblock"`
	Attributes      []Attribute  `json:"attributes"`
	Signers         []Signer     `json:"signers"`
	Script          []byte       `json:"script"`
	Scripts         []Witness    `json:"witnesses"`
}

// MarshalJSON implements json.Marshaler interface.
func (t *Transaction) MarshalJSON() ([]byte, error) {
	tx := transactionJSON{
		TxID:            t.Hash(),
		Size:            t.Size(),
		Version:         t.Version,
		Nonce:           t.Nonce,
		Sender:          address.Uint160ToString(t.Sender()),
		ValidUntilBlock: t.ValidUntilBlock,
		Attributes:      t.Attributes,
		Signers:         t.Signers,
		Script:          t.Script,
		Scripts:         t.Scripts,
		SystemFee:       t.SystemFee,
		NetworkFee:      t.NetworkFee,
	}
	return json.Marshal(tx)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (t *Transaction) UnmarshalJSON(data []byte) error {
	tx := new(transactionJSON)
	if err := json.Unmarshal(data, tx); err != nil {
		return err
	}
	t.Version = tx.Version
	t.Nonce = tx.Nonce
	t.ValidUntilBlock = tx.ValidUntilBlock
	t.Attributes = tx.Attributes
	t.Signers = tx.Signers
	t.Scripts = tx.Scripts
	t.SystemFee = tx.SystemFee
	t.NetworkFee = tx.NetworkFee
	t.Script = tx.Script
	if t.Hash() != tx.TxID {
		return errors.New("txid doesn't match transaction hash")
	}
	if t.Size() != tx.Size {
		return errors.New("'size' doesn't match transaction size")
	}

	return t.isValid()
}

// Various errors for transaction validation.
var (
	ErrInvalidVersion     = errors.New("only version 0 is supported")
	ErrNegativeSystemFee  = errors.New("negative system fee")
	ErrNegativeNetworkFee = errors.New("negative network fee")
	ErrTooBigFees         = errors.New("too big fees: int64 overflow")
	ErrEmptySigners       = errors.New("signers array should contain sender")
	ErrNonUniqueSigners   = errors.New("transaction signers should be unique")
	ErrInvalidAttribute   = errors.New("invalid attribute")
	ErrEmptyScript        = errors.New("no script")
)

// isValid checks whether decoded/unmarshalled transaction has all fields valid.
func (t *Transaction) isValid() error {
	if t.Version > 0 && t.Version != DummyVersion {
		return ErrInvalidVersion
	}
	if t.SystemFee < 0 {
		return ErrNegativeSystemFee
	}
	if t.NetworkFee < 0 {
		return ErrNegativeNetworkFee
	}
	if t.NetworkFee+t.SystemFee < t.SystemFee {
		return ErrTooBigFees
	}
	if len(t.Signers) == 0 {
		return ErrEmptySigners
	}
	for i := 0; i < len(t.Signers); i++ {
		for j := i + 1; j < len(t.Signers); j++ {
			if t.Signers[i].Account.Equals(t.Signers[j].Account) {
				return ErrNonUniqueSigners
			}
		}
	}
	attrs := map[AttrType]bool{}
	for i := range t.Attributes {
		typ := t.Attributes[i].Type
		if !typ.allowMultiple() {
			if attrs[typ] {
				return fmt.Errorf("%w: multiple '%s' attributes", ErrInvalidAttribute, typ.String())
			}
			attrs[typ] = true
		}
	}
	if len(t.Script) == 0 {
		return ErrEmptyScript
	}
	return nil
}

// HasSigner returns true in case if hash is present in the list of signers.
func (t *Transaction) HasSigner(hash util.Uint160) bool {
	for _, h := range t.Signers {
		if h.Account.Equals(hash) {
			return true
		}
	}
	return false
}
