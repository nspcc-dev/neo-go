package io

import (
	"slices"
	"testing"
)

type someval struct {
	a int
	b int
}

func (s someval) EncodeBinary(w *BinWriter) {
	w.WriteU64LE(uint64(s.a))
	w.WriteU64LE(uint64(s.b))
}

type somepoint struct {
	a int
	b int
}

func (s *somepoint) EncodeBinary(w *BinWriter) {
	w.WriteU64LE(uint64(s.a))
	w.WriteU64LE(uint64(s.b))
}

func BenchmarkWriteArray(b *testing.B) {
	const numElems = 10
	var (
		v = slices.Repeat([]someval{{}}, numElems)
		p = slices.Repeat([]*somepoint{{}}, numElems)
	)

	w := NewBufBinWriter()
	w.Grow(numElems * 32) // more than needed, we don't need reallocations here.

	b.Run("WriteArray method, value", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			w.Reset()
			b.StartTimer()
			w.WriteArray(v)
		}
	})
	b.Run("WriteArray method, pointer", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			w.Reset()
			b.StartTimer()
			w.WriteArray(p)
		}
	})
	b.Run("WriteArray generic, value", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			w.Reset()
			b.StartTimer()
			WriteArray(w.BinWriter, v)
		}
	})
	b.Run("WriteArray generic, pointer", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			w.Reset()
			b.StartTimer()
			WriteArray(w.BinWriter, p)
		}
	})
	b.Run("open-coded, value", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			w.Reset()
			b.StartTimer()
			w.WriteVarUint(uint64(len(v)))
			for i := range v {
				v[i].EncodeBinary(w.BinWriter)
			}
		}
	})
	b.Run("open-coded, pointer", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			w.Reset()
			b.StartTimer()
			w.WriteVarUint(uint64(len(p)))
			for i := range v {
				p[i].EncodeBinary(w.BinWriter)
			}
		}
	})
}
