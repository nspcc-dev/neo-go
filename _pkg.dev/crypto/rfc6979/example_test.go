package rfc6979

import (
	"crypto/dsa"
	"crypto/ecdsa"

	"crypto/rand"
	"crypto/sha1"
	"crypto/sha512"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/crypto/elliptic"
)

// Generates a 521-bit ECDSA key, uses SHA-512 to sign a message, then verifies
// it.
func ExampleSignECDSA() {
	// Generate a key pair.
	// You need a high-quality PRNG for this.
	curve := elliptic.NewEllipticCurve(elliptic.Secp256r1)
	k, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Hash a message.
	alg := sha512.New()
	_, _ = alg.Write([]byte("I am a potato."))
	hash := alg.Sum(nil)

	// Sign the message. You don't need a PRNG for this.

	r, s, err := SignECDSA(curve, k.D.Bytes(), hash, sha512.New)
	if err != nil {
		fmt.Println(err)
		return
	}

	if !ecdsa.Verify(&k.PublicKey, hash, r, s) {
		fmt.Println("Invalid signature!")
	}
}

// Generates a 1024-bit DSA key, uses SHA-1 to sign a message, then verifies it.
func ExampleSignDSA() {
	// Here I'm generating some DSA params, but you should really pre-generate
	// these and re-use them, since this takes a long time and isn't necessary.
	k := new(dsa.PrivateKey)
	dsa.GenerateParameters(&k.Parameters, rand.Reader, dsa.L1024N160)

	// Generate a key pair.
	// You need a high-quality PRNG for this.
	err := dsa.GenerateKey(k, rand.Reader)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Hash a message.
	alg := sha1.New()
	_, _ = alg.Write([]byte("I am a potato."))
	hash := alg.Sum(nil)

	// Sign the message. You don't need a PRNG for this.
	r, s, err := SignDSA(k, hash, sha1.New)
	if err != nil {
		fmt.Println(err)
		return
	}

	if !dsa.Verify(&k.PublicKey, hash, r, s) {
		fmt.Println("Invalid signature!")
	}

}
