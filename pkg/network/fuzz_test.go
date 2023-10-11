package network

import (
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/require"
)

func FuzzMessageDecode(f *testing.F) {
	for i := 0; i < 100; i++ {
		seed := make([]byte, rand.Uint32()%1000)
		rand.Read(seed)
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value []byte) {
		m := new(Message)
		r := io.NewBinReaderFromBuf(value)
		require.NotPanics(t, func() { _ = m.Decode(r) })
	})
}
