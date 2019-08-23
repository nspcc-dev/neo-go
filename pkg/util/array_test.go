package util

import (
	"bytes"
	"testing"
)

func TestArrayEvenReverse(t *testing.T) {
	arr := []byte{0x01, 0x02, 0x03, 0x04}
	have := ArrayReverse(arr)
	want := []byte{0x04, 0x03, 0x02, 0x01}
	if !bytes.Equal(have, want) {
		t.Fatalf("expected %v got %v", want, have)
	}
}

func TestArrayOddReverse(t *testing.T) {
	arr := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	have := ArrayReverse(arr)
	want := []byte{0x05, 0x04, 0x03, 0x02, 0x01}
	if !bytes.Equal(have, want) {
		t.Fatalf("expected %v got %v", want, have)
	}
}

// This tests a bug that occurred with arrays of size 1
func TestArrayReverseLen2(t *testing.T) {
	arr := []byte{0x01}
	have := ArrayReverse(arr)
	want := []byte{0x01}
	if !bytes.Equal(have, want) {
		t.Fatalf("expected %v got %v", want, have)
	}
}
