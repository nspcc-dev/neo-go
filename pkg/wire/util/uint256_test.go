package util

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUint256DecodeString(t *testing.T) {
	hexStr := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	val, err := Uint256DecodeString(hexStr)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, hexStr, val.Reverse().String())
}

func TestUint256DecodeBytes(t *testing.T) {
	hexStr := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatal(err)
	}
	val, err := Uint256DecodeBytes(b)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, hexStr, val.Reverse().String())
}

func TestUInt256Equals(t *testing.T) {
	a := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	b := "e287c5b29a1b66092be6803c59c765308ac20287e1b4977fd399da5fc8f66ab5"

	ua, err := Uint256DecodeString(a)
	if err != nil {
		t.Fatal(err)
	}
	ub, err := Uint256DecodeString(b)
	if err != nil {
		t.Fatal(err)
	}
	if ua.Equals(ub) {
		t.Fatalf("%s and %s cannot be equal", ua, ub)
	}
	if !ua.Equals(ua) {
		t.Fatalf("%s and %s must be equal", ua, ua)
	}
}
