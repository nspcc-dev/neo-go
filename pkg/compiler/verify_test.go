package compiler_test

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/stretchr/testify/require"
)

func TestVerifyGood(t *testing.T) {
	msg := []byte("test message")
	pub, sig := signMessage(t, msg)
	src := getVerifyProg(pub, sig, msg)

	eval(t, src, true)
}

func TestVerifyBad(t *testing.T) {
	msg := []byte("test message")
	pub, sig := signMessage(t, msg)
	sig[0] = ^sig[0]
	src := getVerifyProg(pub, sig, msg)

	eval(t, src, false)
}

func signMessage(t *testing.T, msg []byte) ([]byte, []byte) {
	key, err := keys.NewPrivateKey()
	require.NoError(t, err)

	sig := key.Sign(msg)
	pub := key.PublicKey().Bytes()

	return pub, sig
}

func getVerifyProg(pub, sig, msg []byte) string {
	pubS := fmt.Sprintf("%#v", pub)
	sigS := fmt.Sprintf("%#v", sig)
	msgS := fmt.Sprintf("%#v", msg)

	return `
		package hello

		import "github.com/nspcc-dev/neo-go/pkg/interop/crypto"

		func Main() bool {
			pub := ` + pubS + `
			sig := ` + sigS + `
			msg := ` + msgS + `
			return crypto.VerifySignature(msg, sig, pub)
		}
	`
}
