package wallet

import (
	"bytes"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/crypto"
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

	// The string representation of the WIF.
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

	s = crypto.Base58CheckEncode(buf.Bytes())
	return
}

// WIFDecode decoded the given WIF string into a WIF struct.
func WIFDecode(wif string, version byte) (*WIF, error) {
	b, err := crypto.Base58CheckDecode(wif)
	if err != nil {
		return nil, err
	}

	if version == 0x00 {
		version = WIFVersion
	}
	if b[0] != version {
		return nil, fmt.Errorf("invalid WIF version got %d, expected %d", b[0], version)
	}

	// Derive the PrivateKey.
	privKey, err := NewPrivateKeyFromBytes(b[1:33])
	if err != nil {
		return nil, err
	}
	w := &WIF{
		Version:    version,
		PrivateKey: privKey,
		S:          wif,
	}

	// This is an uncompressed WIF
	if len(b) == 33 {
		w.Compressed = false
		return w, nil
	}

	if len(b) != 34 {
		return nil, fmt.Errorf("invalid WIF length: %d expecting 34", len(b))
	}

	// Check the compression flag.
	if b[33] != 0x01 {
		return nil, fmt.Errorf("invalid compression flag %d expecting %d", b[34], 0x01)
	}

	w.Compressed = true
	return w, nil
}
