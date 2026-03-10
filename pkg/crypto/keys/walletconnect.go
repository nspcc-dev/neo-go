package keys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// WalletConnectSaltLen is the length of the random salt used in WalletConnect signing.
const WalletConnectSaltLen = 16

// walletConnectPayload constructs the message payload for WalletConnect signing.
// The data parameter MUST be the base64-encoded original message, and salt MUST
// be WalletConnectSaltLen bytes. The resulting payload is:
//
//	0x01 0x00 0x01 0xf0 | VarUint(len(hex(salt)+data)) | hex(salt) | data | 0x00 0x00
func walletConnectPayload(salt, data []byte) []byte {
	saltedLen := hex.EncodedLen(len(salt)) + len(data)
	varLen := make([]byte, 9)
	n := io.PutVarUint(varLen, uint64(saltedLen))
	b := make([]byte, 0, 4+n+saltedLen+2)
	b = append(b, 0x01, 0x00, 0x01, 0xf0)
	b = append(b, varLen[:n]...)
	hexSalt := make([]byte, hex.EncodedLen(len(salt)))
	hex.Encode(hexSalt, salt)
	b = append(b, hexSalt...)
	b = append(b, data...)
	b = append(b, 0x00, 0x00)
	return b
}

// SignWalletConnect signs arbitrary data using the WalletConnect scheme. It base64-encodes
// the data, generates a random salt, constructs the payload and signs it using
// RFC 6979 ECDSA with SHA-256. The returned signature is 64 bytes of ECDSA
// signature followed by WalletConnectSaltLen bytes of salt (80 bytes total).
func (p *PrivateKey) SignWalletConnect(data []byte) ([]byte, error) {
	b64 := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(b64, data)

	var salt [WalletConnectSaltLen]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return nil, err
	}

	payload := walletConnectPayload(salt[:], b64)
	h := sha256.Sum256(payload)
	sig := p.SignHash(h)
	return append(sig, salt[:]...), nil
}

// VerifyWalletConnect verifies the WalletConnect signature of the given data.
// The signature must be 80 bytes: 64 bytes of ECDSA signature followed by
// WalletConnectSaltLen bytes of salt.
func (p *PublicKey) VerifyWalletConnect(data, signature []byte) bool {
	if len(signature) != SignatureLen+WalletConnectSaltLen {
		return false
	}
	b64 := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(b64, data)

	sig := signature[:SignatureLen]
	salt := signature[SignatureLen:]
	payload := walletConnectPayload(salt, b64)
	h := sha256.Sum256(payload)
	return p.Verify(sig, h[:])
}
