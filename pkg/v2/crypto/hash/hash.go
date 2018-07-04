package hash

import (
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/ripemd160"
)

func Sha256(data []byte) ([]byte, error) {
	hasher := sha256.New()
	hasher.Reset()
	_, err := hasher.Write(data)
	if err != nil {
		return nil, err
	}
	return hasher.Sum(nil), nil
}

func RipeMD160(data []byte) ([]byte, error) {
	hasher := ripemd160.New()
	hasher.Reset()
	_, err := io.WriteString(hasher, string(data))
	if err != nil {
		return nil, err
	}
	return hasher.Sum(nil), nil
}

func Hash160(data []byte) ([]byte, error) {
	hash1, err := Sha256(data)
	if err != nil {
		return nil, err
	}

	hash2, err := RipeMD160(hash1)
	if err != nil {
		return nil, err
	}

	return hash2, nil
}

func DoubleSha256(data []byte) ([]byte, error) {
	hash1, err := Sha256(data)
	if err != nil {
		return nil, err
	}

	hash2, err := Sha256(hash1)
	if err != nil {
		return nil, err
	}
	return hash2, nil
}

func Checksum(data []byte) ([]byte, error) {
	hash, err := DoubleSha256(data)
	if err != nil {
		return nil, err
	}

	return hash[:4], nil
}
