package util

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUint256UnmarshalJSON(t *testing.T) {
	str := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	expected, err := Uint256DecodeString(str)
	if err != nil {
		t.Fatal(err)
	}

	// UnmarshalJSON should decode hex-strings
	var u1, u2 Uint256

	if err = u1.UnmarshalJSON([]byte(`"` + str + `"`)); err != nil {
		t.Fatal(err)
	}
	assert.True(t, expected.Equals(u1))

	s, err := expected.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	// UnmarshalJSON should decode hex-strings prefixed by 0x
	if err = u2.UnmarshalJSON(s); err != nil {
		t.Fatal(err)
	}
	assert.True(t, expected.Equals(u1))
}

func TestUint256DecodeString(t *testing.T) {
	hexStr := "f037308fa0ab18155bccfc08485468c112409ea5064595699e98c545f245f32d"
	val, err := Uint256DecodeString(hexStr)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, hexStr, val.String())
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
	assert.Equal(t, hexStr, val.String())
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
