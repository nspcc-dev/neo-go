package keys

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/base58"
	"golang.org/x/crypto/scrypt"
	"golang.org/x/text/unicode/norm"
)

// NEP-2 standard implementation for encrypting and decrypting private keys.

// NEP-2 specified parameters used for cryptography.
const (
	n       = 16384
	r       = 8
	p       = 8
	keyLen  = 64
	nepFlag = 0xe0
)

var nepHeader = []byte{0x01, 0x42}

// ScryptParams is a json-serializable container for scrypt KDF parameters.
type ScryptParams struct {
	N int `json:"n"`
	R int `json:"r"`
	P int `json:"p"`
}

// NEP2ScryptParams returns scrypt parameters specified in the NEP-2.
func NEP2ScryptParams() ScryptParams {
	return ScryptParams{
		N: n,
		R: r,
		P: p,
	}
}

// NEP2Encrypt encrypts a the PrivateKey using the given passphrase
// under the NEP-2 standard.
func NEP2Encrypt(priv *PrivateKey, passphrase string, params ScryptParams) (s string, err error) {
	address := priv.Address()

	addrHash := hash.Checksum([]byte(address))
	// Normalize the passphrase according to the NFC standard.
	phraseNorm := norm.NFC.Bytes([]byte(passphrase))
	derivedKey, err := scrypt.Key(phraseNorm, addrHash, params.N, params.R, params.P, keyLen)
	if err != nil {
		return s, err
	}
	defer clear(derivedKey)

	derivedKey1 := derivedKey[:32]
	derivedKey2 := derivedKey[32:]

	privBytes := priv.Bytes()
	defer clear(privBytes)
	xr := xor(privBytes, derivedKey1)
	defer clear(xr)

	encrypted, err := aesEncrypt(xr, derivedKey2)
	if err != nil {
		return s, err
	}

	buf := new(bytes.Buffer)
	buf.Write(nepHeader)
	buf.WriteByte(nepFlag)
	buf.Write(addrHash)
	buf.Write(encrypted)

	if buf.Len() != 39 {
		return s, fmt.Errorf("invalid buffer length: expecting 39 bytes got %d", buf.Len())
	}

	return base58.CheckEncode(buf.Bytes()), nil
}

// NEP2Decrypt decrypts an encrypted key using the given passphrase
// under the NEP-2 standard.
func NEP2Decrypt(key, passphrase string, params ScryptParams) (*PrivateKey, error) {
	b, err := base58.CheckDecode(key)
	if err != nil {
		return nil, err
	}
	if err := validateNEP2Format(b); err != nil {
		return nil, err
	}

	addrHash := b[3:7]
	// Normalize the passphrase according to the NFC standard.
	phraseNorm := norm.NFC.Bytes([]byte(passphrase))
	derivedKey, err := scrypt.Key(phraseNorm, addrHash, params.N, params.R, params.P, keyLen)
	if err != nil {
		return nil, err
	}
	defer clear(derivedKey)

	derivedKey1 := derivedKey[:32]
	derivedKey2 := derivedKey[32:]
	encryptedBytes := b[7:]

	decrypted, err := aesDecrypt(encryptedBytes, derivedKey2)
	if err != nil {
		return nil, err
	}
	defer clear(decrypted)

	privBytes := xor(decrypted, derivedKey1)
	defer clear(privBytes)

	// Rebuild the private key.
	privKey, err := NewPrivateKeyFromBytes(privBytes)
	if err != nil {
		return nil, err
	}

	if !compareAddressHash(privKey, addrHash) {
		return nil, errors.New("password mismatch")
	}

	return privKey, nil
}

func compareAddressHash(priv *PrivateKey, inhash []byte) bool {
	address := priv.Address()
	addrHash := hash.Checksum([]byte(address))
	return bytes.Equal(addrHash, inhash)
}

func validateNEP2Format(b []byte) error {
	if len(b) != 39 {
		return fmt.Errorf("invalid length: expecting 39 got %d", len(b))
	}
	if b[0] != 0x01 {
		return fmt.Errorf("invalid byte sequence: expecting 0x01 got 0x%02x", b[0])
	}
	if b[1] != 0x42 {
		return fmt.Errorf("invalid byte sequence: expecting 0x42 got 0x%02x", b[1])
	}
	if b[2] != 0xe0 {
		return fmt.Errorf("invalid byte sequence: expecting 0xe0 got 0x%02x", b[2])
	}
	return nil
}

func xor(a, b []byte) []byte {
	if len(a) != len(b) {
		panic("cannot XOR non equal length arrays")
	}
	dst := make([]byte, len(a))
	for i := 0; i < len(dst); i++ {
		dst[i] = a[i] ^ b[i]
	}
	return dst
}
