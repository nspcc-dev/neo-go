package hash

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSha256(t *testing.T) {
	input := []byte("hello")
	data := Sha256(input)

	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	actual := hex.EncodeToString(data.Bytes())

	assert.Equal(t, expected, actual)
}

func TestHashDoubleSha256(t *testing.T) {
	input := []byte("hello")
	data := DoubleSha256(input)

	firstSha := Sha256(input)
	doubleSha := Sha256(firstSha.Bytes())
	expected := hex.EncodeToString(doubleSha.Bytes())

	actual := hex.EncodeToString(data.Bytes())
	assert.Equal(t, expected, actual)
}

func TestHashRipeMD160(t *testing.T) {
	input := []byte("hello")
	data := RipeMD160(input)

	expected := "108f07b8382412612c048d07d13f814118445acd"
	actual := hex.EncodeToString(data.Bytes())
	assert.Equal(t, expected, actual)
}

func TestHash160(t *testing.T) {
	input := "02cccafb41b220cab63fd77108d2d1ebcffa32be26da29a04dca4996afce5f75db"
	publicKeyBytes, _ := hex.DecodeString(input)
	data := Hash160(publicKeyBytes)

	expected := "c8e2b685cc70ec96743b55beb9449782f8f775d8"
	actual := hex.EncodeToString(data.Bytes())
	assert.Equal(t, expected, actual)
}
