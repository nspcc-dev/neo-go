package crypto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
)

// PublicKey represents a public key.
type PublicKey struct {
	ECPoint
}

// Bytes returns the byte array representation of the public key.
func (p *PublicKey) Bytes() []byte {
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

// DecodeBinary decodes a PublicKey from the given io.Reader.
func (p *PublicKey) DecodeBinary(r io.Reader) error {
	var prefix uint8
	if err := binary.Read(r, binary.LittleEndian, &prefix); err != nil {
		return err
	}

	// Compressed public keys.
	if prefix == 0x02 || prefix == 0x03 {
		c := NewEllipticCurve()
		b := make([]byte, 32)
		if err := binary.Read(r, binary.LittleEndian, b); err != nil {
			return err
		}

		var err error
		p.ECPoint, err = c.Decompress(new(big.Int).SetBytes(b), uint(prefix&0x1))
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
