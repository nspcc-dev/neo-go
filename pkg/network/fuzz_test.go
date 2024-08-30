package network

import (
	"math/rand/v2"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/require"
)

func FuzzMessageDecode(f *testing.F) {
	for range 100 {
		seed := make([]byte, rand.IntN(1000))
		random.Fill(seed)
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value []byte) {
		m := new(Message)
		r := io.NewBinReaderFromBuf(value)
		require.NotPanics(t, func() { _ = m.Decode(r) })
	})
}
