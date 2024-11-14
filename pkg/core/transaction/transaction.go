package transaction

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/bitfield"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MaxScriptLength is the limit for transaction's script length.
	MaxScriptLength = math.MaxUint16
	// MaxTransactionSize is the upper limit size in bytes that a transaction can reach. It is
	// set to be 102400.
	MaxTransactionSize = 102400
	// MaxAttributes is maximum number of attributes including signers that can be contained
	// within a transaction. It is set to be 16.
	MaxAttributes = 16
)

// ErrInvalidWitnessNum returns when the number of witnesses does not match signers.
var ErrInvalidWitnessNum = errors.New("number of signers doesn't match witnesses")

// Transaction is a process recorded in the Neo blockchain.
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
	// transaction should fail verification. E.g. if VUB=N, then transaction
	// can be accepted to block with index N, but not to block with index N+1.
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

	// Whether hash is correct.
	hashed bool

	// Trimmed indicates this is a transaction from trimmed
	// data, meaning it doesn't have anything but hash.
	Trimmed bool
}

// NewTrimmedTX returns a trimmed transaction with only its hash
// and Trimmed to true.
func NewTrimmedTX(hash util.Uint256) *Transaction {
	return &Transaction{
		hash:    hash,
		hashed:  true,
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

// Hash returns the hash of the transaction which is based on serialized
// representation of its fields. Notice that this hash is cached internally
// in [Transaction] for efficiency, so once you call this method it will not
// change even if you change any structure fields. If you need to update the
// hash use encoding/decoding or [Transaction.Copy].
func (t *Transaction) Hash() util.Uint256 {
	if !t.hashed {
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
func (t *Transaction) decodeHashableFields(br *io.BinReader, buf []byte) {
	var start, end int

	if buf != nil {
		start = len(buf) - br.Len()
	}
	t.Version = uint8(br.ReadB())
	t.Nonce = br.ReadU32LE()
	t.SystemFee = int64(br.ReadU64LE())
	t.NetworkFee = int64(br.ReadU64LE())
	t.ValidUntilBlock = br.ReadU32LE()
	nsigners := br.ReadVarUint()
	if br.Err != nil {
		return
	}
	if nsigners > MaxAttributes {
		br.Err = errors.New("too many signers")
		return
	} else if nsigners == 0 {
		br.Err = errors.New("missing signers")
		return
	}
	t.Signers = make([]Signer, nsigners)
	for i := range t.Signers {
		t.Signers[i].DecodeBinary(br)
	}
	nattrs := br.ReadVarUint()
	if nattrs > MaxAttributes-nsigners {
		br.Err = errors.New("too many attributes")
		return
	}
	t.Attributes = make([]Attribute, nattrs)
	for i := range t.Attributes {
		t.Attributes[i].DecodeBinary(br)
	}
	t.Script = br.ReadVarBytes(MaxScriptLength)
	if br.Err == nil {
		br.Err = t.isValid()
	}
	if buf != nil {
		end = len(buf) - br.Len()
		t.hash = hash.Sha256(buf[start:end])
		t.hashed = true
	}
}

func (t *Transaction) decodeBinaryNoSize(br *io.BinReader, buf []byte) {
	t.decodeHashableFields(br, buf)
	if br.Err != nil {
		return
	}
	nscripts := br.ReadVarUint()
	if nscripts > MaxAttributes {
		br.Err = errors.New("too many witnesses")
		return
	} else if int(nscripts) != len(t.Signers) {
		br.Err = fmt.Errorf("%w: %d vs %d", ErrInvalidWitnessNum, len(t.Signers), nscripts)
		return
	}
	t.Scripts = make([]Witness, nscripts)
	for i := range t.Scripts {
		t.Scripts[i].DecodeBinary(br)
	}

	// Create the hash of the transaction at decode, so we dont need
	// to do it anymore.
	if br.Err == nil && buf == nil {
		br.Err = t.createHash()
	}
}

// DecodeBinary implements the [io.Serializable] interface. It also
// computes and caches transaction hash and size (see [Transaction.Hash] and
// [Transaction.Size]).
func (t *Transaction) DecodeBinary(br *io.BinReader) {
	t.decodeBinaryNoSize(br, nil)

	if br.Err == nil {
		_ = t.Size()
	}
}

// EncodeBinary implements the Serializable interface.
func (t *Transaction) EncodeBinary(bw *io.BinWriter) {
	t.encodeHashableFields(bw)
	bw.WriteVarUint(uint64(len(t.Scripts)))
	for i := range t.Scripts {
		t.Scripts[i].EncodeBinary(bw)
	}
}

// encodeHashableFields encodes the fields that are not used for
// signing the transaction, which are all fields except the scripts.
func (t *Transaction) encodeHashableFields(bw *io.BinWriter) {
	bw.WriteB(byte(t.Version))
	bw.WriteU32LE(t.Nonce)
	bw.WriteU64LE(uint64(t.SystemFee))
	bw.WriteU64LE(uint64(t.NetworkFee))
	bw.WriteU32LE(t.ValidUntilBlock)
	bw.WriteVarUint(uint64(len(t.Signers)))
	for i := range t.Signers {
		t.Signers[i].EncodeBinary(bw)
	}
	bw.WriteVarUint(uint64(len(t.Attributes)))
	for i := range t.Attributes {
		t.Attributes[i].EncodeBinary(bw)
	}
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
	shaHash := sha256.New()
	bw := io.NewBinWriterFromIO(shaHash)
	t.encodeHashableFields(bw)
	if bw.Err != nil {
		return bw.Err
	}

	shaHash.Sum(t.hash[:0])
	t.hashed = true
	return nil
}

// DecodeHashableFields decodes a part of transaction which should be hashed.
func (t *Transaction) DecodeHashableFields(buf []byte) error {
	r := io.NewBinReaderFromBuf(buf)
	t.decodeHashableFields(r, buf)
	if r.Err != nil {
		return r.Err
	}
	// Ensure all the data was read.
	if r.Len() != 0 {
		return errors.New("additional data after the signed part")
	}
	t.Scripts = make([]Witness, 0)
	return nil
}

// Bytes converts the transaction to []byte.
func (t *Transaction) Bytes() []byte {
	buf := io.NewBufBinWriter()
	t.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil
	}
	return buf.Bytes()
}

// NewTransactionFromBytes decodes byte array into [*Transaction]. It also
// computes and caches transaction hash and size (see [Transaction.Hash] and
// [Transaction.Size]).
func NewTransactionFromBytes(b []byte) (*Transaction, error) {
	tx := &Transaction{}
	r := io.NewBinReaderFromBuf(b)
	tx.decodeBinaryNoSize(r, b)
	if r.Err != nil {
		return nil, r.Err
	}
	if r.Len() != 0 {
		return nil, errors.New("additional data after the transaction")
	}
	tx.size = len(b)
	return tx, nil
}

// FeePerByte returns NetworkFee of the transaction divided by
// its size.
func (t *Transaction) FeePerByte() int64 {
	return t.NetworkFee / int64(t.Size())
}

// Size returns size of the serialized transaction. This value is cached
// in the [Transaction], so once you obtain it no changes to fields will be
// reflected in value returned from this method, use encoding/decoding or
// [Transaction.Copy] if needed.
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
// used for correct marhalling of transaction.Data.
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

// MarshalJSON implements the json.Marshaler interface.
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

// UnmarshalJSON implements the json.Unmarshaler interface.
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
	if t.Version > 0 {
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
	for i := range t.Signers {
		for j := i + 1; j < len(t.Signers); j++ {
			if t.Signers[i].Account.Equals(t.Signers[j].Account) {
				return ErrNonUniqueSigners
			}
		}
	}
	var attrBits = bitfield.New(256)
	for i := range t.Attributes {
		typ := t.Attributes[i].Type
		if !typ.allowMultiple() {
			if attrBits.IsSet(int(typ)) {
				return fmt.Errorf("%w: multiple '%s' attributes", ErrInvalidAttribute, typ.String())
			}
			attrBits.Set(int(typ))
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

// ToStackItem converts Transaction to stackitem.Item.
func (t *Transaction) ToStackItem() stackitem.Item {
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(t.Hash().BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(t.Version))),
		stackitem.NewBigInteger(big.NewInt(int64(t.Nonce))),
		stackitem.NewByteArray(t.Sender().BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(t.SystemFee))),
		stackitem.NewBigInteger(big.NewInt(int64(t.NetworkFee))),
		stackitem.NewBigInteger(big.NewInt(int64(t.ValidUntilBlock))),
		stackitem.NewByteArray(t.Script),
	})
}

// Copy creates a deep copy of the Transaction, including all slice fields.
// Cached values like hash and size are reset to ensure the copy can be
// modified independently of the original (see [Transaction.Hash] and
// [Transaction.Size]).
func (t *Transaction) Copy() *Transaction {
	if t == nil {
		return nil
	}
	cp := *t
	if t.Attributes != nil {
		cp.Attributes = make([]Attribute, len(t.Attributes))
		for i, attr := range t.Attributes {
			cp.Attributes[i] = *attr.Copy()
		}
	}
	if t.Signers != nil {
		cp.Signers = make([]Signer, len(t.Signers))
		for i, signer := range t.Signers {
			cp.Signers[i] = *signer.Copy()
		}
	}
	if t.Scripts != nil {
		cp.Scripts = make([]Witness, len(t.Scripts))
		for i, script := range t.Scripts {
			cp.Scripts[i] = script.Copy()
		}
	}
	cp.Script = bytes.Clone(t.Script)

	cp.hashed = false
	cp.size = 0
	cp.hash = util.Uint256{}
	return &cp
}
