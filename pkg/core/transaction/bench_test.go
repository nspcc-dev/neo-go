package transaction

import (
	"encoding/base64"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/require"
)

// Some typical transfer tx from mainnet.
var (
	benchTx     []byte
	benchTxB64  = "AK9KzFu0P5gAAAAAAIjOEgAAAAAA7jAAAAGIDdjSt7aj2J+dktSobkC9j0/CJwEAWwsCAMLrCwwUtXfkIuockX9HAVMNeEuQMxMlYkMMFIgN2NK3tqPYn52S1KhuQL2PT8InFMAfDAh0cmFuc2ZlcgwUz3bii9AGLEpHjuNVYQETGfPPpNJBYn1bUjkBQgxAUiZNae4OTSu2EOGW+6fwslLIpVsczOAR9o6R796tFf2KG+nLzs709tCQ7NELZOQ7zUzfF19ADLvH/efNT4v9LygMIQNT96/wFdPSBO7NUI9Kpn9EffTRXsS6ZJ9PqRvbenijVEFW57Mn"
	benchTxJSON []byte
)

func init() {
	var err error
	benchTx, err = base64.StdEncoding.DecodeString(benchTxB64)
	if err != nil {
		panic(err)
	}
	t, err := NewTransactionFromBytes(benchTx)
	if err != nil {
		panic(err)
	}
	benchTxJSON, err = t.MarshalJSON()
	if err != nil {
		panic(err)
	}
}

func BenchmarkDecodeBinary(t *testing.B) {
	for range t.N {
		r := io.NewBinReaderFromBuf(benchTx)
		tx := &Transaction{}
		tx.DecodeBinary(r)
		require.NoError(t, r.Err)
	}
}

func BenchmarkDecodeJSON(t *testing.B) {
	for range t.N {
		tx := &Transaction{}
		require.NoError(t, tx.UnmarshalJSON(benchTxJSON))
	}
}

func BenchmarkDecodeFromBytes(t *testing.B) {
	for range t.N {
		_, err := NewTransactionFromBytes(benchTx)
		require.NoError(t, err)
	}
}

func BenchmarkTransaction_Bytes(b *testing.B) {
	tx, err := NewTransactionFromBytes(benchTx)
	require.NoError(b, err)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = tx.Bytes()
	}
}

func BenchmarkGetVarSize(b *testing.B) {
	tx, err := NewTransactionFromBytes(benchTx)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		_ = io.GetVarSize(tx)
	}
}
