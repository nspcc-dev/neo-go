package wallet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/anthdm/rfc6979"
	"golang.org/x/crypto/ripemd160"
)

// PrivateKey represents a NEO private key.
type PrivateKey struct {
	b []byte
}

func NewPrivateKey() (*PrivateKey, error) {
	c := crypto.NewEllipticCurve()
	b := make([]byte, c.N.BitLen()/8+8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}

	d := new(big.Int).SetBytes(b)
	d.Mod(d, new(big.Int).Sub(c.N, big.NewInt(1)))
	d.Add(d, big.NewInt(1))

	p := &PrivateKey{b: d.Bytes()}
	return p, nil
}

// NewPrivateKeyFromHex returns a PrivateKey created from the
// given hex string.
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

// PublicKey derives the public key from the private key.
func (p *PrivateKey) PublicKey() ([]byte, error) {
	var (
		c = crypto.NewEllipticCurve()
		q = new(big.Int).SetBytes(p.b)
	)

	point := c.ScalarBaseMult(q)
	if !c.IsOnCurve(point) {
		return nil, errors.New("failed to derive public key using elliptic curve")
	}

	bx := point.X.Bytes()
	padded := append(
		bytes.Repeat(
			[]byte{0x00},
			32-len(bx),
		),
		bx...,
	)

	prefix := []byte{0x03}
	if point.Y.Bit(0) == 0 {
		prefix = []byte{0x02}
	}
	b := append(prefix, padded...)

	return b, nil
}

// NewPrivateKeyFromWIF returns a NEO PrivateKey from the given
// WIF (wallet import format).
func NewPrivateKeyFromWIF(wif string) (*PrivateKey, error) {
	w, err := WIFDecode(wif, WIFVersion)
	if err != nil {
		return nil, err
	}
	return w.PrivateKey, nil
}

// WIF returns the (wallet import format) of the PrivateKey.
// Good documentation about this process can be found here:
// https://en.bitcoin.it/wiki/Wallet_import_format
func (p *PrivateKey) WIF() (string, error) {
	return WIFEncode(p.b, WIFVersion, true)
}

// Address derives the public NEO address that is coupled with the private key, and
// returns it as a string.
func (p *PrivateKey) Address() (string, error) {
	b, err := p.Signature()
	if err != nil {
		return "", err
	}

	b = append([]byte{0x17}, b...)

	sha := sha256.New()
	sha.Write(b)
	hash := sha.Sum(nil)

	sha.Reset()
	sha.Write(hash)
	hash = sha.Sum(nil)

	b = append(b, hash[0:4]...)

	address := crypto.Base58Encode(b)

	return address, nil
}

// Signature creates the signature using the private key.
func (p *PrivateKey) Signature() ([]byte, error) {
	b, err := p.PublicKey()
	if err != nil {
		return nil, err
	}

	b = append([]byte{0x21}, b...)
	b = append(b, 0xAC)

	sha := sha256.New()
	sha.Write(b)
	hash := sha.Sum(nil)

	ripemd := ripemd160.New()
	ripemd.Reset()
	ripemd.Write(hash)
	hash = ripemd.Sum(nil)

	return hash, nil
}

// Sign signs arbitrary length data using the private key.
func (p *PrivateKey) Sign(data []byte) ([]byte, error) {
	var (
		privateKey = p.ecdsa()
		digest     = sha256.Sum256(data)
	)

	r, s, err := rfc6979.SignECDSA(privateKey, digest[:], sha256.New)
	if err != nil {
		return nil, err
	}

	params := privateKey.Curve.Params()
	curveOrderByteSize := params.P.BitLen() / 8
	rBytes, sBytes := r.Bytes(), s.Bytes()
	signature := make([]byte, curveOrderByteSize*2)
	copy(signature[curveOrderByteSize-len(rBytes):], rBytes)
	copy(signature[curveOrderByteSize*2-len(sBytes):], sBytes)

	return signature, nil
}

// ecsda converts the key to a usable ecsda.PrivateKey for signing data.
func (p *PrivateKey) ecdsa() *ecdsa.PrivateKey {
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = elliptic.P256()
	priv.D = new(big.Int).SetBytes(p.b)
	priv.PublicKey.X, priv.PublicKey.Y = priv.PublicKey.Curve.ScalarBaseMult(p.b)
	return priv
}

// String implements the stringer interface.
func (p *PrivateKey) String() string {
	return hex.EncodeToString(p.b)
}

// Bytes returns the underlying bytes of the PrivateKey.
func (p *PrivateKey) Bytes() []byte {
	return p.b
}
