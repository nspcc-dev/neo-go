package hash

import (
	"crypto/sha256"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"golang.org/x/crypto/ripemd160"
)

func Sha256(data []byte) (util.Uint256, error) {
	var hash util.Uint256
	hasher := sha256.New()
	hasher.Reset()
	_, err := hasher.Write(data)

	hash, err = util.Uint256DecodeBytes(hasher.Sum(nil))
	if err != nil {
		return hash, err
	}
	return hash, nil
}

func DoubleSha256(data []byte) (util.Uint256, error) {
	var hash util.Uint256

	h1, err := Sha256(data)
	if err != nil {
		return hash, err
	}

	hash, err = Sha256(h1.Bytes())
	if err != nil {
		return hash, err
	}
	return hash, nil
}

func RipeMD160(data []byte) (util.Uint160, error) {
	var hash util.Uint160
	hasher := ripemd160.New()
	hasher.Reset()
	_, err := io.WriteString(hasher, string(data))

	hash, err = util.Uint160DecodeBytes(hasher.Sum(nil))
	if err != nil {
		return hash, err
	}
	return hash, nil
}

func Hash160(data []byte) (util.Uint160, error) {
	var hash util.Uint160
	h1, err := Sha256(data)

	h2, err := RipeMD160(h1.Bytes())

	hash, err = util.Uint160DecodeBytes(h2.Bytes())

	if err != nil {
		return hash, err
	}
	return hash, nil
}

func Checksum(data []byte) ([]byte, error) {
	hash, err := Sum(data)
	if err != nil {
		return nil, err
	}
	return hash[:4], nil
}

func Sum(b []byte) (util.Uint256, error) {
	hash, err := DoubleSha256((b))
	return hash, err
}
