package slice

import (
	"bytes"
	"testing"
)

func TestSliceReverse(t *testing.T) {
	arr := []byte{0x01, 0x02, 0x03, 0x04}
	have := Reverse(arr)
	want := []byte{0x04, 0x03, 0x02, 0x01}
	if bytes.Compare(have, want) != 0 {
		t.Fatalf("expected %v got %v", want, have)
	}
}
func TestSliceReverseOddNumberOfElements(t *testing.T) {
	arr := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	have := Reverse(arr)
	want := []byte{0x05, 0x04, 0x03, 0x02, 0x01}
	if bytes.Compare(have, want) != 0 {
		t.Fatalf("expected %v got %v", want, have)
	}
}

// This tests a bug that occured with arrays of size 1
func TestSliceReverseLen2(t *testing.T) {
	arr := []byte{0x01}
	have := Reverse(arr)
	want := []byte{0x01}
	if bytes.Compare(have, want) != 0 {
		t.Fatalf("expected %v got %v", want, have)
	}
}
