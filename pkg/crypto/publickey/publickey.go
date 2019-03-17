package publickey

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/crypto/base58"
	"github.com/CityOfZion/neo-go/pkg/crypto/elliptic"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
)

// PublicKeys is a list of public keys.
type PublicKeys []*PublicKey

func (keys PublicKeys) Len() int      { return len(keys) }
func (keys PublicKeys) Swap(i, j int) { keys[i], keys[j] = keys[j], keys[i] }
func (keys PublicKeys) Less(i, j int) bool {

	if keys[i].X.Cmp(keys[j].X) == -1 {
		return true
	}
	if keys[i].X.Cmp(keys[j].X) == 1 {
		return false
	}
	if keys[i].X.Cmp(keys[j].X) == 0 {
		return false
	}

	return keys[i].Y.Cmp(keys[j].Y) == -1
}

// PublicKey represents a public key and provides a high level
// API around the ECPoint.
type PublicKey struct {
	Curve elliptic.Curve
	elliptic.Point
}

// NewPublicKeyFromString return a public key created from the
// given hex string.
func NewPublicKeyFromString(s string) (*PublicKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	curve := elliptic.NewEllipticCurve(elliptic.Secp256r1)

	pubKey := &PublicKey{curve, elliptic.Point{}}

	if err := pubKey.DecodeBinary(bytes.NewReader(b)); err != nil {
		return nil, err
	}

	return pubKey, nil
}

// Bytes returns the byte array representation of the public key.
func (p *PublicKey) Bytes() []byte {
	if p.Curve.IsInfinity(p.Point) {
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

// ToAddress will convert a public key to it's neo-address
func (p *PublicKey) ToAddress() string {

	publicKeyBytes := p.Bytes()

	publicKeyBytes = append([]byte{0x21}, publicKeyBytes...) // 0x21 = length of pubKey
	publicKeyBytes = append(publicKeyBytes, 0xAC)            // 0xAC = CheckSig

	hash160PubKey, _ := hash.Hash160(publicKeyBytes)

	versionHash160PubKey := append([]byte{0x17}, hash160PubKey.Bytes()...)

	checksum, _ := hash.Checksum(versionHash160PubKey)

	checkVersionHash160 := append(versionHash160PubKey, checksum...)

	address := base58.Encode(checkVersionHash160)

	return address
}

// DecodeBinary decodes a PublicKey from the given io.Reader.
func (p *PublicKey) DecodeBinary(r io.Reader) error {

	var prefix uint8
	if err := binary.Read(r, binary.LittleEndian, &prefix); err != nil {
		return err
	}

	// Infinity
	if prefix == 0x00 {
		p.Point = elliptic.Point{}
		return nil
	}

	// Compressed public keys.
	if prefix == 0x02 || prefix == 0x03 {

		b := make([]byte, 32)
		if err := binary.Read(r, binary.LittleEndian, b); err != nil {
			return err
		}

		var err error

		p.Point, err = p.Curve.Decompress(new(big.Int).SetBytes(b), uint(prefix&0x1))
		if err != nil {
			return err
		}

	} else if prefix == 0x04 {
		buf := make([]byte, 65)
		if err := binary.Read(r, binary.LittleEndian, buf); err != nil {
			return err
		}
		p.X = new(big.Int).SetBytes(buf[1:33])
		p.Y = new(big.Int).SetBytes(buf[33:65])
	} else {
		return fmt.Errorf("invalid prefix %d", prefix)
	}

	return nil
}

// EncodeBinary encodes a PublicKey to the given io.Writer.
func (p *PublicKey) EncodeBinary(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, p.Bytes())
}

// Verify returns true if the signature is valid and corresponds
// to the hash and public key
func (p *PublicKey) Verify(signature []byte, hash []byte) bool {

	publicKey := &ecdsa.PublicKey{}
	publicKey.Curve = p.Curve
	publicKey.X = p.X
	publicKey.Y = p.Y
	if p.X == nil || p.Y == nil {
		return false
	}
	rBytes := new(big.Int).SetBytes(signature[0:32])
	sBytes := new(big.Int).SetBytes(signature[32:64])
	return ecdsa.Verify(publicKey, hash, rBytes, sBytes)
}
