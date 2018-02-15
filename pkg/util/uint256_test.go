package util

import (
	"bytes"
	"testing"
)

func TestToArrayReverse(t *testing.T) {
	arr := []byte{0x01, 0x02, 0x03, 0x04}
	have := ToArrayReverse(arr)
	want := []byte{0x04, 0x03, 0x02, 0x01}
	if bytes.Compare(have, want) != 0 {
		t.Fatalf("expected %v got %v", want, have)
	}
}

// This tests a bug that occured with arrays of size 1
func TestToArrayReverseLen2(t *testing.T) {
	arr := []byte{0x01}
	have := ToArrayReverse(arr)
	want := []byte{0x01}
	if bytes.Compare(have, want) != 0 {
		t.Fatalf("expected %v got %v", want, have)
	}
}
