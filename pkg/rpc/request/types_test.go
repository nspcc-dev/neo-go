package request

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

type readCloser struct {
	io.Reader
}

func (readCloser) Close() error { return nil }

func BenchmarkUnmarshal(b *testing.B) {
	req := []byte(`{"jsonrpc":"2.0", "method":"getsomething","params":[1,2,3]}`)
	b.Run("single", func(b *testing.B) {
		b.ReportAllocs()
		b.Run("unmarshal", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				in := new(In)
				b.StartTimer()
				err := json.Unmarshal(req, in)
				if err != nil {
					b.FailNow()
				}
			}
		})
		b.Run("decode data", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				r := NewRequest()
				r.In = new(In)
				rd := bytes.NewReader(req)
				b.StartTimer()
				err := r.DecodeData(readCloser{rd})
				if err != nil {
					b.FailNow()
				}
			}
		})
	})
}
