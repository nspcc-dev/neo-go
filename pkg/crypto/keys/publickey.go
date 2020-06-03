package keys

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/pkg/errors"
)

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

// Contains checks whether passed param contained in PublicKeys.
func (keys PublicKeys) Contains(pKey *PublicKey) bool {
	for _, key := range keys {
		if key.Equal(pKey) {
			return true
		}
	}
	return false
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
// API around the X/Y point.
type PublicKey struct {
	X *big.Int
	Y *big.Int
}

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

	pubKey := new(PublicKey)
	r := io.NewBinReaderFromBuf(b)
	pubKey.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}

	return pubKey, nil
}

// Bytes returns the byte array representation of the public key.
func (p *PublicKey) Bytes() []byte {
	if p.IsInfinity() {
		return []byte{0x00}
	}

	var (
		x       = p.X.Bytes()
		paddedX = append(bytes.Repeat([]byte{0x00}, 32-len(x)), x...)
		prefix  = byte(0x03)
	)

	if p.Y.Bit(0) == 0 {
		prefix = byte(0x02)
	}

	return append([]byte{prefix}, paddedX...)
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
	key := PublicKey{
		X: pk.X,
		Y: pk.Y,
	}
	return &key, nil
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
	l := len(data)
	if !((l == 1 && data[0] == 0) ||
		(l == 33 && (data[0] == 0x02 || data[0] == 0x03)) ||
		(l == 65 && data[0] == 0x04)) {
		return errors.New("invalid key size/prefix")
	}
	b := io.NewBinReaderFromBuf(data)
	p.DecodeBinary(b)
	return b.Err
}

// DecodeBinary decodes a PublicKey from the given BinReader.
func (p *PublicKey) DecodeBinary(r *io.BinReader) {
	var prefix uint8
	var x, y *big.Int
	var err error

	prefix = uint8(r.ReadB())
	if r.Err != nil {
		return
	}

	p256 := elliptic.P256()
	p256Params := p256.Params()
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
		y, err = decodeCompressedY(x, ylsb, p256)
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
		if !p256.IsOnCurve(x, y) {
			r.Err = errors.New("encoded point is not on the P256 curve")
			return
		}
	default:
		r.Err = errors.Errorf("invalid prefix %d", prefix)
		return
	}
	if x.Cmp(p256Params.P) >= 0 || y.Cmp(p256Params.P) >= 0 {
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
	b = append([]byte{byte(opcode.PUSHBYTES33)}, b...)
	b = append(b, byte(opcode.CHECKSIG))

	return b
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

	publicKey := &ecdsa.PublicKey{}
	publicKey.Curve = elliptic.P256()
	publicKey.X = p.X
	publicKey.Y = p.Y
	if p.X == nil || p.Y == nil {
		return false
	}
	rBytes := new(big.Int).SetBytes(signature[0:32])
	sBytes := new(big.Int).SetBytes(signature[32:64])
	return ecdsa.Verify(publicKey, hash, rBytes, sBytes)
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

	bytes := make([]byte, hex.DecodedLen(l-2))
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

// KeyRecover recovers public key from the given signature (r, s) on the given message hash using given elliptic curve.
// Algorithm source: SEC 1 Ver 2.0, section 4.1.6, pages 47-48 (https://www.secg.org/sec1-v2.pdf).
// Flag isEven denotes Y's least significant bit in decompression algorithm.
func KeyRecover(curve elliptic.Curve, r, s *big.Int, messageHash []byte, isEven bool) (PublicKey, error) {
	var (
		res PublicKey
		err error
	)
	if r.Cmp(big.NewInt(1)) == -1 || s.Cmp(big.NewInt(1)) == -1 {
		return res, errors.New("invalid signature")
	}
	params := curve.Params()
	// Calculate h = (Q + 1 + 2 * Sqrt(Q)) / N
	// num := new(big.Int).Add(new(big.Int).Add(params.P, big.NewInt(1)), new(big.Int).Mul(big.NewInt(2), new(big.Int).Sqrt(params.P)))
	// h := new(big.Int).Div(num, params.N)
	// We are skipping this step for secp256k1 and secp256r1 because we know cofactor of these curves (h=1)
	// (see section 2.4 of http://www.secg.org/sec2-v2.pdf)
	h := 1
	for i := 0; i <= h; i++ {
		// Step 1.1: x = (n * i) + r
		Rx := new(big.Int).Mul(params.N, big.NewInt(int64(i)))
		Rx.Add(Rx, r)
		if Rx.Cmp(params.P) == 1 {
			break
		}

		// Steps 1.2 and 1.3: get point R (Ry)
		var R *big.Int
		if isEven {
			R, err = decodeCompressedY(Rx, 0, curve)
		} else {
			R, err = decodeCompressedY(Rx, 1, curve)
		}
		if err != nil {
			return res, err
		}

		// Step 1.4: check n*R is point at infinity
		nRx, nR := curve.ScalarMult(Rx, R, params.N.Bytes())
		if nRx.Sign() != 0 || nR.Sign() != 0 {
			continue
		}

		// Step 1.5: compute e
		e := hashToInt(messageHash, curve)

		// Step 1.6: Q = r^-1 (sR-eG)
		invr := new(big.Int).ModInverse(r, params.N)
		// First term.
		invrS := new(big.Int).Mul(invr, s)
		invrS.Mod(invrS, params.N)
		sRx, sR := curve.ScalarMult(Rx, R, invrS.Bytes())
		// Second term.
		e.Neg(e)
		e.Mod(e, params.N)
		e.Mul(e, invr)
		e.Mod(e, params.N)
		minuseGx, minuseGy := curve.ScalarBaseMult(e.Bytes())
		Qx, Qy := curve.Add(sRx, sR, minuseGx, minuseGy)
		res.X = Qx
		res.Y = Qy
	}
	return res, nil
}

// copied from crypto/ecdsa
func hashToInt(hash []byte, c elliptic.Curve) *big.Int {
	orderBits := c.Params().N.BitLen()
	orderBytes := (orderBits + 7) / 8
	if len(hash) > orderBytes {
		hash = hash[:orderBytes]
	}

	ret := new(big.Int).SetBytes(hash)
	excess := len(hash)*8 - orderBits
	if excess > 0 {
		ret.Rsh(ret, uint(excess))
	}
	return ret
}
