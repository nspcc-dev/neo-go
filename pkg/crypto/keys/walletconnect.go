package keys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	neogo_io "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// WalletConnectSaltLen is the length of the random salt used in WalletConnect signing.
const WalletConnectSaltLen = 16

// hashWalletConnect computes the SHA-256 hash of the WalletConnect message payload
// for the given message data and random salt. The payload format is:
//
//	0x01 0x00 0x01 0xf0 | VarUint(len(hex(salt)+base64(data))) | hex(salt) | base64(data) | 0x00 0x00
func hashWalletConnect(salt, data []byte) util.Uint256 {
	h := sha256.New()
	saltedLen := hex.EncodedLen(len(salt)) + base64.StdEncoding.EncodedLen(len(data))
	var lenBuf [9]byte
	n := neogo_io.PutVarUint(lenBuf[:], uint64(saltedLen))
	_, _ = h.Write([]byte{0x01, 0x00, 0x01, 0xf0})
	_, _ = h.Write(lenBuf[:n])
	hexEnc := hex.NewEncoder(h)
	_, _ = hexEnc.Write(salt)
	b64Enc := base64.NewEncoder(base64.StdEncoding, h)
	_, _ = b64Enc.Write(data)
	_ = b64Enc.Close()
	_, _ = h.Write([]byte{0x00, 0x00})
	var digest util.Uint256
	h.Sum(digest[:0])
	return digest
}

// SignWalletConnect signs arbitrary data using the WalletConnect scheme. It
// generates a random salt, base64-encodes the data, constructs the payload and
// signs it using RFC 6979 ECDSA with SHA-256. The returned signature is 64 bytes
// of ECDSA signature followed by WalletConnectSaltLen bytes of salt (80 bytes total).
func (p *PrivateKey) SignWalletConnect(data []byte) ([]byte, error) {
	var salt [WalletConnectSaltLen]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return nil, err
	}
	digest := hashWalletConnect(salt[:], data)
	sig := p.SignHash(digest)
	return append(sig, salt[:]...), nil
}

// VerifyWalletConnect verifies the WalletConnect signature of the given data.
// The signature must be 80 bytes: 64 bytes of ECDSA signature followed by
// WalletConnectSaltLen bytes of salt.
func (p *PublicKey) VerifyWalletConnect(data, signature []byte) bool {
	if len(signature) != SignatureLen+WalletConnectSaltLen {
		return false
	}
	sig := signature[:SignatureLen]
	salt := signature[SignatureLen:]
	digest := hashWalletConnect(salt, data)
	return p.Verify(sig, digest[:])
}
