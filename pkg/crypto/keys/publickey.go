package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	lru "github.com/hashicorp/golang-lru"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// coordLen is the number of bytes in serialized X or Y coordinate.
const coordLen = 32

// SignatureLen is the length of standard signature for 256-bit EC key.
const SignatureLen = 64

// PublicKeys is a list of public keys.
type PublicKeys []*PublicKey

func (keys PublicKeys) Len() int      { return len(keys) }
func (keys PublicKeys) Swap(i, j int) { keys[i], keys[j] = keys[j], keys[i] }
func (keys PublicKeys) Less(i, j int) bool {
	return keys[i].Cmp(keys[j]) == -1
}

// DecodeBytes decodes a PublicKeys from the given slice of bytes.
func (keys *PublicKeys) DecodeBytes(data []byte) error {
	b := io.NewBinReaderFromBuf(data)
	b.ReadArray(keys)
	return b.Err
}

// Bytes encodes PublicKeys to the new slice of bytes.
func (keys *PublicKeys) Bytes() []byte {
	buf := io.NewBufBinWriter()
	buf.WriteArray(*keys)
	if buf.Err != nil {
		panic(buf.Err)
	}
	return buf.Bytes()
}

// Contains checks whether passed param contained in PublicKeys.
func (keys PublicKeys) Contains(pKey *PublicKey) bool {
	for _, key := range keys {
		if key.Equal(pKey) {
			return true
		}
	}
	return false
}

// Copy returns copy of keys.
func (keys PublicKeys) Copy() PublicKeys {
	res := make(PublicKeys, len(keys))
	copy(res, keys)
	return res
}

// Unique returns set of public keys.
func (keys PublicKeys) Unique() PublicKeys {
	unique := PublicKeys{}
	for _, publicKey := range keys {
		if !unique.Contains(publicKey) {
			unique = append(unique, publicKey)
		}
	}
	return unique
}

// PublicKey represents a public key and provides a high level
// API around ecdsa.PublicKey.
type PublicKey ecdsa.PublicKey

// Equal returns true in case public keys are equal.
func (p *PublicKey) Equal(key *PublicKey) bool {
	return p.X.Cmp(key.X) == 0 && p.Y.Cmp(key.Y) == 0
}

// Cmp compares two keys.
func (p *PublicKey) Cmp(key *PublicKey) int {
	xCmp := p.X.Cmp(key.X)
	if xCmp != 0 {
		return xCmp
	}
	return p.Y.Cmp(key.Y)
}

// NewPublicKeyFromString returns a public key created from the
// given hex string.
func NewPublicKeyFromString(s string) (*PublicKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return NewPublicKeyFromBytes(b, elliptic.P256())
}

// keycache is a simple lru cache for P256 keys that avoids Y calculation overhead
// for known keys.
var keycache *lru.Cache

func init() {
	// Less than 100K, probably enough for our purposes.
	keycache, _ = lru.New(1024)
}

// NewPublicKeyFromBytes returns public key created from b using given EC.
func NewPublicKeyFromBytes(b []byte, curve elliptic.Curve) (*PublicKey, error) {
	var pubKey *PublicKey
	cachedKey, ok := keycache.Get(string(b))
	if ok {
		pubKey = cachedKey.(*PublicKey)
		if pubKey.Curve == curve {
			return pubKey, nil
		}
	}
	pubKey = new(PublicKey)
	pubKey.Curve = curve
	if err := pubKey.DecodeBytes(b); err != nil {
		return nil, err
	}
	keycache.Add(string(b), pubKey)
	return pubKey, nil
}

// getBytes serializes X and Y using compressed or uncompressed format.
func (p *PublicKey) getBytes(compressed bool) []byte {
	if p.IsInfinity() {
		return []byte{0x00}
	}

	var resLen = 1 + coordLen
	if !compressed {
		resLen += coordLen
	}
	var res = make([]byte, resLen)
	var prefix byte

	xBytes := p.X.Bytes()
	copy(res[1+coordLen-len(xBytes):], xBytes)
	if compressed {
		if p.Y.Bit(0) == 0 {
			prefix = 0x02
		} else {
			prefix = 0x03
		}
	} else {
		prefix = 0x04
		yBytes := p.Y.Bytes()
		copy(res[1+coordLen+coordLen-len(yBytes):], yBytes)

	}
	res[0] = prefix

	return res
}

// Bytes returns byte array representation of the public key in compressed
// form (33 bytes with 0x02 or 0x03 prefix, except infinity which is always 0).
func (p *PublicKey) Bytes() []byte {
	return p.getBytes(true)
}

// UncompressedBytes returns byte array representation of the public key in
// uncompressed form (65 bytes with 0x04 prefix, except infinity which is
// always 0).
func (p *PublicKey) UncompressedBytes() []byte {
	return p.getBytes(false)
}

// NewPublicKeyFromASN1 returns a NEO PublicKey from the ASN.1 serialized key.
func NewPublicKeyFromASN1(data []byte) (*PublicKey, error) {
	var (
		err    error
		pubkey interface{}
	)
	if pubkey, err = x509.ParsePKIXPublicKey(data); err != nil {
		return nil, err
	}
	pk, ok := pubkey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("given bytes aren't ECDSA public key")
	}
	result := PublicKey(*pk)
	return &result, nil
}

// decodeCompressedY performs decompression of Y coordinate for given X and Y's least significant bit.
// We use here a short-form Weierstrass curve (https://www.hyperelliptic.org/EFD/g1p/auto-shortw.html)
// y² = x³ + ax + b. Two types of elliptic curves are supported:
// 1. Secp256k1 (Koblitz curve): y² = x³ + b,
// 2. Secp256r1 (Random curve): y² = x³ - 3x + b.
// To decode compressed curve point we perform the following operation: y = sqrt(x³ + ax + b mod p)
// where `p` denotes the order of the underlying curve field
func decodeCompressedY(x *big.Int, ylsb uint, curve elliptic.Curve) (*big.Int, error) {
	var a *big.Int
	switch curve.(type) {
	case *btcec.KoblitzCurve:
		a = big.NewInt(0)
	default:
		a = big.NewInt(3)
	}
	cp := curve.Params()
	xCubed := new(big.Int).Exp(x, big.NewInt(3), cp.P)
	aX := new(big.Int).Mul(x, a)
	aX.Mod(aX, cp.P)
	ySquared := new(big.Int).Sub(xCubed, aX)
	ySquared.Add(ySquared, cp.B)
	ySquared.Mod(ySquared, cp.P)
	y := new(big.Int).ModSqrt(ySquared, cp.P)
	if y == nil {
		return nil, errors.New("error computing Y for compressed point")
	}
	if y.Bit(0) != ylsb {
		y.Neg(y)
		y.Mod(y, cp.P)
	}
	return y, nil
}

// DecodeBytes decodes a PublicKey from the given slice of bytes.
func (p *PublicKey) DecodeBytes(data []byte) error {
	b := io.NewBinReaderFromBuf(data)
	p.DecodeBinary(b)
	return b.Err
}

// DecodeBinary decodes a PublicKey from the given BinReader using information
// about the EC curve to decompress Y point. Secp256r1 is a default value for EC curve.
func (p *PublicKey) DecodeBinary(r *io.BinReader) {
	var prefix uint8
	var x, y *big.Int
	var err error

	prefix = uint8(r.ReadB())
	if r.Err != nil {
		return
	}

	if p.Curve == nil {
		p.Curve = elliptic.P256()
	}
	curve := p.Curve
	curveParams := p.Params()
	// Infinity
	switch prefix {
	case 0x00:
		// noop, initialized to nil
		return
	case 0x02, 0x03:
		// Compressed public keys
		xbytes := make([]byte, 32)
		r.ReadBytes(xbytes)
		if r.Err != nil {
			return
		}
		x = new(big.Int).SetBytes(xbytes)
		ylsb := uint(prefix & 0x1)
		y, err = decodeCompressedY(x, ylsb, curve)
		if err != nil {
			r.Err = err
			return
		}
	case 0x04:
		xbytes := make([]byte, 32)
		ybytes := make([]byte, 32)
		r.ReadBytes(xbytes)
		r.ReadBytes(ybytes)
		if r.Err != nil {
			return
		}
		x = new(big.Int).SetBytes(xbytes)
		y = new(big.Int).SetBytes(ybytes)
		if !curve.IsOnCurve(x, y) {
			r.Err = errors.New("encoded point is not on the P256 curve")
			return
		}
	default:
		r.Err = fmt.Errorf("invalid prefix %d", prefix)
		return
	}
	if x.Cmp(curveParams.P) >= 0 || y.Cmp(curveParams.P) >= 0 {
		r.Err = errors.New("enccoded point is not correct (X or Y is bigger than P")
		return
	}
	p.X, p.Y = x, y
}

// EncodeBinary encodes a PublicKey to the given BinWriter.
func (p *PublicKey) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(p.Bytes())
}

// GetVerificationScript returns NEO VM bytecode with CHECKSIG command for the
// public key.
func (p *PublicKey) GetVerificationScript() []byte {
	b := p.Bytes()
	buf := io.NewBufBinWriter()
	if address.Prefix == address.NEO2Prefix {
		buf.WriteB(0x21) // PUSHBYTES33
		buf.WriteBytes(p.Bytes())
		buf.WriteB(0xAC) // CHECKSIG
		return buf.Bytes()
	}
	emit.Bytes(buf.BinWriter, b)
	emit.Syscall(buf.BinWriter, interopnames.NeoCryptoCheckSig)

	return buf.Bytes()
}

// GetScriptHash returns a Hash160 of verification script for the key.
func (p *PublicKey) GetScriptHash() util.Uint160 {
	return hash.Hash160(p.GetVerificationScript())
}

// Address returns a base58-encoded NEO-specific address based on the key hash.
func (p *PublicKey) Address() string {
	return address.Uint160ToString(p.GetScriptHash())
}

// Verify returns true if the signature is valid and corresponds
// to the hash and public key.
func (p *PublicKey) Verify(signature []byte, hash []byte) bool {
	if p.X == nil || p.Y == nil || len(signature) != SignatureLen {
		return false
	}
	rBytes := new(big.Int).SetBytes(signature[0:32])
	sBytes := new(big.Int).SetBytes(signature[32:64])
	pk := ecdsa.PublicKey(*p)
	return ecdsa.Verify(&pk, hash, rBytes, sBytes)
}

// VerifyHashable returns true if the signature is valid and corresponds
// to the hash and public key.
func (p *PublicKey) VerifyHashable(signature []byte, net uint32, hh hash.Hashable) bool {
	var digest = hash.NetSha256(net, hh)
	return p.Verify(signature, digest[:])
}

// IsInfinity checks if the key is infinite (null, basically).
func (p *PublicKey) IsInfinity() bool {
	return p.X == nil && p.Y == nil
}

// String implements the Stringer interface.
func (p *PublicKey) String() string {
	if p.IsInfinity() {
		return "00"
	}
	bx := hex.EncodeToString(p.X.Bytes())
	by := hex.EncodeToString(p.Y.Bytes())
	return fmt.Sprintf("%s%s", bx, by)
}

// MarshalJSON implements the json.Marshaler interface.
func (p PublicKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(p.Bytes()))
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (p *PublicKey) UnmarshalJSON(data []byte) error {
	l := len(data)
	if l < 2 || data[0] != '"' || data[l-1] != '"' {
		return errors.New("wrong format")
	}

	bytes := make([]byte, l-2)
	_, err := hex.Decode(bytes, data[1:l-1])
	if err != nil {
		return err
	}
	err = p.DecodeBytes(bytes)
	if err != nil {
		return err
	}

	return nil
}
