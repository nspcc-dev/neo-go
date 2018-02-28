package crypto

import (
	"crypto/aes"
	"crypto/cipher"
)

// AESEncrypt encrypts the key with the given source.
func AESEncrypt(src, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ecb := newECBEncrypter(block)
	out := make([]byte, len(src))
	ecb.CryptBlocks(out, src)

	return out, nil
}

// AESDecrypt decrypts the encrypted source with the given key.
func AESDecrypt(crypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockMode := newECBDecrypter(block)
	out := make([]byte, len(crypted))
	blockMode.CryptBlocks(out, crypted)
	return out, nil
}

type ecb struct {
	b         cipher.Block
	blockSize int
}

func newECB(b cipher.Block) *ecb {
	return &ecb{
		b:         b,
		blockSize: b.BlockSize(),
	}
}

type ecbEncrypter ecb

func newECBEncrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbEncrypter)(newECB(b))
}

func (ecb *ecbEncrypter) BlockSize() int {
	return ecb.blockSize
}

func (ecb *ecbEncrypter) CryptBlocks(dst, src []byte) {
	if len(src)%ecb.blockSize != 0 {
		panic("crypto/cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	for len(src) > 0 {
		ecb.b.Encrypt(dst, src[:ecb.blockSize])
		src = src[ecb.blockSize:]
		dst = dst[ecb.blockSize:]
	}
}

type ecbDecrypter ecb

func newECBDecrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbDecrypter)(newECB(b))
}

func (ecb ecbDecrypter) BlockSize() int {
	return ecb.blockSize
}

func (ecb *ecbDecrypter) CryptBlocks(dst, src []byte) {
	if len(src)%ecb.blockSize != 0 {
		panic("crypto/cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	for len(src) > 0 {
		ecb.b.Decrypt(dst, src[:ecb.blockSize])
		src = src[ecb.blockSize:]
		dst = dst[ecb.blockSize:]
	}
}
