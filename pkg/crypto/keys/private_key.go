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

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/rfc6979"
)

// PrivateKey represents a Neo private key and provides a high level API around
// ecdsa.PrivateKey.
type PrivateKey struct {
	ecdsa.PrivateKey
}

// NewPrivateKey creates a new random Secp256r1 private key.
func NewPrivateKey() (*PrivateKey, error) {
	return newPrivateKeyOnCurve(elliptic.P256())
}

// NewSecp256k1PrivateKey creates a new random Secp256k1 private key.
func NewSecp256k1PrivateKey() (*PrivateKey, error) {
	return newPrivateKeyOnCurve(secp256k1.S256())
}

// newPrivateKeyOnCurve creates a new random private key using curve c.
func newPrivateKeyOnCurve(c elliptic.Curve) (*PrivateKey, error) {
	pk, err := ecdsa.GenerateKey(c, rand.Reader)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{*pk}, nil
}

// NewPrivateKeyFromHex returns a Secp256k1 PrivateKey created from the
// given hex string.
func NewPrivateKeyFromHex(str string) (*PrivateKey, error) {
	b, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}
	defer clear(b)
	return NewPrivateKeyFromBytes(b)
}

// NewPrivateKeyFromBytes returns a NEO Secp256r1 PrivateKey from the given
// byte slice.
func NewPrivateKeyFromBytes(b []byte) (*PrivateKey, error) {
	if len(b) != 32 {
		return nil, fmt.Errorf(
			"invalid byte length: expected %d bytes got %d", 32, len(b),
		)
	}
	var (
		c = elliptic.P256()
		d = new(big.Int).SetBytes(b)
	)

	x, y := c.ScalarBaseMult(b)

	return &PrivateKey{
		ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: c,
				X:     x,
				Y:     y,
			},
			D: d,
		},
	}, nil
}

// NewPrivateKeyFromASN1 returns a NEO Secp256k1 PrivateKey from the ASN.1
// serialized key.
func NewPrivateKeyFromASN1(b []byte) (*PrivateKey, error) {
	privkey, err := x509.ParseECPrivateKey(b)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{*privkey}, nil
}

// PublicKey derives the public key from the private key.
func (p *PrivateKey) PublicKey() *PublicKey {
	result := PublicKey(p.PrivateKey.PublicKey)
	return &result
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
	pb := p.Bytes()
	defer clear(pb)
	w, err := WIFEncode(pb, WIFVersion, true)
	// The only way WIFEncode() can fail is if we're to give it a key of
	// wrong size, but we have a proper key here, aren't we?
	if err != nil {
		panic(err)
	}
	return w
}

// Destroy wipes the contents of the private key from memory. Any operations
// with the key after call to Destroy have undefined behavior.
func (p *PrivateKey) Destroy() {
	clear(p.D.Bits())
}

// Address derives the public NEO address that is coupled with the private key, and
// returns it as a string.
func (p *PrivateKey) Address() string {
	pk := p.PublicKey()
	return pk.Address()
}

// GetScriptHash returns verification script hash for the public key associated with
// the private key.
func (p *PrivateKey) GetScriptHash() util.Uint160 {
	pk := p.PublicKey()
	return pk.GetScriptHash()
}

// Sign signs arbitrary length data using the private key. It uses SHA256 to
// calculate hash and then SignHash to create a signature (so you can save on
// hash calculation if you already have it).
func (p *PrivateKey) Sign(data []byte) []byte {
	var digest = sha256.Sum256(data)

	return p.SignHash(digest)
}

// SignHash signs a particular hash with the private key.
func (p *PrivateKey) SignHash(digest util.Uint256) []byte {
	r, s := rfc6979.SignECDSA(&p.PrivateKey, digest[:], sha256.New)
	return getSignatureSlice(p.Curve, r, s)
}

// SignHashable signs some Hashable item for the network specified using
// hash.NetSha256() with the private key.
func (p *PrivateKey) SignHashable(net uint32, hh hash.Hashable) []byte {
	return p.SignHash(hash.NetSha256(net, hh))
}

func getSignatureSlice(curve elliptic.Curve, r, s *big.Int) []byte {
	params := curve.Params()
	curveOrderByteSize := params.P.BitLen() / 8
	signature := make([]byte, curveOrderByteSize*2)
	_ = r.FillBytes(signature[:curveOrderByteSize])
	_ = s.FillBytes(signature[curveOrderByteSize:])

	return signature
}

// String implements the stringer interface.
func (p *PrivateKey) String() string {
	return hex.EncodeToString(p.Bytes())
}

// Bytes returns the underlying bytes of the PrivateKey.
func (p *PrivateKey) Bytes() []byte {
	result := make([]byte, 32)
	_ = p.D.FillBytes(result)

	return result
}
