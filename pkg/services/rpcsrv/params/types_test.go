package params

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/stretchr/testify/require"
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
			for b.Loop() {
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
			for b.Loop() {
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

func TestRequestDecode(t *testing.T) {
	t.Run("valid single", func(t *testing.T) {
		r := NewRequest()
		err := r.DecodeData(readCloser{strings.NewReader(`{"jsonrpc":"2.0","method":"getblockcount","params":[],"id":1}`)})
		require.NoError(t, err)
		require.NotNil(t, r.In)
		require.Nil(t, r.Batch)
		require.Equal(t, neorpc.JSONRPCVersion, r.In.JSONRPC)
		require.Equal(t, "getblockcount", r.In.Method)
		require.Equal(t, json.RawMessage("1"), r.In.RawID)
		require.Nil(t, r.In.Err)
	})

	t.Run("valid single with trailing token via DecodeData", func(t *testing.T) {
		r := NewRequest()
		err := r.DecodeData(readCloser{strings.NewReader(`{"jsonrpc":"2.0","method":"getblockcount","params":[],"id":1}"`)})
		require.NoError(t, err)
		require.NotNil(t, r.In)
		require.Equal(t, "getblockcount", r.In.Method)
	})

	t.Run("valid batch", func(t *testing.T) {
		r := NewRequest()
		err := r.DecodeData(readCloser{strings.NewReader(`[{"jsonrpc":"2.0","method":"getblockcount","id":1},{"jsonrpc":"2.0","method":"getversion","id":2}]`)})
		require.NoError(t, err)
		require.Nil(t, r.In)
		require.Len(t, r.Batch, 2)
		require.Equal(t, "getblockcount", r.Batch[0].Method)
		require.Nil(t, r.Batch[0].Err)
		require.Equal(t, "getversion", r.Batch[1].Method)
		require.Nil(t, r.Batch[1].Err)
	})

	t.Run("maxBatchSize boundary", func(t *testing.T) {
		makeBatch := func(n int) string {
			item := `{"jsonrpc":"2.0","method":"getblockcount","id":1}`
			return "[" + strings.TrimSuffix(strings.Repeat(item+",", n), ",") + "]"
		}

		r := NewRequest()
		require.NoError(t, r.DecodeData(readCloser{strings.NewReader(makeBatch(maxBatchSize))}))
		require.Len(t, r.Batch, maxBatchSize)

		r = NewRequest()
		err := r.DecodeData(readCloser{strings.NewReader(makeBatch(maxBatchSize + 1))})
		rpcErr, ok := errors.AsType[*neorpc.Error](err)
		require.True(t, ok)
		require.Equal(t, int64(neorpc.InvalidRequestCode), rpcErr.Code)
	})

	invalidRequestCases := []struct {
		name string
		data string
	}{
		{name: "empty batch", data: `[]`},
		{name: "number", data: `42`},
		{name: "true", data: `true`},
		{name: "false", data: `false`},
		{name: "string", data: `"hello"`},
		{name: "null", data: `null`},
		{name: "invalid method type", data: `{"jsonrpc":"2.0","method":1,"params":"bar"}`},
		{name: "invalid params type", data: `{"jsonrpc":"2.0","method":"foo","params":"bar"}`},
	}
	for _, tc := range invalidRequestCases {
		t.Run("invalid request/"+tc.name, func(t *testing.T) {
			for _, via := range []string{"DecodeData", "UnmarshalJSON"} {
				t.Run(via, func(t *testing.T) {
					r := NewRequest()
					var err error
					if via == "DecodeData" {
						err = r.DecodeData(readCloser{strings.NewReader(tc.data)})
					} else {
						err = r.UnmarshalJSON([]byte(tc.data))
					}
					rpcErr, ok := errors.AsType[*neorpc.Error](err)
					require.True(t, ok, "expected *neorpc.Error, got %T: %v", err, err)
					require.Equal(t, int64(neorpc.InvalidRequestCode), rpcErr.Code)
				})
			}
		})
	}

	t.Run("mixed batch", func(t *testing.T) {
		payload := `[42, [42], null, true, "hello", {"jsonrpc":"2.0","method":1,"params":"bar"}, {"jsonrpc":"2.0","method":"getblockcount","id":7}]`
		r := NewRequest()
		require.NoError(t, r.DecodeData(readCloser{strings.NewReader(payload)}))
		require.Nil(t, r.In)
		require.Len(t, r.Batch, 7)
		for i := range 6 {
			require.Equal(t, neorpc.JSONRPCVersion, r.Batch[i].JSONRPC)
			require.Nil(t, r.Batch[i].RawID)
			require.NotNil(t, r.Batch[i].Err)
			require.Equal(t, int64(neorpc.InvalidRequestCode), r.Batch[i].Err.Code)
		}
		require.Nil(t, r.Batch[6].Err)
		require.Equal(t, "getblockcount", r.Batch[6].Method)
		require.Equal(t, json.RawMessage("7"), r.Batch[6].RawID)
	})

	malformedCases := []struct {
		name string
		data string
	}{
		{name: "empty", data: ``},
		{name: "whitespace", data: " \t\n"},
		{name: "malformed object", data: `{"jsonrpc": "2.0", "method": "foobar, "params": "bar", "baz]`},
		{name: "malformed batch", data: `[{"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": "1"}, {"jsonrpc": "2.0", "method"]`},
		{name: "unclosed array", data: `[1, 2`},
		{name: "trailing comma in array", data: `[1,]`},
		{name: "invalid literal", data: `tru`},
	}
	for _, tc := range malformedCases {
		t.Run("malformed/"+tc.name, func(t *testing.T) {
			for _, via := range []string{"DecodeData", "UnmarshalJSON"} {
				t.Run(via, func(t *testing.T) {
					r := NewRequest()
					var err error
					if via == "DecodeData" {
						err = r.DecodeData(readCloser{strings.NewReader(tc.data)})
					} else {
						err = r.UnmarshalJSON([]byte(tc.data))
					}
					require.Error(t, err)
					_, ok := errors.AsType[*neorpc.Error](err)
					require.False(t, ok, "malformed JSON must not return *neorpc.Error: %v", err)
				})
			}
		})
	}

	t.Run("UnmarshalJSON trailing data", func(t *testing.T) {
		for _, data := range []string{`[1] extra`, `[] extra`, `{"jsonrpc":"2.0"} extra`, `42 43`} {
			r := NewRequest()
			err := r.UnmarshalJSON([]byte(data))
			require.Error(t, err)
			_, ok := errors.AsType[*neorpc.Error](err)
			require.False(t, ok, "trailing data must not return *neorpc.Error: %v", err)
		}
	})
}
