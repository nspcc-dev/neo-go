package params

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
	req := []byte(`{"jsonrpc":"2.0", "method":"invokefunction","params":["0x50befd26fdf6e4d957c11e078b24ebce6291456f", "someMethod", [{"type": "String", "value": "50befd26fdf6e4d957c11e078b24ebce6291456f"}, {"type": "Integer", "value": "42"}, {"type": "Boolean", "value": false}]]}`)
	b.Run("single", func(b *testing.B) {
		b.ReportAllocs()
		b.Run("unmarshal", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
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
			for range b.N {
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
