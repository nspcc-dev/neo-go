package nef

import (
	"errors"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// maxMethodLength is the maximum length of method.
const maxMethodLength = 32

var (
	errInvalidMethodName = errors.New("method name should't start with '_'")
	errInvalidCallFlag   = errors.New("invalid call flag")
)

// MethodToken is contract method description.
type MethodToken struct {
	// Hash is contract hash.
	Hash util.Uint160 `json:"hash"`
	// Method is method name.
	Method string `json:"method"`
	// ParamCount is method parameter count.
	ParamCount uint16 `json:"paramcount"`
	// HasReturn is true if method returns value.
	HasReturn bool `json:"hasreturnvalue"`
	// CallFlag is a set of call flags the method will be called with.
	CallFlag callflag.CallFlag `json:"callflags"`
}

// EncodeBinary implements io.Serializable.
func (t *MethodToken) EncodeBinary(w io.BinaryWriter) {
	w.WriteBytes(t.Hash[:])
	w.WriteString(t.Method)
	w.WriteU16LE(t.ParamCount)
	w.WriteBool(t.HasReturn)
	w.WriteB(byte(t.CallFlag))
}

// DecodeBinary implements io.Serializable.
func (t *MethodToken) DecodeBinary(r io.BinaryReader) {
	r.ReadBytes(t.Hash[:])
	t.Method = r.ReadString(maxMethodLength)
	if r.Error() == nil && strings.HasPrefix(t.Method, "_") {
		r.SetError(errInvalidMethodName)
		return
	}
	t.ParamCount = r.ReadU16LE()
	t.HasReturn = r.ReadBool()
	t.CallFlag = callflag.CallFlag(r.ReadB())
	if r.Error() == nil && t.CallFlag&^callflag.All != 0 {
		r.SetError(errInvalidCallFlag)
	}
}
