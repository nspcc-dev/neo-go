package rfc6979

import (
	"crypto/dsa"
	"hash"
	"math/big"
)

// SignDSA signs an arbitrary length hash (which should be the result of hashing
// a larger message) using the private key, priv. It returns the signature as a
// pair of integers.
//
// Note that FIPS 186-3 section 4.6 specifies that the hash should be truncated
// to the byte-length of the subgroup. This function does not perform that
// truncation itself.
func SignDSA(priv *dsa.PrivateKey, hash []byte, alg func() hash.Hash) (r, s *big.Int, err error) {
	n := priv.Q.BitLen()
	if n&7 != 0 {
		err = dsa.ErrInvalidPublicKey
		return
	}
	n >>= 3

	generateSecret(priv.Q, priv.X, alg, hash, func(k *big.Int) bool {
		inv := new(big.Int).ModInverse(k, priv.Q)
		r = new(big.Int).Exp(priv.G, k, priv.P)
		r.Mod(r, priv.Q)

		if r.Sign() == 0 {
			return false
		}

		z := new(big.Int).SetBytes(hash)

		s = new(big.Int).Mul(priv.X, r)
		s.Add(s, z)
		s.Mod(s, priv.Q)
		s.Mul(s, inv)
		s.Mod(s, priv.Q)

		return s.Sign() != 0
	})

	return
}
