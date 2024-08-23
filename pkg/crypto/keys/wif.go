package keys

import (
	"bytes"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/encoding/base58"
)

const (
	// WIFVersion is the version used to decode and encode WIF keys.
	WIFVersion = 0x80
)

// WIF represents a wallet import format.
type WIF struct {
	// Version of the wallet import format. Default to 0x80.
	Version byte

	// Bool to determine if the WIF is compressed or not.
	Compressed bool

	// A reference to the PrivateKey which this WIF is created from.
	PrivateKey *PrivateKey

	// A string representation of the WIF.
	S string
}

// WIFEncode encodes the given private key into a WIF string.
func WIFEncode(key []byte, version byte, compressed bool) (s string, err error) {
	if version == 0x00 {
		version = WIFVersion
	}
	if len(key) != 32 {
		return s, fmt.Errorf("invalid private key length: %d", len(key))
	}

	buf := new(bytes.Buffer)
	buf.WriteByte(version)
	buf.Write(key)
	if compressed {
		buf.WriteByte(0x01)
	}

	s = base58.CheckEncode(buf.Bytes())
	return
}

// WIFDecode decodes the given WIF string into a WIF struct.
func WIFDecode(wif string, version byte) (*WIF, error) {
	b, err := base58.CheckDecode(wif)
	if err != nil {
		return nil, err
	}
	defer clear(b)

	if version == 0x00 {
		version = WIFVersion
	}
	w := &WIF{
		Version: version,
		S:       wif,
	}
	switch len(b) {
	case 33: // OK, uncompressed public key.
	case 34: // OK, compressed public key.
		// Check the compression flag.
		if b[33] != 0x01 {
			return nil, fmt.Errorf("invalid compression flag %d expecting %d", b[33], 0x01)
		}
		w.Compressed = true
	default:
		return nil, fmt.Errorf("invalid WIF length %d, expecting 33 or 34", len(b))
	}

	if b[0] != version {
		return nil, fmt.Errorf("invalid WIF version got %d, expected %d", b[0], version)
	}

	// Derive the PrivateKey.
	w.PrivateKey, err = NewPrivateKeyFromBytes(b[1:33])
	if err != nil {
		return nil, err
	}

	return w, nil
}
