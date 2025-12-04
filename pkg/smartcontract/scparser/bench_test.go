package scparser

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkIsSignatureContract(t *testing.B) {
	b64script := "DCED2eixa9myLTNF1tTN4xvhw+HRYVMuPQzOy5Xs4utYM25BVuezJw=="
	script, err := base64.StdEncoding.DecodeString(b64script)
	require.NoError(t, err)
	for t.Loop() {
		_ = IsSignatureContract(script)
	}
}
