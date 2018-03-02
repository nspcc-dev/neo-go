package util

import (
	"bytes"
	"testing"
)

func TestArrayReverse(t *testing.T) {
	arr := []byte{0x01, 0x02, 0x03, 0x04}
	have := ArrayReverse(arr)
	want := []byte{0x04, 0x03, 0x02, 0x01}
	if bytes.Compare(have, want) != 0 {
		t.Fatalf("expected %v got %v", want, have)
	}
}

// This tests a bug that occured with arrays of size 1
func TestArrayReverseLen2(t *testing.T) {
	arr := []byte{0x01}
	have := ArrayReverse(arr)
	want := []byte{0x01}
	if bytes.Compare(have, want) != 0 {
		t.Fatalf("expected %v got %v", want, have)
	}
}
