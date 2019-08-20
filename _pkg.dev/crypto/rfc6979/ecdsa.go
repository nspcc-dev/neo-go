package rfc6979

import (
	"hash"
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/crypto/elliptic"
)

// SignECDSA signs an arbitrary length hash (which should be the result of
// hashing a larger message) using the private key, priv. It returns the
// signature as a pair of integers.
//
// Note that FIPS 186-3 section 4.6 specifies that the hash should be truncated
// to the byte-length of the subgroup. This function does not perform that
// truncation itself.
func SignECDSA(curve elliptic.Curve, priv []byte, hash []byte, alg func() hash.Hash) (r, s *big.Int, err error) {
	c := curve
	N := c.N
	D := new(big.Int)
	D.SetBytes(priv)
	generateSecret(N, D, alg, hash, func(k *big.Int) bool {

		inv := new(big.Int).ModInverse(k, N)

		r, _ = curve.ScalarBaseMult(k.Bytes())
		r.Mod(r, N)

		if r.Sign() == 0 {
			return false
		}

		e := hashToInt(hash, c)
		s = new(big.Int).Mul(D, r)
		s.Add(s, e)
		s.Mul(s, inv)
		s.Mod(s, N)

		return s.Sign() != 0
	})

	return
}

// copied from crypto/ecdsa
func hashToInt(hash []byte, c elliptic.Curve) *big.Int {
	orderBits := c.N.BitLen()
	orderBytes := (orderBits + 7) / 8
	if len(hash) > orderBytes {
		hash = hash[:orderBytes]
	}

	ret := new(big.Int).SetBytes(hash)
	excess := len(hash)*8 - orderBits
	if excess > 0 {
		ret.Rsh(ret, uint(excess))
	}
	return ret
}
