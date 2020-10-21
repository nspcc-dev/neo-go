package state

import (
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MPTRootBase represents storage state root.
type MPTRootBase struct {
	Version  byte         `json:"version"`
	Index    uint32       `json:"index"`
	PrevHash util.Uint256 `json:"prehash"`
	Root     util.Uint256 `json:"stateroot"`
}

// MPTRoot represents storage state root together with sign info.
type MPTRoot struct {
	MPTRootBase
	Witness *transaction.Witness `json:"witness,omitempty"`
}

// MPTRootStateFlag represents verification state of the state root.
type MPTRootStateFlag byte

// Possible verification states of MPTRoot.
const (
	Unverified MPTRootStateFlag = 0x00
	Verified   MPTRootStateFlag = 0x01
	Invalid    MPTRootStateFlag = 0x03
)

// MPTRootState represents state root together with its verification state.
type MPTRootState struct {
	MPTRoot `json:"stateroot"`
	Flag    MPTRootStateFlag `json:"flag"`
}

// EncodeBinary implements io.Serializable.
func (s *MPTRootState) EncodeBinary(w *io.BinWriter) {
	w.WriteB(byte(s.Flag))
	s.MPTRoot.EncodeBinary(w)
}

// DecodeBinary implements io.Serializable.
func (s *MPTRootState) DecodeBinary(r *io.BinReader) {
	s.Flag = MPTRootStateFlag(r.ReadB())
	s.MPTRoot.DecodeBinary(r)
}

// GetSignedPart returns part of MPTRootBase which needs to be signed.
func (s *MPTRootBase) GetSignedPart() []byte {
	buf := io.NewBufBinWriter()
	s.EncodeBinary(buf.BinWriter)
	return buf.Bytes()
}

// GetSignedHash returns hash of MPTRootBase which needs to be signed.
func (s *MPTRootBase) GetSignedHash() util.Uint256 {
	buf := io.NewBufBinWriter()
	s.EncodeBinary(buf.BinWriter)
	return hash.Sha256(buf.Bytes())
}

// Equals checks if s == other.
func (s *MPTRootBase) Equals(other *MPTRootBase) bool {
	return s.Version == other.Version && s.Index == other.Index &&
		s.PrevHash.Equals(other.PrevHash) && s.Root.Equals(other.Root)
}

// Hash returns hash of s.
func (s *MPTRootBase) Hash() util.Uint256 {
	return hash.DoubleSha256(s.GetSignedPart())
}

// DecodeBinary implements io.Serializable.
func (s *MPTRootBase) DecodeBinary(r *io.BinReader) {
	s.Version = r.ReadB()
	s.Index = r.ReadU32LE()
	s.PrevHash.DecodeBinary(r)
	s.Root.DecodeBinary(r)
}

// EncodeBinary implements io.Serializable.
func (s *MPTRootBase) EncodeBinary(w *io.BinWriter) {
	w.WriteB(s.Version)
	w.WriteU32LE(s.Index)
	s.PrevHash.EncodeBinary(w)
	s.Root.EncodeBinary(w)
}

// DecodeBinary implements io.Serializable.
func (s *MPTRoot) DecodeBinary(r *io.BinReader) {
	s.MPTRootBase.DecodeBinary(r)

	var ws []transaction.Witness
	r.ReadArray(&ws, 1)
	if len(ws) == 1 {
		s.Witness = &ws[0]
	}
}

// EncodeBinary implements io.Serializable.
func (s *MPTRoot) EncodeBinary(w *io.BinWriter) {
	s.MPTRootBase.EncodeBinary(w)
	if s.Witness == nil {
		w.WriteVarUint(0)
	} else {
		w.WriteArray([]*transaction.Witness{s.Witness})
	}
}

// String implements fmt.Stringer.
func (f MPTRootStateFlag) String() string {
	switch f {
	case Unverified:
		return "Unverified"
	case Verified:
		return "Verified"
	case Invalid:
		return "Invalid"
	default:
		return ""
	}
}

// MarshalJSON implements json.Marshaler.
func (f MPTRootStateFlag) MarshalJSON() ([]byte, error) {
	return []byte(`"` + f.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (f *MPTRootStateFlag) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "Unverified":
		*f = Unverified
	case "Verified":
		*f = Verified
	case "Invalid":
		*f = Invalid
	default:
		return errors.New("unknown flag")
	}
	return nil
}
