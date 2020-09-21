package transaction

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// OracleResponseCode represents result code of oracle response.
type OracleResponseCode byte

// OracleResponse represents oracle response.
type OracleResponse struct {
	ID     uint64             `json:"id"`
	Code   OracleResponseCode `json:"code"`
	Result []byte             `json:"result"`
}

const maxResultSize = 1024

// Enumeration of possible oracle response types.
const (
	Success   OracleResponseCode = 0x00
	NotFound  OracleResponseCode = 0x10
	Timeout   OracleResponseCode = 0x12
	Forbidden OracleResponseCode = 0x14
	Error     OracleResponseCode = 0xff
)

// Various validation errors.
var (
	ErrInvalidResponseCode = errors.New("invalid oracle response code")
	ErrInvalidResult       = errors.New("oracle response != success, but result is not empty")
)

// IsValid checks if c is valid response code.
func (c OracleResponseCode) IsValid() bool {
	return c == Success || c == NotFound || c == Timeout || c == Forbidden || c == Error
}

// DecodeBinary implements io.Serializable interface.
func (r *OracleResponse) DecodeBinary(br *io.BinReader) {
	r.ID = br.ReadU64LE()
	r.Code = OracleResponseCode(br.ReadB())
	if !r.Code.IsValid() {
		br.Err = ErrInvalidResponseCode
		return
	}
	r.Result = br.ReadVarBytes(maxResultSize)
	if r.Code != Success && len(r.Result) > 0 {
		br.Err = ErrInvalidResult
	}
}

// EncodeBinary implements io.Serializable interface.
func (r *OracleResponse) EncodeBinary(w *io.BinWriter) {
	w.WriteU64LE(r.ID)
	w.WriteB(byte(r.Code))
	w.WriteVarBytes(r.Result)
}

func (r *OracleResponse) toJSONMap(m map[string]interface{}) {
	m["id"] = r.ID
	m["code"] = r.Code
	m["result"] = r.Result
}
