package nef

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NEO Executable Format 3 (NEF3)
// Standard: https://github.com/neo-project/proposals/pull/121/files
// Implementation: https://github.com/neo-project/neo/blob/v3.0.0-preview2/src/neo/SmartContract/NefFile.cs#L8
// +------------+-----------+------------------------------------------------------------+
// |   Field    |  Length   |                          Comment                           |
// +------------+-----------+------------------------------------------------------------+
// | Magic      | 4 bytes   | Magic header                                               |
// | Compiler   | 32 bytes  | Compiler used                                              |
// | Version    | 16 bytes  | Compiler version (Major, Minor, Build, Version)            |
// | ScriptHash | 20 bytes  | ScriptHash for the script (BE)                             |
// +------------+-----------+------------------------------------------------------------+
// | Checksum   | 4 bytes   | Sha256 of the header (CRC)                                 |
// +------------+-----------+------------------------------------------------------------+
// | Script     | Var bytes | Var bytes for the payload                                  |
// +------------+-----------+------------------------------------------------------------+

const (
	// Magic is a magic File header constant.
	Magic uint32 = 0x3346454E
	// MaxScriptLength is the maximum allowed contract script length.
	MaxScriptLength = 1024 * 1024
	// compilerFieldSize is the length of `Compiler` File header field in bytes.
	compilerFieldSize = 32
)

// File represents compiled contract file structure according to the NEF3 standard.
type File struct {
	Header   Header
	Checksum uint32
	Script   []byte
}

// Header represents File header.
type Header struct {
	Magic      uint32
	Compiler   string
	Version    Version
	ScriptHash util.Uint160
}

// Version represents compiler version.
type Version struct {
	Major    int32
	Minor    int32
	Build    int32
	Revision int32
}

// NewFile returns new NEF3 file with script specified.
func NewFile(script []byte) (File, error) {
	file := File{
		Header: Header{
			Magic:      Magic,
			Compiler:   "neo-go",
			ScriptHash: hash.Hash160(script),
		},
		Script: script,
	}
	v, err := GetVersion(config.Version)
	if err != nil {
		return file, err
	}
	file.Header.Version = v
	file.Checksum = file.Header.CalculateChecksum()
	return file, nil
}

// GetVersion returns Version from the given string. It accepts the following formats:
// `major.minor.build-[...]`
// `major.minor.build.revision-[...]`
// where `major`, `minor`, `build` and `revision` are 32-bit integers with base=10
func GetVersion(version string) (Version, error) {
	var (
		result Version
	)
	versions := strings.SplitN(version, ".", 4)
	if len(versions) < 3 {
		return result, errors.New("invalid version format")
	}
	major, err := strconv.ParseInt(versions[0], 10, 32)
	if err != nil {
		return result, fmt.Errorf("failed to parse major version: %v", err)
	}
	result.Major = int32(major)

	minor, err := strconv.ParseInt(versions[1], 10, 32)
	if err != nil {
		return result, fmt.Errorf("failed to parse minor version: %v", err)

	}
	result.Minor = int32(minor)

	b := versions[2]
	if len(versions) == 3 {
		b = strings.SplitN(b, "-", 2)[0]
	}
	build, err := strconv.ParseInt(b, 10, 32)
	if err != nil {
		return result, fmt.Errorf("failed to parse build version: %v", err)
	}
	result.Build = int32(build)

	if len(versions) == 4 {
		r := strings.SplitN(versions[3], "-", 2)[0]
		revision, err := strconv.ParseInt(r, 10, 32)
		if err != nil {
			return result, fmt.Errorf("failed to parse revision version: %v", err)
		}
		result.Revision = int32(revision)
	}

	return result, nil
}

// EncodeBinary implements io.Serializable interface.
func (v *Version) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(uint32(v.Major))
	w.WriteU32LE(uint32(v.Minor))
	w.WriteU32LE(uint32(v.Build))
	w.WriteU32LE(uint32(v.Revision))
}

// DecodeBinary implements io.Serializable interface.
func (v *Version) DecodeBinary(r *io.BinReader) {
	v.Major = int32(r.ReadU32LE())
	v.Minor = int32(r.ReadU32LE())
	v.Build = int32(r.ReadU32LE())
	v.Revision = int32(r.ReadU32LE())
}

// EncodeBinary implements io.Serializable interface.
func (h *Header) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(h.Magic)
	if len(h.Compiler) > compilerFieldSize {
		w.Err = errors.New("invalid compiler name length")
		return
	}
	bytes := []byte(h.Compiler)
	w.WriteBytes(bytes)
	if len(bytes) < compilerFieldSize {
		w.WriteBytes(make([]byte, compilerFieldSize-len(bytes)))
	}
	h.Version.EncodeBinary(w)
	h.ScriptHash.EncodeBinary(w)
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
	h.Version.DecodeBinary(r)
	h.ScriptHash.DecodeBinary(r)
}

// CalculateChecksum returns first 4 bytes of SHA256(Header) converted to uint32.
func (h *Header) CalculateChecksum() uint32 {
	buf := io.NewBufBinWriter()
	h.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		panic(buf.Err)
	}
	return binary.LittleEndian.Uint32(hash.Sha256(buf.Bytes()).BytesBE())
}

// EncodeBinary implements io.Serializable interface.
func (n *File) EncodeBinary(w *io.BinWriter) {
	n.Header.EncodeBinary(w)
	w.WriteU32LE(n.Checksum)
	w.WriteVarBytes(n.Script)
}

// DecodeBinary implements io.Serializable interface.
func (n *File) DecodeBinary(r *io.BinReader) {
	n.Header.DecodeBinary(r)
	n.Checksum = r.ReadU32LE()
	checksum := n.Header.CalculateChecksum()
	if checksum != n.Checksum {
		r.Err = errors.New("CRC verification fail")
		return
	}
	n.Script = r.ReadVarBytes()
	if len(n.Script) > MaxScriptLength {
		r.Err = errors.New("invalid script length")
		return
	}
	if !hash.Hash160(n.Script).Equals(n.Header.ScriptHash) {
		r.Err = errors.New("script hashes mismatch")
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
