package privatekey

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/v2/crypto/publickey"

	"github.com/CityOfZion/neo-go/pkg/v2/crypto/base58"
	"github.com/CityOfZion/neo-go/pkg/v2/crypto/elliptic"
	"github.com/CityOfZion/neo-go/pkg/v2/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/v2/crypto/rfc6979"
)

// PrivateKey represents a NEO private key.
type PrivateKey struct {
	b []byte
}

func NewPrivateKey() (*PrivateKey, error) {
	curve := elliptic.NewEllipticCurve(elliptic.Secp256r1)
	b := make([]byte, curve.N.BitLen()/8+8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}

	d := new(big.Int).SetBytes(b)
	d.Mod(d, new(big.Int).Sub(curve.N, big.NewInt(1)))
	d.Add(d, big.NewInt(1))

	p := &PrivateKey{b: d.Bytes()}
	return p, nil
}

func NewPrivateKeyFromHex(str string) (*PrivateKey, error) {
	b, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyFromBytes(b)
}

// NewPrivateKeyFromBytes returns a NEO PrivateKey from the given byte slice.
func NewPrivateKeyFromBytes(b []byte) (*PrivateKey, error) {
	if len(b) != 32 {
		return nil, fmt.Errorf(
			"invalid byte length: expected %d bytes got %d", 32, len(b),
		)
	}
	return &PrivateKey{b}, nil
}

func (p *PrivateKey) PublicKey() (*publickey.PublicKey, error) {
	var (
		c = elliptic.NewEllipticCurve(elliptic.Secp256r1)
		q = new(big.Int).SetBytes(p.b)
	)

	p1, p2 := c.ScalarBaseMult(q.Bytes())
	point := elliptic.Point{p1, p2}
	if !c.IsOnCurve(p1, p2) {
		return nil, errors.New("failed to derive public key using elliptic curve")
	}

	return &publickey.PublicKey{point}, nil

}

func WIFEncode(key []byte) (s string) {
	if len(key) != 32 {
		return "invalid private key length"
	}

	buf := new(bytes.Buffer)
	buf.WriteByte(0x80)
	buf.Write(key)

	buf.WriteByte(0x01)

	checksum, _ := hash.Checksum(buf.Bytes())

	buf.Write(checksum)

	WIF := base58.Encode(buf.Bytes())
	return WIF
}

func (p *PrivateKey) Sign(data []byte) ([]byte, error) {
	curve := elliptic.NewEllipticCurve(elliptic.Secp256r1)
	key := p.b
	digest, _ := hash.Sha256(data)

	r, s, err := rfc6979.SignECDSA(curve, key, digest[:], sha256.New)
	if err != nil {
		return nil, err
	}

	curveOrderByteSize := curve.P.BitLen() / 8
	rBytes, sBytes := r.Bytes(), s.Bytes()
	signature := make([]byte, curveOrderByteSize*2)
	copy(signature[curveOrderByteSize-len(rBytes):], rBytes)
	copy(signature[curveOrderByteSize*2-len(sBytes):], sBytes)

	return signature, nil
}
