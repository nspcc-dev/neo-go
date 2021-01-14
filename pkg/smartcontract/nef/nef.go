package nef

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// NEO Executable Format 3 (NEF3)
// Standard: https://github.com/neo-project/proposals/pull/121/files
// Implementation: https://github.com/neo-project/neo/blob/v3.0.0-preview2/src/neo/SmartContract/NefFile.cs#L8
// +------------+-----------+------------------------------------------------------------+
// |   Field    |  Length   |                          Comment                           |
// +------------+-----------+------------------------------------------------------------+
// | Magic      | 4 bytes   | Magic header                                               |
// | Compiler   | 32 bytes  | Compiler used                                              |
// | Version    | 32 bytes  | Compiler version                                           |
// +------------+-----------+------------------------------------------------------------+
// | Script     | Var bytes | Var bytes for the payload                                  |
// +------------+-----------+------------------------------------------------------------+
// | Checksum   | 4 bytes   | First four bytes of double SHA256 hash of the header       |
// +------------+-----------+------------------------------------------------------------+

const (
	// Magic is a magic File header constant.
	Magic uint32 = 0x3346454E
	// MaxScriptLength is the maximum allowed contract script length.
	MaxScriptLength = 512 * 1024
	// compilerFieldSize is the length of `Compiler` and `Version` File header fields in bytes.
	compilerFieldSize = 32
)

// File represents compiled contract file structure according to the NEF3 standard.
type File struct {
	Header
	Script   []byte `json:"script"`
	Checksum uint32 `json:"checksum"`
}

// Header represents File header.
type Header struct {
	Magic    uint32 `json:"magic"`
	Compiler string `json:"compiler"`
	Version  string `json:"version"`
}

// NewFile returns new NEF3 file with script specified.
func NewFile(script []byte) (*File, error) {
	file := &File{
		Header: Header{
			Magic:    Magic,
			Compiler: "neo-go",
			Version:  config.Version,
		},
		Script: script,
	}
	if len(config.Version) > compilerFieldSize {
		return nil, errors.New("too long version")
	}
	file.Checksum = file.CalculateChecksum()
	return file, nil
}

// EncodeBinary implements io.Serializable interface.
func (h *Header) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(h.Magic)
	if len(h.Compiler) > compilerFieldSize {
		w.Err = errors.New("invalid compiler name length")
		return
	}
	var b = make([]byte, compilerFieldSize)
	copy(b, []byte(h.Compiler))
	w.WriteBytes(b)
	for i := range b {
		b[i] = 0
	}
	copy(b, []byte(h.Version))
	w.WriteBytes(b)
}

// DecodeBinary implements io.Serializable interface.
func (h *Header) DecodeBinary(r *io.BinReader) {
	h.Magic = r.ReadU32LE()
	if h.Magic != Magic {
		r.Err = errors.New("invalid Magic")
		return
	}
	buf := make([]byte, compilerFieldSize)
	r.ReadBytes(buf)
	buf = bytes.TrimRightFunc(buf, func(r rune) bool {
		return r == 0
	})
	h.Compiler = string(buf)
	buf = buf[:compilerFieldSize]
	r.ReadBytes(buf)
	buf = bytes.TrimRightFunc(buf, func(r rune) bool {
		return r == 0
	})
	h.Version = string(buf)
}

// CalculateChecksum returns first 4 bytes of double-SHA256(Header) converted to uint32.
func (n *File) CalculateChecksum() uint32 {
	bb, err := n.Bytes()
	if err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint32(hash.Checksum(bb[:len(bb)-4]))
}

// EncodeBinary implements io.Serializable interface.
func (n *File) EncodeBinary(w *io.BinWriter) {
	n.Header.EncodeBinary(w)
	w.WriteVarBytes(n.Script)
	w.WriteU32LE(n.Checksum)
}

// DecodeBinary implements io.Serializable interface.
func (n *File) DecodeBinary(r *io.BinReader) {
	n.Header.DecodeBinary(r)
	n.Script = r.ReadVarBytes(MaxScriptLength)
	if len(n.Script) == 0 {
		r.Err = errors.New("empty script")
		return
	}
	n.Checksum = r.ReadU32LE()
	checksum := n.CalculateChecksum()
	if checksum != n.Checksum {
		r.Err = errors.New("checksum verification failure")
		return
	}
}

// Bytes returns byte array with serialized NEF File.
func (n File) Bytes() ([]byte, error) {
	buf := io.NewBufBinWriter()
	n.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil, buf.Err
	}
	return buf.Bytes(), nil
}

// FileFromBytes returns NEF File deserialized from given bytes.
func FileFromBytes(source []byte) (File, error) {
	result := File{}
	r := io.NewBinReaderFromBuf(source)
	result.DecodeBinary(r)
	if r.Err != nil {
		return result, r.Err
	}
	return result, nil
}
