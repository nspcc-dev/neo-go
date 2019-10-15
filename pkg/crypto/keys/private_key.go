package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/rfc6979"
)

// PrivateKey represents a NEO private key.
type PrivateKey struct {
	b []byte
}

// NewPrivateKey creates a new random private key.
func NewPrivateKey() (*PrivateKey, error) {
	priv, _, _, err := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{b: priv}, nil
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

// NewPrivateKeyFromASN1 returns a NEO PrivateKey from the ASN.1 serialized key.
func NewPrivateKeyFromASN1(b []byte) (*PrivateKey, error) {
	privkey, err := x509.ParseECPrivateKey(b)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyFromBytes(privkey.D.Bytes())
}

// PublicKey derives the public key from the private key.
func (p *PrivateKey) PublicKey() *PublicKey {
	var (
		c = elliptic.P256()
		q = new(big.Int).SetBytes(p.b)
	)

	x, y := c.ScalarBaseMult(q.Bytes())

	return &PublicKey{X: x, Y: y}
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
func (p *PrivateKey) WIF() string {
	w, err := WIFEncode(p.b, WIFVersion, true)
	// The only way WIFEncode() can fail is if we're to give it a key of
	// wrong size, but we have a proper key here, aren't we?
	if err != nil {
		panic(err)
	}
	return w
}

// Address derives the public NEO address that is coupled with the private key, and
// returns it as a string.
func (p *PrivateKey) Address() string {
	pk := p.PublicKey()
	return pk.Address()
}

// Signature creates the signature using the private key.
func (p *PrivateKey) Signature() []byte {
	pk := p.PublicKey()
	return pk.Signature()
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
